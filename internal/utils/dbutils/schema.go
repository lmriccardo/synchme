package dbutils

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Schema represents the entire structure of an SQLite database.
// It holds the file path to the database file and a collection of all defined tables.
type Schema struct {
	FilePath string            // In which .db or .sqlite file the schema is located
	Tables   map[string]*Table // All tables of the schema
	IsValid  bool              // If the schema is valid for any operation
	IsBuilt  bool              // If the schema has been built
	db       *sql.DB           // The connection with the Db
}

// SchemaBuilder is a helper structure used to programmatically construct and manage
// the definition of a database schema. It holds a reference to the schema object
// being built, allowing for fluid chaining of configuration and table creation methods.
type SchemaBuilder struct {
	sch *Schema // The object schema
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
		IsValid:  true,
		IsBuilt:  false,
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
		sch:          bld.sch,
	}

	bld.sch.Tables[tbl_name] = table
	return &TableBuilder{tbl: table, columnIndex: 0, parent: bld}
}

// Build finalizes the schema definition by executing the SQL CREATE statements
// for all tables defined within the schema builder.
//
// It iterates through each table registered in the schema and runs its
// corresponding CREATE TABLE statement against the connected SQLite database.
// If any table creation fails, Build stops immediately and returns an error
// describing which table failed and the underlying cause.
func (bld *SchemaBuilder) Build() error {
	// First validate the entire schema, if there are errors print them
	// and exit immediately without building anything
	if errs := bld.sch.Validate(); len(errs) > 0 {
		for _, err := range errs {
			fmt.Println(err)
		}

		bld.sch.IsValid = false
		return fmt.Errorf("schema validation failed (%d tables had errors)", len(errs))
	}

	for _, tbl := range bld.sch.Tables {
		fmt.Println(tbl.SQLCreate())
		if _, err := bld.sch.db.Exec(tbl.SQLCreate()); err != nil {
			return fmt.Errorf("creating table %s: %w", tbl.Name, err)
		}
	}

	bld.sch.IsBuilt = true
	return nil
}
