package dbutils

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
)

// Buildable defines the minimum set of methods required for any object
// that can be converted into an executable SQL statement. This interface
// is the foundation for all SQL builders.
type Buildable interface {
	// Validate checks the internal state of the builder to ensure it is
	// correctly configured before attempting to generate the SQL query.
	// It returns an error if the configuration is invalid (e.g., missing table, no columns).
	Validate() error

	// Build generates and returns the raw SQL query string for the operation.
	// It does not include logic for database-specific placeholders or preparation.
	// It returns the query string and an error if validation or building fails.
	Build() (string, error)

	// Prepare finalizes the query, handles database-specific formatting (like placeholders),
	// and wraps the result in a *Statement structure, ready for execution.
	Prepare() *Statement
}

// Rangeable extends the Buildable interface with methods for controlling the
// size and order of a result set, commonly used in SELECT statements.
// These methods typically return the Rangeable interface itself to allow
// for method chaining.
type Rangeable interface {
	// Embeds the Buildable interface, meaning any Rangeable object must also
	// implement Validate, Build, and Prepare.
	Buildable

	// Limit specifies the maximum number of rows to return from the result set.
	// It takes an integer for the limit value and returns the Rangeable interface
	// for further chaining.
	Limit(int) Rangeable

	// Offset specifies the number of rows to skip before starting to return
	// the result set. This is commonly used for pagination.
	// It takes an integer for the offset value and returns Rangeable.
	Offset(int) Rangeable

	// OrderBy specifies a column and the direction (e.g., ASC or DESC, defined by OrderByType)
	// by which the result set should be sorted.
	// It takes the column name and the order type, and returns Rangeable.
	OrderBy(string, OrderByType) Rangeable
}

// Statement represents a prepared SQL statement. It holds the final query,
// metadata about the columns, and the necessary underlying objects for execution,
// including a mapping from named arguments to their respective placeholder positions.
type Statement struct {
	QueryString   string            // The fully formatted, final SQL query text with placeholders.
	Columns       map[string]string // Maps original column names to their aliases/final result names.
	table         *Table            // Pointer to the primary Table structure involved in the statement.
	ctx           context.Context   // The execution context (for timeouts, cancellation, etc.).
	stmt          *sql.Stmt         // The actual underlying pre-compiled *sql.Stmt object.
	positionalMap map[string][]int  // Maps named arguments to their 1-based positional index(es) in the query.
	nofArgs       int               // The total number of unique arguments (placeholders) in the query.
}

// constructInputArguments translates a map of named argument values into a
// slice of positional arguments ([]any) suitable for execution against a database
// driver (like sql.DB.Exec).
func (stmt *Statement) constructInputArguments(args map[string]any) ([]any, error) {
	stmt_arguments := make([]any, stmt.nofArgs)
	for param_name, param_value := range args {
		param_positions, ok := stmt.positionalMap[param_name]
		if !ok {
			return nil, fmt.Errorf(
				"parameter %q is not required to run the statement", param_name)
		}

		for _, position := range param_positions {
			stmt_arguments[position] = param_value
		}
	}

	return stmt_arguments, nil
}

// prepareArguments takes flexible input arguments, normalizes them, validates their
// types and values against the statement's schema, and finally converts them into
// a positionally ordered slice ready for SQL execution. This method is a crucial
// preprocessing step before executing a database query, ensuring the arguments are
// correctly structured and valid.
func (stmt *Statement) prepareArguments(args any) ([]any, error) {
	// First normalize the input arguments to a map from string to any
	normalized_args, err := normalizeArguments(args)
	if err != nil {
		return nil, err
	}

	// Run the validator on the input arguments
	if _, errs := ValidateTypes(stmt.Columns, normalized_args, stmt.table); len(errs) > 0 {
		return nil, errors.New(strings.Join(utils.Map(func(e error) string {
			return e.Error()
		}, errs), "\n"))
	}

	// Construct the input parameters for the statement execution
	stmt_arguments, err := stmt.constructInputArguments(normalized_args)
	if err != nil {
		return nil, err
	}

	return stmt_arguments, nil
}

// Exec executes the underlying prepared SQL statement (`stmt.stmt`) after processing,
// validating, and arranging the input arguments. It returns the result of the operation
// and an optional error, if something didnt go as expected.
func (stmt *Statement) Exec(args any) (sql.Result, error) {
	// If the input argument is nil then we need to check if the statement
	// does have some named parameters that are necessarily
	if args == nil {
		if stmt.nofArgs > 0 {
			return nil, fmt.Errorf("statement requires %d but nil is passed", stmt.nofArgs)
		}

		return stmt.stmt.ExecContext(stmt.ctx)
	}

	stmt_arguments, err := stmt.prepareArguments(args)
	if err != nil {
		return nil, err
	}

	// Execute the statement and returns error if any with the SQL Result
	return stmt.stmt.ExecContext(stmt.ctx, stmt_arguments...)
}

// Exec executes the underlying prepared SQL statement (`stmt.stmt`) after processing,
// validating, and arranging the input arguments. It returns the result of the operation
// and an optional error, if something didnt go as expected.
func (stmt *Statement) Query(args any) (*RowScanner, error) {
	// If the input argument is nil then we need to check if the statement
	// does have some named parameters that are necessarily
	var rows *sql.Rows
	var err error

	if args == nil {
		if stmt.nofArgs > 0 {
			return nil, fmt.Errorf("statement requires %d but nil is passed", stmt.nofArgs)
		}

		rows, err = stmt.stmt.QueryContext(stmt.ctx)
	} else {
		stmt_arguments, err1 := stmt.prepareArguments(args)
		if err1 != nil {
			return nil, err1
		}

		// Execute the statement and returns error if any with the SQL Result
		rows, err = stmt.stmt.QueryContext(stmt.ctx, stmt_arguments...)
	}

	if err != nil {
		return nil, err
	}

	return &RowScanner{rows: rows, columns: stmt.table.ColumnNames()}, nil
}

type preparator_t struct {
	sql   string // The SQL string produced by a previous Build
	table *Table // The table subject to the current operation
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
func (p *preparator_t) Prepare() *Statement {
	stmt := &Statement{
		Columns:       make(map[string]string),
		table:         p.table,
		ctx:           p.table.sch.ctx,
		positionalMap: make(map[string][]int),
	}

	query := p.sql

	// Extract all parameter matches from the SQL
	matches := extractParamMatches(p.sql)

	// Process each match and replace parameters
	for _, m := range matches {
		query, _ = p.processMatch(stmt, stmt.Columns, query, m)
	}

	// Prepare the statement
	preparedStmt, err := p.table.sch.db.Prepare(query)
	if err != nil {
		fmt.Println("Error preparing statement:", err)
		return nil
	}

	stmt.stmt = preparedStmt
	stmt.QueryString = query

	return stmt
}

// UpdateBuilder is a structure used to construct SQL UPDATE statements.
// It contains the data and methods necessary to specify which columns to update,
// the values for those columns, the target table, and clauses for filtering
// (WHERE) and limiting the affected rows (RANGE).
type UpdateBuilder struct {
	columns        map[string]any // Maps column names (string) to their new values for the SET clause.
	parameters     []string       // Slice of column names used internally to manage the update order.
	table          *Table         // Pointer to the target Table structure for the UPDATE operation.
	where_clause_t                // Embedded structure managing the conditions for the WHERE clause.
	range_clause_t                // Embedded structure managing ORDER BY, LIMIT, and OFFSET clauses.
	preparator_t                  // Embedded structure for preparing the final SQL statement and arguments.
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
func (u *UpdateBuilder) Prepare() *Statement {
	// First build the SQL query containing positional arguments ids
	// and check that there are no errors during build stage
	if sql, err := u.Build(); err != nil {
		fmt.Println(err)
		return nil
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

// InsertBuilder is a structure used to build SQL INSERT statements.
// It holds the data for the columns and values to be inserted,
// the target table, and a preparator for generating the final
// prepared SQL statement.
type InsertBuilder struct {
	columns      map[string]any // Maps column names (string) to their insertion values (any).
	table        *Table         // Pointer to the target Table structure for the INSERT.
	preparator_t                // Embedded structure for preparing the final SQL statement and arguments.
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
func (i *InsertBuilder) Prepare() *Statement {
	// First build the SQL query containing positional arguments ids
	// and check that there are no errors during build stage
	if sql, err := i.Build(); err != nil {
		fmt.Println(err)
		return nil
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

// select_column_t is an internal structure used to represent a single
// column or expression being selected in a SQL SELECT statement.
// It stores the column's name, an optional alias, a flag to indicate
// if an alias is present, and a possible operation applied to the column.
type select_column_t struct {
	name      string   // Column name or expression (e.g., "id", "COUNT(*)").
	alias     any      // Alternative name for the result (the AS clause).
	has_alias bool     // True if the 'alias' field should be used in the query.
	op        ColumnOp // Optional function or operation applied to the column.
	table     string   // Optional table name or alias used to qualify the column
}

// table_ref_t is an internal structure used to hold a reference to a table
// in a SQL query, including its original name and an optional alias.
type table_ref_t struct {
	name      string // The original name of the database table.
	alias     any    // The optional alternative name (alias) for the table in the query.
	has_alias bool   // True if the 'alias' field is actively set and should be used (e.g., 'table alias').
}

// join_t is an internal structure used to represent a single JOIN clause
// in a SQL SELECT statement, defining the type of join, the table being
// joined, and the condition for the join (the ON clause).
type join_t struct {
	join_type JoinType     // The type of join (e.g., INNER, LEFT, RIGHT, FULL) defined by the JoinType enum/type.
	target    *table_ref_t // Pointer to the *table_ref_t detailing the table being joined.
	condition string       // The raw SQL condition string for the ON clause (e.g., "u.id = o.user_id").
}

// SelectBuilder is a structure used to construct SQL SELECT statements.
// It manages the columns to be retrieved, column aliases, the target table,
// and embeds clauses for filtering (WHERE), ordering/limiting (RANGE),
// and preparing the final SQL string.
type SelectBuilder struct {
	columns map[string]*select_column_t // Maps keys to *select_column_t for all selected columns/expressions.
	from    map[string]*table_ref_t     // Maps table aliases to *table_ref_t for all tables in the query.
	joins   []*join_t                   // Slice of joins statements

	schema   *Schema // Reference to the entire Database Schema for context and validation.
	distinct bool    // If the distinct operation is applied to query-level

	where_clause_t // Embedded structure managing conditions for the WHERE clause.
	range_clause_t // Embedded structure managing ORDER BY, LIMIT, and OFFSET clauses.
}

// Distinct sets the DISTINCT operator query-level
func (s *SelectBuilder) Distinct() *SelectBuilder {
	s.distinct = true
	return s
}

func (s *SelectBuilder) addColumn(name string, alias any, op ColumnOp) {
	// We need to check whether the name also provide a table reference
	// The table reference might also be an alias for a table. At this
	// step of SELECT operation construction, we dont really care.
	table_reference := ""
	raw_column_name := name
	if strings.Contains(name, ".") {
		split_result := strings.Split(name, ".")
		table_reference = split_result[0]
		raw_column_name = split_result[1]
	}

	// If the column has already been previously inserted returns
	if _, ok := s.columns[name]; ok {
		return
	}

	// Now construct the column reference and put it into the map
	s.columns[name] = &select_column_t{
		name:      raw_column_name,
		alias:     alias,
		has_alias: alias != nil,
		op:        op,
		table:     table_reference,
	}
}

// Columns specifies the columns to be selected in the SQL query. It does not
// append columns that already belongs to the column list. Moreover, this function
// does not perform any sanity check on the input strings, therefore each
// element in the input slice must represent an exact column name
func (s *SelectBuilder) Columns(names ...string) *SelectBuilder {
	for _, column_name := range names {
		s.addColumn(column_name, nil, NONE)
	}

	return s
}

// ColumnWithAlias adds a new column and its associated alias
func (s *SelectBuilder) ColumnWithAlias(name, alias string) *SelectBuilder {
	s.addColumn(name, alias, NONE)
	return s
}

// ColumnWithAlias adds a new column, its associated alias and the operation
// applied to that column. Available operations are expressed in terms of ColumnOp type
// and are: NONE, COUNT, MIN, MAX, AVG, SUM, UPPER, LOWER, DISTINCT.
func (s *SelectBuilder) ColumnWithOp(name, alias string, op ColumnOp) *SelectBuilder {
	s.addColumn(name, alias, op)
	return s
}

// ColumnWithOpDistinct adds a column to the SELECT statement with an optional
// alias, a specific operation (like an aggregate function), and applies the
// DISTINCT modifier to that column's operation.
func (s *SelectBuilder) ColumnWithOpDistinct(name, alias string, op ColumnOp) *SelectBuilder {
	s.addColumn(name, alias, op|DISTINCT)
	return s
}

// addTable is an internal helper method used to add a table reference to the SelectBuilder.
// It prevents adding the same table twice and sets the table's name and alias details.
func (s *SelectBuilder) addTable(table string, alias any) {
	if _, ok := s.from[table]; ok {
		return
	}

	s.from[table] = &table_ref_t{
		name:      table,
		alias:     alias,
		has_alias: alias != nil,
	}
}

// From specifies one or more tables to select data from, forming the FROM clause.
// It adds the provided tables to the builder without any aliases.
func (s *SelectBuilder) From(tables ...string) *SelectBuilder {
	for _, table := range tables {
		s.addTable(table, nil)
	}

	return s
}

// FromWithAlias specifies a single table to select data from and assigns it a specific alias.
// This is typically used for clarity or to distinguish tables in a self-join.
func (s *SelectBuilder) FromWithAlias(table, alias string) *SelectBuilder {
	s.addTable(table, alias)
	return s
}

// Join adds a new JOIN clause to the SELECT statement, specifying the table,
// an optional alias, the type of join, and the ON condition expression.
func (s *SelectBuilder) Join(target string, alias any,
	join_type JoinType, expr string) *SelectBuilder {
	// Construct and add the new join operation
	s.joins = append(s.joins, &join_t{
		join_type: join_type,
		target: &table_ref_t{
			name:      target,
			alias:     alias,
			has_alias: alias != nil,
		},
		condition: expr,
	})

	return s
}

func (s *SelectBuilder) InnerJoin(target string, alias any, expr string) *SelectBuilder {
	return s.Join(target, alias, INNER, expr)
}
func (s *SelectBuilder) LeftJoin(target string, alias any, expr string) *SelectBuilder {
	return s.Join(target, alias, LEFT, expr)
}
func (s *SelectBuilder) RightJoin(target string, alias any, expr string) *SelectBuilder {
	return s.Join(target, alias, RIGHT, expr)
}
func (s *SelectBuilder) FullJoin(target string, alias any, expr string) *SelectBuilder {
	return s.Join(target, alias, FULL, expr)
}
func (s *SelectBuilder) CrossJoin(target string, alias any, expr string) *SelectBuilder {
	return s.Join(target, alias, CROSS, expr)
}

func (s *SelectBuilder) Validate() error {
	return nil
}

func (s *SelectBuilder) Build() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}

	return "", nil
}

// Prepare finalizes the SelectBuilder and returns a prepared SQL Statement
// ready for execution, along with a function to validate argument types.
func (u *SelectBuilder) Prepare() *Statement {
	// First build the SQL query containing positional arguments ids
	// and check that there are no errors during build stage
	return nil
}
