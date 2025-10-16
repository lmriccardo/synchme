package dbutils

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
)

// Statement represents a prepared SQL statement. It also holds a mapping from named
// arguments to their respective positions, required when executing the statement
type Statement struct {
	Query string // Query used to build the statement

	stmt          *sql.Stmt        // The actual sql prepared statement
	positionalMap map[string][]int // A mapping from arguments to position
	nofArgs       int              // Total number of arguments
}

// Buildable defines the minimum set of methods required for any object
// that can be converted into an executable SQL statement.
type Buildable interface {
	Validate() error                          // Validates the buildable before building it
	Build() (string, error)                   // Returns the string query of the operation
	Prepare() (*Statement, TypeValidatorFunc) // Prepare finalizes the query and prepares it for execution
}

// Rangeable extends the Buildable interface with methods for controlling the
// size and order of a result set, commonly used in SELECT statements.
// These methods typically return the Rangeable interface itself to allow
// for method chaining.
type Rangeable interface {
	Buildable
	Limit(int) Rangeable                   // Limit specifies the maximum number of rows to return.
	Offset(int) Rangeable                  // Offset specifies the number of rows to skip
	OrderBy(string, OrderByType) Rangeable // OrderBy specifies a column and direction (ASC/DESC)
}

type preparator_t struct {
	sql   string // The SQL string produced by a previous Build
	table *Table // The table subject to the current operation
}

// UpdateBuilder is a fluent interface structure used to construct an SQL UPDATE statement.
// It tracks the columns and values to be set, and embeds structures for defining
// the WHERE clause and any limiting/ordering clauses.
type UpdateBuilder struct {
	columns    map[string]any // Columns affected by the update operation
	parameters []string       // Parameters for each column (actually the column name)

	table *Table // The table the current operation is updating

	where_clause_t // Embedded structure for building the WHERE clause.
	range_clause_t // Embedded structure for building the ORDER BY, LIMIT and OFFSET clauses.
	preparator_t   // Embedded structure for preparing SQL statements
}

type InsertBuilder struct {
	preparator_t // Embedded structure for preparing SQL statements

	columns map[string]any // Columns to which insert values
	table   *Table         // The table the current operation is updating
}

type SelectBuilder struct {
	// table *Table // The table the current operation is updating

	where_clause_t // Embedded structure for building the WHERE clause.
	range_clause_t // Embedded structure for building the ORDER BY, LIMIT and OFFSET clauses.
	preparator_t   // Embedded structure for preparing SQL statements
}

// processMatch updates the query, statement mappings, and column associations
// based on a single regex match.
func (p *preparator_t) processMatch(stmt *Statement, columns map[string]string,
	query string, match []string) (string, bool) {

	// If the length of the match is less than 4 then returns the entire query
	// This should not happen but for safety it is good practice to prevent it
	if len(match) < 4 {
		return query, false
	}

	// Determine which capture contains the parameter name
	param := match[1]
	if match[3] != "" {
		param = match[3]
	}

	name, ok := strings.CutPrefix(param, ":")
	if !ok {
		return query, false
	}

	// Register parameter position
	stmt.positionalMap[name] = append(stmt.positionalMap[name], stmt.nofArgs)
	stmt.nofArgs++

	// Associate parameter with its column name (if any)
	if match[3] != "" && match[2] != "" {
		columns[name] = match[2]
	} else {
		columns[name] = name
	}

	// Replace parameter with the correct placeholder
	placeholder := p.table.sch.GetPlaceholder(stmt.nofArgs)
	query = strings.Replace(query, param, placeholder, 1)

	return query, true
}

// Prepare analyzes the embedded SQL string (`p.sql`) to identify named parameters
// and the database columns they reference. It prepares the underlying SQL statement
// and constructs a mapping from named parameters to their positional indices
// within the prepared statement.
func (p *preparator_t) Prepare() (*Statement, TypeValidatorFunc) {
	stmt := &Statement{
		positionalMap: make(map[string][]int),
	}

	query := p.sql
	columns := make(map[string]string)

	// Extract all parameter matches from the SQL
	matches := extractParamMatches(p.sql)

	// Process each match and replace parameters
	for _, m := range matches {
		query, _ = p.processMatch(stmt, columns, query, m)
	}

	// Prepare the statement
	preparedStmt, err := p.table.sch.db.Prepare(query)
	if err != nil {
		fmt.Println("Error preparing statement:", err)
		return nil, nil
	}

	stmt.stmt = preparedStmt
	stmt.Query = query

	return stmt, func(params map[string]any) (bool, []error) {
		return ValidateTypes(columns, params, p.table)
	}
}

// Set registers one or more column names to be included in the SET clause of the
// UPDATE statement. Each column is internally mapped to a placeholder ('?') for a
// prepared statement, and its name is added to the list of expected parameters for
// later binding.
func (u *UpdateBuilder) Set(columns ...string) *UpdateBuilder {
	// Actually, columns and parameters represent the same slice
	for _, column := range columns {
		// Add the column and parameter if not existing
		if _, ok := u.columns[column]; !ok {
			u.columns[column] = "?"
			u.parameters = append(u.parameters, column)
		}
	}

	return u
}

// SetValue directly assigns a literal value or expression to a column in
// the SET clause. This is used for values that should be embedded directly
// into the SQL statement.
func (u *UpdateBuilder) SetValue(name string, value any) *UpdateBuilder {
	// This is the value directly in the column and does not
	// include the input column name as a parameter
	if _, ok := u.columns[name]; !ok {
		u.columns[name] = value
	}

	return u
}

// Prepare finalizes the UpdateBuilder and returns a prepared SQL Statement
// ready for execution, along with a function to validate argument types.
func (u *UpdateBuilder) Prepare() (*Statement, TypeValidatorFunc) {
	// First build the SQL query containing positional arguments ids
	// and check that there are no errors during build stage
	if sql, err := u.Build(); err != nil {
		fmt.Println(err)
		return nil, nil
	} else {
		u.sql = sql
		return u.preparator_t.Prepare()
	}
}

// Validate validates the UPDATE SQL Query
func (u *UpdateBuilder) Validate() error {
	// Initialize the list of all errors detected during validation
	errs := []error{}

	// First we would like to check if columns in each suboperation
	// exists in the actual table.

	// First check columns used for the SET clause
	for _, col := range utils.MapKeys(u.columns) {
		if _, ok := u.table.ColumnsIndex[col]; !ok {
			errs = append(errs, fmt.Errorf(
				"column used for SET %q not a table column", col))
		}
	}

	// Check WHERE clause
	errs = append(errs, u.ValidateColumns(u.table)...)

	if len(errs) == 0 {
		return nil
	}

	// Format the error string and return all errs
	h := func(e error) string { return e.Error() }
	return fmt.Errorf("%s", strings.Join(utils.Map(h, errs), "\n"))
}

func (s *SelectBuilder) Validate() error {
	return nil
}

// Columns select the input columns for the INSERT operation
func (i *InsertBuilder) Columns(names ...string) *InsertBuilder {
	for _, name := range names {
		i.columns[name] = "?"
	}

	return i
}

// ColumnWithValue fix the value of the input column for all INSERT operation
// to the given input value. Subsquent call on the same column name ovverrides
// all values previously inserted
func (i *InsertBuilder) ColumnWithValue(name string, value any) *InsertBuilder {
	i.columns[name] = value
	return i
}

// Prepare finalizes the InsertBuilder and returns a prepared SQL Statement
// ready for execution, along with a function to validate argument types.
func (i *InsertBuilder) Prepare() (*Statement, TypeValidatorFunc) {
	// First build the SQL query containing positional arguments ids
	// and check that there are no errors during build stage
	if sql, err := i.Build(); err != nil {
		fmt.Println(err)
		return nil, nil
	} else {
		i.sql = sql
		return i.preparator_t.Prepare()
	}
}

func (i *InsertBuilder) Validate() error {
	// Initialize the list of all errors detected during validation
	errs := []error{}

	// Check if there is at least one column
	if len(i.columns) == 0 {
		return errors.New(
			"in INSERT there must be at least one specified column")
	}

	// Validates that all column exists in the current table and that
	// associated values have the expected type
	for name, value := range i.columns {
		idx, ok := i.table.ColumnsIndex[name]
		if !ok {
			errs = append(errs, fmt.Errorf(
				"in INSERT operation column %q not a table %q column",
				name, i.table.Name,
			))
			continue
		}

		// Check if the current value is a string but does not starts
		// with the named parameter prefix, or it is not a string at all
		x, ok := value.(string)
		if (ok && !strings.HasPrefix(x, ":")) || !ok {
			if err := checkValueType(value, i.table.Columns[idx]); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}

	// Format the error string and return all errors
	h := func(e error) string { return e.Error() }
	return fmt.Errorf("%s", strings.Join(utils.Map(h, errs), "\n"))
}

// Build builds the SQL operation for updating the database
func (u *UpdateBuilder) Build() (string, error) {
	if err := u.Validate(); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("UPDATE %s\n", u.table.Name))

	// Write the SET values in the string builder
	sets := []string{}
	for name, value := range u.columns {
		// I need to map values to correct formatting for a
		// a partial valid SQL query to be prepare in a future moment
		if value != "?" {
			value = SqlLiteral(value)
		} else {
			value = fmt.Sprintf(":%s", name)
		}

		sets = append(sets, fmt.Sprintf("%s = %s", name, value))
	}

	if len(sets) > 0 {
		builder.WriteString(fmt.Sprintf("SET %s\n",
			strings.Join(sets, ", ")))
	}

	// Now put the WHERE clause
	if sql := u.where_clause_t.ToSQLString(); sql != "" {
		builder.WriteString(sql)
		builder.WriteString("\n")
	}

	// Then the rage limit clause (ORDER BY, LIMIT, OFFSET)
	if sql := u.range_clause_t.ToSQLString(); sql != "" {
		builder.WriteString(sql)
	}

	return builder.String(), nil
}

func (s *SelectBuilder) Build() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}

	return "", nil
}

// Build builds the SQL operation for inserting into the database
func (i *InsertBuilder) Build() (string, error) {
	// First validate
	if err := i.Validate(); err != nil {
		return "", err
	}

	// If validation is successful, then starts building the SQL string
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("INSERT INTO %s ", i.table.Name))

	columns := []string{}
	values := []string{}

	for column, value := range i.columns {
		columns = append(columns, column)

		x, ok := value.(string)
		if ok && x == "?" {
			values = append(values, fmt.Sprintf(":%s", column))
		} else {
			values = append(values, SqlLiteral(value))
		}
	}

	builder.WriteString(fmt.Sprintf("(%s)\nVALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(values, ", ")))

	return builder.String(), nil
}
