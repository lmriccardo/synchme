package dbutils

import (
	"fmt"
	"reflect"
)

type DataType int
type ForeignKeyAction int

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

const (
	NO_ACTION ForeignKeyAction = iota
	RESTRICT
	SET_NULL
	SET_DEFAULT
	CASCADE
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
	Name          string   // The name of the column
	Type          DataType // The type of elements in the column
	PrimaryKey    bool     // This column is a primary key
	Autoincrement bool     // This column has the autoincrement flag
	NotNull       bool     // This column does not contains null values
	Unique        bool     // This column must contain only distinct values
	Default       any      // Default value for this column
}

// foreignKey_t represents a FOREIGN KEY constraint definition in an SQLite table.
//
// A foreign key enforces referential integrity between the current table
// and another referenced table. It specifies which local column(s) must
// correspond to the primary key or unique column(s) in the target table,
// as well as what actions should occur when the referenced rows are
// updated or deleted.
type foreignKey_t struct {
	Columns           []string         // The list of local column names participating in the foreign key
	RefTable          string           // The name of the referenced (parent) table.
	RefColumns        []string         // The list of column names in the referenced table.
	OnDelete          ForeignKeyAction // Action to perform when a referenced row is deleted.
	OnUpdate          ForeignKeyAction // Action to perform when a referenced row is updated.
	Deferrable        bool             // Whether enforcement of the constraint can be deferred until the end of a transaction.
	InitiallyDeferred bool             // Whether the constraint starts deferred by default when Deferrable is true.
}

// Table represents a database table schema, including its name, columns,
// and a lookup index for accessing columns by name.
type Table struct {
	Name         string          // The name of the table
	Columns      []*column_t     // The list of columns in the table
	ColumnsIndex map[string]int  // Maps columns name into indexes
	ForeignKeys  []*foreignKey_t // The list of all foreign key
	sch          *Schema         // The schema the table belongs to
}

// TableBuilder provides a fluent interface for constructing a table definition
// within a schema. It is typically obtained by calling SchemaBuilder.AddTable(),
// and allows chaining methods such as Column() to define the table structure.
type TableBuilder struct {
	Errors      []error        // A slice of errors collected during operations
	tbl         *Table         // The table object of the building
	columnIndex int            // The current index of the column
	parent      *SchemaBuilder // A pointer to the parent schema builder
}

// Column adds a new column definition to the table being built.
// It returns the *TableBuilder itself, allowing for method chaining.
func (tbld *TableBuilder) Column(name string, data_t DataType, not_null,
	unique bool, def any) *TableBuilder {

	// Create the new column to be added to the table
	column := &column_t{
		Name:    name,
		Type:    data_t,
		NotNull: not_null,
		Unique:  unique,
		Default: def,
	}

	tbld.tbl.Columns = append(tbld.tbl.Columns, column)
	tbld.tbl.ColumnsIndex[name] = tbld.columnIndex
	tbld.columnIndex++

	return tbld
}

// PrimaryKey sets the specified column as the primary key for the table.
// If the column name does not exist, an error is appended to the TableBuilder's
// Errors slice, and the builder is returned.
func (tbld *TableBuilder) PrimaryKey(name string, autoincrement bool) *TableBuilder {
	// Get the corresponding column, if it exists
	col_idx, ok := tbld.tbl.ColumnsIndex[name]

	// If the column name does not exists, append the error and returns
	if !ok {
		fmt.Println("Error: ", fmt.Errorf("column name %q does not exists", name))
		return nil
	}

	// Set the corresponding values. Validation will happen in a later step
	column := tbld.tbl.Columns[col_idx]
	column.PrimaryKey = true
	column.Autoincrement = autoincrement

	return tbld
}

// ForeignKey adds a single Foreign Key constraint
func (tbld *TableBuilder) ForeignKey(name, ref_table, ref_name string,
	on_delete, on_update ForeignKeyAction,
	deferrable, init_deferrable bool,
) *TableBuilder {

	return tbld.CompositeForeignKey([]string{name}, ref_table, []string{ref_name},
		on_delete, on_update, deferrable, init_deferrable)
}

// CompositeForeignKey defines a foreign key constraint using multiple columns.
func (tbld *TableBuilder) CompositeForeignKey(name []string, ref_table string, ref_name []string,
	on_delete, on_update ForeignKeyAction, deferrable, init_deferrable bool,
) *TableBuilder {

	tbld.tbl.ForeignKeys = append(tbld.tbl.ForeignKeys,
		&foreignKey_t{
			Columns:           name,
			RefTable:          ref_table,
			RefColumns:        ref_name,
			OnDelete:          on_delete,
			OnUpdate:          on_update,
			Deferrable:        deferrable,
			InitiallyDeferred: init_deferrable,
		},
	)

	return tbld
}
