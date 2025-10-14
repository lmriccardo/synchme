package dbutils

import (
	"fmt"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
)

// String returns the string representation of the Data type
func (d DataType) String() string {
	switch d {
	case NULL:
		return "NULL"
	case INTEGER:
		return "INTEGER"
	case REAL:
		return "REAL"
	case TEXT:
		return "TEXT"
	case BLOB:
		return "BLOB"
	default:
		return fmt.Sprintf("UNKNOWN DATA TYPE(%d)", int(d))
	}
}

func (a ForeignKeyAction) String() string {
	switch a {
	case NO_ACTION:
		return ""
	case RESTRICT:
		return "RESTRICT"
	case SET_NULL:
		return "SET_NULL"
	case SET_DEFAULT:
		return "SET_DEFAULT"
	case CASCADE:
		return "CASCADE"
	default:
		return fmt.Sprintf("UNKNOWN FK ACTION(%d)", int(a))
	}
}

// String returns a formatted ASCII table describing the table schema.
// It includes the column name, type, and key/not-null/default attributes.
func (t *Table) String() string {
	if len(t.Columns) == 0 {
		return fmt.Sprintf("Table '%s' has no columns defined.\n", t.Name)
	}

	headers := []string{"Column Name", "Type", "Primary Key", "Not Null", "Default"}
	rows := make([][]string, 0, len(t.Columns)+1)
	rows = append(rows, headers)

	for _, col := range t.Columns {
		def := ""
		if col.Default != nil && col.Default != "" {
			def = fmt.Sprintf("%v", col.Default)
		}
		rows = append(rows, []string{
			col.Name,
			col.Type.String(),
			fmt.Sprintf("%v", col.PrimaryKey),
			fmt.Sprintf("%v", col.NotNull),
			def,
		})
	}

	colWidths := utils.CalcColWidths(rows)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Table: %s\n", t.Name))
	sb.WriteString(utils.MakeTopBorder(colWidths) + "\n")
	sb.WriteString(utils.FormatRow(headers, colWidths))
	sb.WriteString(utils.MakeMidBorder(colWidths) + "\n")

	for _, row := range rows[1:] {
		sb.WriteString(utils.FormatRow(row, colWidths))
	}
	sb.WriteString(utils.MakeBottomBorder(colWidths) + "\n")

	return sb.String()
}

// String returns a human-readable representation of the schema,
// displaying all tables and their column structures using
// Unicode box-drawing tables.
func (s *Schema) String() string {
	var sb strings.Builder

	// Print header
	sb.WriteString(fmt.Sprintf("Schema: %s\n\n", s.FilePath))

	if len(s.Tables) == 0 {
		sb.WriteString("(no tables defined)\n")
		return sb.String()
	}

	// Print all tables
	for _, tbl := range s.Tables {
		if tbl == nil {
			continue
		}
		sb.WriteString(tbl.String())
		sb.WriteString("\n")
	}

	return sb.String()
}
