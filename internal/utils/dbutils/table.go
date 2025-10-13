package dbutils

import (
	"fmt"
	"reflect"
	"strings"
)

type DataType int

// SQLite provides five primary data types such as NULL,
// INTEGER, REAL, TEXT, and BLOB each of them is used
// for distinct purposes.
const (
	NULL DataType = iota
	INTEGER
	REAL
	TEXT
	BLOB
)

// Maps Go type into SQL TYPES
var TYPE_MAP = map[reflect.Kind]DataType{
	reflect.Bool:    INTEGER,
	reflect.Int:     INTEGER,
	reflect.Int8:    INTEGER,
	reflect.Int16:   INTEGER,
	reflect.Int32:   INTEGER,
	reflect.Int64:   INTEGER,
	reflect.Uint:    INTEGER,
	reflect.Uint8:   INTEGER,
	reflect.Uint16:  INTEGER,
	reflect.Uint32:  INTEGER,
	reflect.Uint64:  INTEGER,
	reflect.Float32: REAL,
	reflect.Float64: REAL,
	reflect.String:  TEXT,
}

// column_t represents the schema definition for a single database column.
type column_t struct {
	Name       string   // The name of the column
	Type       DataType // The type of elements in the column
	PrimaryKey bool     // This column is a primary key
	NotNull    bool     // This column does not contains null values
	Default    any      // Default value for this column
}

// Table represents a database table schema, including its name, columns,
// and a lookup index for accessing columns by name.
type Table struct {
	Name         string         // The name of the table
	Columns      []*column_t    // The list of columns in the table
	ColumnsIndex map[string]int // Maps columns name into indexes
}

// TableBuilder provides a fluent interface for constructing a table definition
// within a schema. It is typically obtained by calling SchemaBuilder.AddTable(),
// and allows chaining methods such as Column() to define the table structure.
type TableBuilder struct {
	tbl         *Table // The table object of the building
	columnIndex int    // The current index of the column
}

// SQLCreate formats the column string to be used in the table CREATE statement
func (c *column_t) SQLCreate() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("\t%s %s", c.Name, c.Type.String()))

	// Write the primary key condition if required
	if c.PrimaryKey {
		builder.WriteString(" PRIMARY KEY")
	}

	// Write the NOT NULL condition if required
	if c.NotNull {
		builder.WriteString(" NOT NULL")
	}

	// TODO: Write the default value

	return builder.String()
}

// Column adds a new column definition to the table being built.
// It returns the *TableBuilder itself, allowing for method chaining.
func (tbld *TableBuilder) Column(name string, data_t DataType, pk,
	not_null bool, def any) *TableBuilder {

	// Check that the default type maps exactly the input data type
	if def != nil {
		t := reflect.TypeOf(def)
		dt, ok := TYPE_MAP[t.Kind()]
		if !ok {
			panic(fmt.Sprintf("unsupported default type for column %q: %v", name, t))
		}
		if dt != data_t {
			panic(fmt.Sprintf("default type mismatch for column %q: expected %s but got %s",
				name, data_t.String(), dt.String()))
		}
	}

	// Create the new column to be added to the table
	tbld.tbl.Columns = append(tbld.tbl.Columns, &column_t{
		Name:       name,
		Type:       data_t,
		PrimaryKey: pk,
		NotNull:    not_null,
		Default:    def,
	})

	tbld.tbl.ColumnsIndex[name] = tbld.columnIndex
	tbld.columnIndex++

	return tbld
}

// SQLCreate creates the CREATE statement for current table
func (t *Table) SQLCreate() string {
	var cols []string
	for _, col := range t.Columns {
		cols = append(cols, col.SQLCreate())
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);\n",
		t.Name, strings.Join(cols, ",\n"))
}
