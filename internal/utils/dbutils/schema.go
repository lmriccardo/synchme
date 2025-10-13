package dbutils

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Schema represents the entire structure of an SQLite database.
// It holds the file path to the database file and a collection of all defined tables.
type Schema struct {
	FilePath string            // In which .db or .sqlite file the schema is located
	Tables   map[string]*Table // All tables of the schema
	db       *sql.DB           // The connection with the Db
}

// SchemaBuilder is a helper structure used to programmatically construct and manage
// the definition of a database schema. It holds a reference to the schema object
// being built, allowing for fluid chaining of configuration and table creation methods.
type SchemaBuilder struct {
	sch *Schema // The object schema
}

// SQLCreate creates the CREATE statement for each table in the schema
func (s *Schema) SQLCreate() string {
	var builder strings.Builder
	for _, table := range s.Tables {
		builder.WriteString(table.SQLCreate() + "\n")
	}
	return builder.String()
}

// Close closes the database and the Schema
func (s *Schema) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// NewSchema creates an empty schema and returns both the schema
// itself and the builder for filling the schema with tables
func NewSchema(path string) (*Schema, *SchemaBuilder, error) {
	// Open the DB connection
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, nil, err
	}

	// Check if DB connection works
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	// Create the schema
	schema := &Schema{
		FilePath: path,
		Tables:   make(map[string]*Table),
		db:       db,
	}

	return schema, &SchemaBuilder{sch: schema}, nil
}

// AddTable initializes a new table definition within the current schema builder.
//
// It creates a new TableBuilder instance associated with the provided table name,
// and prepares internal data structures (such as the column list and index map)
// for defining columns fluently.
func (bld *SchemaBuilder) AddTable(tbl_name string) *TableBuilder {
	// Create the table object and add it to the schema
	table := &Table{
		Name:         tbl_name,
		Columns:      []*column_t{},
		ColumnsIndex: make(map[string]int),
	}

	bld.sch.Tables[tbl_name] = table
	return &TableBuilder{tbl: table, columnIndex: 0}
}

// Build finalizes the schema definition by executing the SQL CREATE statements
// for all tables defined within the schema builder.
//
// It iterates through each table registered in the schema and runs its
// corresponding CREATE TABLE statement against the connected SQLite database.
// If any table creation fails, Build stops immediately and returns an error
// describing which table failed and the underlying cause.
func (bld *SchemaBuilder) Build() error {
	for _, tbl := range bld.sch.Tables {
		if _, err := bld.sch.db.Exec(tbl.SQLCreate()); err != nil {
			return fmt.Errorf("creating table %s: %w", tbl.Name, err)
		}
	}
	return nil
}
