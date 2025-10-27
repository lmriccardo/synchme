package dbutils

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Schema represents the entire structure of an SQLite database.
// It holds the file path to the database file and a collection of all defined tables.
type Schema struct {
	FilePath string            // In which .db or .sqlite file the schema is located
	Tables   map[string]*Table // All tables of the schema

	valid       bool            // If the schema is valid for any operation
	built       bool            // If the schema has been built
	db          *sql.DB         // The connection with the Db
	ctx         context.Context // The context for the DB transactions
	placeholder string          // The placeholder style
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

// Use returns a table belonging to the schema
func (s *Schema) Use(name string) (*Table, error) {
	table, ok := s.Tables[name]
	if !ok {
		return nil, fmt.Errorf("input table %q not in the schema", name)
	}
	return table, nil
}

// NewSchema creates an empty schema and returns both the schema
// itself and the builder for filling the schema with tables
func NewSchema(driver, path string, ctx context.Context) (*Schema, *SchemaBuilder, error) {
	// Open the DB connection
	db, err := sql.Open(driver, path)
	if err != nil {
		return nil, nil, err
	}

	// Check if DB connection works
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	// Check if the driver belongs to the mapping otherwise nil
	if _, ok := PLACEHOLDER[driver]; !ok {
		_ = db.Close()
		return nil, nil, fmt.Errorf("unknown driver: %s", driver)
	}

	// Create the schema
	schema := &Schema{
		FilePath:    path,
		Tables:      make(map[string]*Table),
		valid:       true,
		built:       false,
		db:          db,
		ctx:         ctx,
		placeholder: PLACEHOLDER[driver],
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

		bld.sch.valid = false
		return fmt.Errorf("schema validation failed (%d tables had errors)", len(errs))
	}

	// Use transactions to detect any error and rollback to a previous consistent state
	tx, err := bld.sch.db.BeginTx(bld.sch.ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	fk_enforced := false

	// Now use the created transaction to build all tables
	for _, tbl := range bld.sch.Tables {
		// Check for any foreign key and ensure they are active at the start
		if len(tbl.ForeignKeys) > 0 && !fk_enforced {
			_, _ = tx.ExecContext(bld.sch.ctx, "PRAGMA foreign_keys = ON;")
			fk_enforced = true
		}

		if _, err = tx.ExecContext(bld.sch.ctx, tbl.SQLCreate()); err != nil {
			_ = tx.Rollback() // Abort all transactions
			return fmt.Errorf("creating table %s: %w", tbl.Name, err)
		}
	}

	// Commit the transactions and set the built flag to true
	if err = tx.Commit(); err == nil {
		bld.sch.built = true
	}

	return err
}

// Validate validates the entire schema and returns a slice of errors
func (s *Schema) Validate() (errs []error) {
	for _, table := range s.Tables {
		if table_errs := table.Validate(); len(table_errs) > 0 {
			for _, err := range table_errs {
				fmt.Println(err)
			}

			errs = append(errs, fmt.Errorf(
				"validation failed for table %q", table.Name))
		}
	}

	return
}

// SQLCreate creates the CREATE statement for each table in the schema
func (s *Schema) SQLCreate() string {
	var builder strings.Builder
	for _, table := range s.Tables {
		builder.WriteString(table.SQLCreate() + "\n")
	}
	return builder.String()
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

// GetPlaceholder returns the dialect-specific placeholder for a given argument index.
func (s *Schema) GetPlaceholder(index int) string {
	pl := s.placeholder
	if pl == "?" {
		return pl
	}

	return fmt.Sprintf(pl, index)
}

// Insert initializes and returns a new SelectBuilder instance for constructing
// an SQL SELECT statement against the associated schema.
func (s *Schema) Select() *SelectBuilder {
	builder := &SelectBuilder{
		columns: make(map[string]*select_column_t),
		from:    make(map[string]*table_ref_t),
		schema:  s,
		range_clause_t: range_clause_t{
			order: make(map[string]OrderByType),
		},
	}

	builder.parent = builder

	return builder
}
