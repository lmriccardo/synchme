package dbutils

import (
	"fmt"
	"strings"
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
