package dbutils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
	_ "github.com/mattn/go-sqlite3"
)

type Buildable interface {
	Validate() error        // Validates the buildable before building it
	Build() (string, error) // Returns the string query of the operation
	Prepare() (*Statement, TypeValidatorFunc)
}

type Rangeable interface {
	Buildable
	Limit(int) Rangeable
	Offset(int) Rangeable
	OrderBy(string, OrderByType) Rangeable
}

// ConditionGroup represents a logical grouping of SQL WHERE conditions.
// It allows for combining multiple raw conditions and/or nested subgroups
// using a logical operator (AND, OR, NOT).
type condition_group_t struct {
	Op         WhereOpType          // The logical connection (AND OR)
	Conds      []string             // Raw conditions
	Parameters []string             // The parameters of all conditions
	SubGroups  []*condition_group_t // Nested subgroup of conditions
	Negated    bool                 // The entire condition group is negated

	parent *condition_group_t // Parent condition group of this group
	where  *where_clause_t    // Parent where clause (only for the root group)
}

// whereClause_t encapsulates the entire WHERE clause structure for a
// database operation. It also holds a reference to the parent database
// operation to allow operation chaining fluently.
type where_clause_t struct {
	root   *condition_group_t // The top-level condition group
	parent Rangeable          // The parent of the where clause
}

// range_clause_t encapsulates the SQL clauses used to define the order and extent
// (i.e., the range) of the result set, specifically ORDER BY, LIMIT, and OFFSET.
type range_clause_t struct {
	Order       map[string]OrderByType // Maps columns to order direction
	LimitValue  int                    // The value with which limiting the selection
	LimitSet    bool                   // If the limit value has been set
	OffsetValue int                    // Values from which to start selecting
	OffsetSet   bool                   // If the offset value has been set

	parent Rangeable // The parent of this clause
}

type preparator_t struct {
	sql   string // The SQL string produced by a previous Build
	table *Table // The table subject to the current operation
}

// UpdateBuilder is a fluent interface structure used to construct an SQL UPDATE statement.
// It tracks the columns and values to be set, and embeds structures for defining
// the WHERE clause and any limiting/ordering clauses.
type UpdateBuilder struct {
	Columns    map[string]any // Columns affected by the update operation
	Parameters []string       // Parameters for each column (actually the column name)

	table *Table // The table the current operation is updating

	where_clause_t // Embedded structure for building the WHERE clause.
	range_clause_t // Embedded structure for building the ORDER BY, LIMIT and OFFSET clauses.
	preparator_t   // Embedded structure for preparing SQL statements
}

type SelectBuilder struct {
	// table *Table // The table the current operation is updating

	where_clause_t // Embedded structure for building the WHERE clause.
	range_clause_t // Embedded structure for building the ORDER BY, LIMIT and OFFSET clauses.
	preparator_t   // Embedded structure for preparing SQL statements
}

// And returns an new condition group for chaining AND conditions
func (c *condition_group_t) And() *condition_group_t {
	return &condition_group_t{Op: AND, parent: c}
}

// Or returns an new condition group for chaining OR conditions
func (c *condition_group_t) Or() *condition_group_t {
	return &condition_group_t{Op: OR, parent: c}
}

// NotAnd returns an new condition group for chaining AND conditions
// that is globally negated
func (c *condition_group_t) NotAnd() *condition_group_t {
	g := c.And()
	g.Negated = true
	return g
}

// NotOr returns an new condition group for chaining OR conditions
// that is globally negated
func (c *condition_group_t) NotOr() *condition_group_t {
	g := c.Or()
	g.Negated = true
	return g
}

// Cond adds a new raw condition expression to the current ConditionGroup.
// It can parse named parameters used in the expression in the form :<param_name>.
func (c *condition_group_t) Cond(expr string) *condition_group_t {
	// First we must compile the regex to find the param ids
	param_ids_re := regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)

	// Find all substrings that matches the pattern
	matches := param_ids_re.FindAllStringSubmatch(expr, -1)
	ids := utils.Map(func(ss []string) string { return ss[1] }, matches)
	c.Parameters = append(c.Parameters, ids...)
	c.Conds = append(c.Conds, expr)

	return c
}

// Not adds a negated condition to the current ConditionGroup.
// It wraps the provided expression in "NOT (...)".
func (c *condition_group_t) Not(expr string) *condition_group_t {
	return c.Cond(fmt.Sprintf("NOT (%s)", expr))
}

// EndGroup concludes the condition building for the current group. If the group
// is the top-level one it returns nil and the program breaks
func (c *condition_group_t) EndGroup() *condition_group_t {
	if c.parent != nil {
		// When going up one level on the AST tree of the WHERE
		// clause, we have to embed this group into the parent
		c.parent.SubGroups = append(c.parent.SubGroups, c)
		return c.parent
	}

	return nil
}

// EndWhere concludes the condition building for the current group and
// return the WHERE parent actually ending also the WHERE clause. If the
// group is not the top-level one it returns nil and the program breaks
func (c *condition_group_t) EndWhere() Rangeable {
	return c.where.End()
}

// Where initializes and returns the top-level ConditionGroup for building
// the WHERE clause. If the clause has not been started, it initializes it
// with a default logical operator (usually AND). This method is used to begin
// or continue chaining condition methods.
func (w *where_clause_t) Where() *condition_group_t {
	// Initialize a new Conditional group if nil
	if w.root == nil {
		w.root = &condition_group_t{Op: AND, where: w}
	}

	return w.root
}

// End signals the completion of the WHERE clause building process.
// It returns the parent DB_Operation (e.g., SELECT, UPDATE) to allow
// for further method chaining on the main operation object.
func (w *where_clause_t) End() Rangeable {
	return w.parent
}

// OrderBy sets the sorting direction (ascending or descending) for a
// specified column. It ensures that a column is added to the ORDER BY
// list only once.
func (r *range_clause_t) OrderBy(name string, dir OrderByType) Rangeable {
	// Check if the name does not already exists in the map and set it
	if _, ok := r.Order[name]; !ok {
		r.Order[name] = dir
	}

	return r.parent
}

// Limit sets the maximum number of rows to be returned by the query.
// The value is overwritten on every call to this method.
func (r *range_clause_t) Limit(value int) Rangeable {
	// Value are overwritten every time the operation is performed
	r.LimitValue = value
	r.LimitSet = true
	return r.parent
}

// Offset sets the number of rows to skip before starting to return results.
// The value is overwritten on every call to this method.
func (r *range_clause_t) Offset(value int) Rangeable {
	// Value are overwritten every time the operation is performed
	r.OffsetValue = value
	r.OffsetSet = true
	return r.parent
}

// Prepare analyzes the embedded SQL string (`p.sql`) to identify named parameters
// and the database columns they reference. It prepares the underlying SQL statement
// and constructs a mapping from named parameters to their positional indices
// within the prepared statement.
func (p *preparator_t) Prepare() (*Statement, TypeValidatorFunc) {
	// First thing, given the SQL string it shall extract a mapping between
	// parameters and referenced columns
	pattern := regexp.MustCompile(
		`\b([a-zA-Z_][a-zA-Z0-9_\.]*)\b\s*` +
			`(?:=|!=|<>|<|>|<=|>=|LIKE|IN|IS\s+(?:NOT\s+)?NULL|BETWEEN)\s*` +
			`(:[a-zA-Z_][a-zA-Z0-9_]*|\S+)`,
	)

	// Initialize the statement object to be returned
	statement := &Statement{positionalMap: map[string][]int{}}
	columns := make(map[string]string)
	matches := pattern.FindAllStringSubmatch(p.sql, -1)
	sql_query := p.sql

	for _, match := range matches {
		// Take the column name and the parameter name
		col_name := match[0]
		par_name := match[2]

		if after, ok := strings.CutPrefix(par_name, ":"); ok {
			// Adds to the list of all positions releated to the current
			// parameter, the last possible position
			statement.positionalMap[after] = append(
				statement.positionalMap[after],
				statement.nofArgs,
			)

			statement.nofArgs++

			// Add the mapping parameter to column name
			columns[after] = col_name
			sql_query = strings.ReplaceAll(sql_query, par_name, "?")
		}
	}

	fmt.Println(sql_query)
	stmt, err := p.table.sch.db.Prepare(sql_query)
	if err != nil {
		fmt.Println("Error when preparing the statement: ", err)
		return nil, nil
	}

	statement.stmt = stmt
	return statement, func(ps map[string]any) (bool, []error) {
		return ValidateTypes(columns, ps, p.table)
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
		if _, ok := u.Columns[column]; !ok {
			u.Columns[column] = "?"
			u.Parameters = append(u.Parameters, column)
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
	if _, ok := u.Columns[name]; !ok {
		u.Columns[name] = value
	}

	return u
}

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
