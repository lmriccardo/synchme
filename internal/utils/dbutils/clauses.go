package dbutils

import (
	"container/list"
	"fmt"
	"regexp"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
)

// ConditionGroup represents a logical grouping of SQL WHERE conditions.
// It allows for combining multiple raw conditions and/or nested subgroups
// using a logical operator (AND, OR, NOT).
type condition_group_t struct {
	op         WhereOpType          // The logical connection (AND OR)
	conds      []string             // Raw conditions
	parameters []string             // The parameters of all conditions
	subGroups  []*condition_group_t // Nested subgroup of conditions
	negated    bool                 // The entire condition group is negated

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
	order       map[string]OrderByType // Maps columns to order direction
	limiteValue int                    // The value with which limiting the selection
	limitSet    bool                   // If the limit value has been set
	offsetValue int                    // Values from which to start selecting
	offsetSet   bool                   // If the offset value has been set

	parent Rangeable // The parent of this clause
}

// And returns an new condition group for chaining AND conditions
func (c *condition_group_t) And() *condition_group_t {
	return &condition_group_t{op: AND, parent: c}
}

// Or returns an new condition group for chaining OR conditions
func (c *condition_group_t) Or() *condition_group_t {
	return &condition_group_t{op: OR, parent: c}
}

// NotAnd returns an new condition group for chaining AND conditions
// that is globally negated
func (c *condition_group_t) NotAnd() *condition_group_t {
	g := c.And()
	g.negated = true
	return g
}

// NotOr returns an new condition group for chaining OR conditions
// that is globally negated
func (c *condition_group_t) NotOr() *condition_group_t {
	g := c.Or()
	g.negated = true
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
	c.parameters = append(c.parameters, ids...)
	c.conds = append(c.conds, expr)

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
		c.parent.subGroups = append(c.parent.subGroups, c)
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
		w.root = &condition_group_t{op: AND, where: w}
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
	if _, ok := r.order[name]; !ok {
		r.order[name] = dir
	}

	return r.parent
}

// Limit sets the maximum number of rows to be returned by the query.
// The value is overwritten on every call to this method.
func (r *range_clause_t) Limit(value int) Rangeable {
	// Value are overwritten every time the operation is performed
	r.limiteValue = value
	r.limitSet = true
	return r.parent
}

// Offset sets the number of rows to skip before starting to return results.
// The value is overwritten on every call to this method.
func (r *range_clause_t) Offset(value int) Rangeable {
	// Value are overwritten every time the operation is performed
	r.offsetValue = value
	r.offsetSet = true
	return r.parent
}

// ToSQLString creates the SQL string releated to this condition group
func (c *condition_group_t) ToSQLString() string {
	var builder strings.Builder
	op_str := fmt.Sprintf(" %s ", c.op.String())
	conditions := c.conds
	conditions = append(conditions,
		utils.Map(func(c *condition_group_t) string {
			return c.ToSQLString()
		}, c.subGroups)...,
	)

	conditions_s := fmt.Sprintf("(%s)", strings.Join(
		conditions, op_str))

	builder.WriteString(conditions_s)
	return builder.String()
}

// ToSQLString creates the SQL string releated to WHERE clause
func (w *where_clause_t) ToSQLString() string {
	var builder strings.Builder

	// Check that there is at least one condition
	if len(w.root.conds) > 0 || len(w.root.subGroups) > 0 {
		condition_s := w.root.ToSQLString()
		builder.WriteString(fmt.Sprintf("WHERE %s", condition_s))
	}

	return builder.String()
}

// ToSQLString creates the SQL string releated to ORDER BY, LIMIT
// and OFFSET clauses
func (r *range_clause_t) ToSQLString() string {
	var builder strings.Builder

	// Adds ORDER BY if there are entries in the map
	if len(r.order) > 0 {
		builder.WriteString("ORDER BY ")
		orders := []string{}
		for name, direction := range r.order {
			orders = append(orders, fmt.Sprintf("%s %s", name, direction))
		}
		builder.WriteString(strings.Join(orders, ", "))
		builder.WriteString("\n")
	}

	// Adds LIMIT if it has been set
	if r.limitSet {
		builder.WriteString(fmt.Sprintf("LIMIT %d\n", r.limiteValue))
	}

	// Adds OFFSET if it has been set
	if r.offsetSet {
		builder.WriteString(fmt.Sprintf("OFFSET %d", r.offsetValue))
	}

	return builder.String()
}

// ValidateColumns validates all the columns used in the WHERE operation
// and returns a list of errors for each column
func (w *where_clause_t) ValidateColumns(t *Table) (errs []error) {
	// Create the column pattern for finding columns in conditions
	pattern := regexp.MustCompile(
		`\b([a-zA-Z_][a-zA-Z0-9_\.]*)\b\s*` +
			`(?:=|!=|<>|<|>|<=|>=|LIKE|IN|IS\s+(?:NOT\s+)?NULL|BETWEEN)`,
	)

	// Create a new double linked-list
	queue := list.New()
	queue.PushBack(w.root)

	// Use a BFS to collect all conditions from the AST
	for queue.Len() > 0 {
		// Get the first element and remove it from the list
		element := queue.Front()
		queue.Remove(element)
		group := element.Value.(*condition_group_t)

		// Extract columns referenced in each condition and validate them
		for _, condition := range group.conds {
			errs = append(errs, validateConditionColumns(condition, t, pattern)...)
		}

		for _, subgroup := range group.subGroups {
			queue.PushBack(subgroup)
		}
	}

	return
}
