package dbutils

import (
	"fmt"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
)

// SQLCreate formats the column definition into a valid SQL fragment for CREATE TABLE.
func (c *column_t) SQLCreate() string {
	var builder strings.Builder

	// Column name and type
	builder.WriteString(fmt.Sprintf("\t%s %s", c.Name, c.Type.String()))

	// Primary Key
	if c.PrimaryKey {
		builder.WriteString(" PRIMARY KEY")

		// AUTOINCREMENT must come immediately after PRIMARY KEY if present
		if c.Autoincrement {
			builder.WriteString(" AUTOINCREMENT")
		}
	}

	// NOT NULL
	if c.NotNull {
		builder.WriteString(" NOT NULL")
	}

	// UNIQUE
	if c.Unique {
		builder.WriteString(" UNIQUE")
	}

	// DEFAULT value
	if c.Default != nil {
		builder.WriteString(" DEFAULT ")
		switch v := c.Default.(type) {
		case string:
			// Wrap string literals in single quotes, escape internal quotes
			escaped := strings.ReplaceAll(v, "'", "''")
			builder.WriteString(fmt.Sprintf("'%s'", escaped))

		case bool:
			// Booleans in SQLite are represented as 1/0
			if v {
				builder.WriteString("1")
			} else {
				builder.WriteString("0")
			}

		case nil:
			builder.WriteString("NULL")

		default:
			// Numeric and other literal types
			builder.WriteString(fmt.Sprintf("%v", v))
		}
	}

	return builder.String()
}

// SQLCreate formats the foreign key constraint into a valid SQL fragment for CREATE TABLE.
func (fk *foreignKey_t) SQLCreate() string {
	var builder strings.Builder

	init_str := "\tFOREIGN KEY (%s) REFERENCES %s(%s)"
	builder.WriteString(fmt.Sprintf(init_str,
		strings.Join(fk.Columns, ", "),
		fk.RefTable,
		strings.Join(fk.RefColumns, ", ")))

	if fk.OnDelete != NO_ACTION {
		builder.WriteString(fmt.Sprintf(" ON DELETE %s", fk.OnDelete.String()))
	}

	if fk.OnUpdate != NO_ACTION {
		builder.WriteString(fmt.Sprintf(" ON UPDATE %s", fk.OnUpdate.String()))
	}

	if fk.Deferrable {
		builder.WriteString(" DEFERRABLE")
		if fk.InitiallyDeferred {
			builder.WriteString(" INITIALLY DEFERRED")
		}
	} else {
		builder.WriteString(" NOT DEFERRABLE")
	}

	return builder.String()
}

// SQLCreate creates the CREATE statement for current table
func (t *Table) SQLCreate() string {
	var rows []string
	for _, col := range t.Columns {
		rows = append(rows, col.SQLCreate())
	}

	for _, fk := range t.ForeignKeys {
		rows = append(rows, fk.SQLCreate())
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);\n",
		t.Name, strings.Join(rows, ",\n"))
}

// SQLCreate creates the CREATE statement for each table in the schema
func (s *Schema) SQLCreate() string {
	var builder strings.Builder
	for _, table := range s.Tables {
		builder.WriteString(table.SQLCreate() + "\n")
	}
	return builder.String()
}

// ToSQLString creates the SQL string releated to this condition group
func (c *condition_group_t) ToSQLString() string {
	var builder strings.Builder
	op_str := fmt.Sprintf(" %s ", c.Op.String())
	conditions := c.Conds
	conditions = append(conditions,
		utils.Map(func(c *condition_group_t) string {
			return c.ToSQLString()
		}, c.SubGroups)...,
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
	if len(w.root.Conds) > 0 || len(w.root.SubGroups) > 0 {
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
	if len(r.Order) > 0 {
		builder.WriteString("ORDER BY ")
		orders := []string{}
		for name, direction := range r.Order {
			orders = append(orders, fmt.Sprintf("%s %s", name, direction))
		}
		builder.WriteString(strings.Join(orders, ", "))
		builder.WriteString("\n")
	}

	// Adds LIMIT if it has been set
	if r.LimitSet {
		builder.WriteString(fmt.Sprintf("LIMIT %d\n", r.LimitValue))
	}

	// Adds OFFSET if it has been set
	if r.OffsetSet {
		builder.WriteString(fmt.Sprintf("OFFSET %d", r.OffsetValue))
	}

	return builder.String()
}

// SqlLiteral converts a Go value into its appropriate SQL literal string representation.
// This function is generally used for embedding constants directly into a query (NOT recommended
// for user-supplied data, which should use prepared statements).
func SqlLiteral(value any) string {
	switch x := value.(type) {
	case string:
		// Here we need to correctly format string aroung quotes
		return fmt.Sprintf("'%s'", strings.ReplaceAll(x, "'", "''"))
	case bool:
		if x {
			return "1"
		}
		return "0"
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("%v", x)
	}
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
	for name, value := range u.Columns {
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
