package dbutils

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
)

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

// Validate performs a series of semantic and structural checks on a column definition
// to ensure that it is valid and consistent with SQLite’s rules and constraints.
// The method returns a slice of errors describing all validation issues found.
// If the returned slice is empty, the column definition is considered valid.
func (c *column_t) Validate() (errs []error) {
	// The column name cannot be empty
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, fmt.Errorf("column name cannot be empty"))
	}

	// Check that the default type maps exactly the input data type
	if c.Default != nil {
		if err := checkValueType(c.Default, c); err != nil {
			errs = append(errs, err)
		}

		// The column cannot be a primary key, unsual and might break
		// autoincrement semantics
		if c.PrimaryKey {
			err := fmt.Errorf("column %q: PRIMARY KEY columns should not have DEFAULT values", c.Name)
			errs = append(errs, err)
		}

		// Check for disallowed defaults on BLOB or complex types
		if c.Type == BLOB {
			err := fmt.Errorf("BLOB column %q cannot have a DEFAULT value in SQLite", c.Name)
			errs = append(errs, err)
		}
	}

	// Check for autoincrement conditions. It can only be used with
	// primary key active and data type = INTEGER. It should not be used
	// with any other column condition
	if c.Autoincrement {
		// Default value check on the autoincrement column
		if c.Default != nil {
			err := fmt.Errorf("AUTOINCREMENT column %q cannot have a DEFAULT value", c.Name)
			errs = append(errs, err)
		}

		// Type and key check on the autoincrement column
		if !c.PrimaryKey || c.Type != INTEGER {
			err := fmt.Errorf(
				"AUTOINCREMENT only allowed on INTEGER PRIMARY KEY columns (column: %s)",
				c.Name)

			errs = append(errs, err)
		}

		// UNIQUE is redundant and invalid in this context
		if c.Unique {
			err := fmt.Errorf("AUTOINCREMENT column %q cannot also be declared UNIQUE", c.Name)
			errs = append(errs, err)
		}
	}

	return
}

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

// Validate performs sanity check on the current foreign key structure
// and returns a list of errors.
func (fk *foreignKey_t) Validate() (errs []error) {
	// Sanity check on the number of columns and referenced columns
	if len(fk.Columns) != len(fk.RefColumns) {
		err := fmt.Errorf("foreign key mismatch: %d local cols vs %d ref cols",
			len(fk.Columns), len(fk.RefColumns))

		errs = append(errs, err)
	}

	// Check that the table name is not an empty string
	if fk.RefTable == "" {
		err := fmt.Errorf("foreign key error: table name cannot be empty")
		errs = append(errs, err)
	}

	// Check that none of the column names is empty
	for idx, col_name := range fk.Columns {
		ref_col_name := fk.RefColumns[idx]
		if col_name == "" || ref_col_name == "" {
			err := fmt.Errorf("foreign key error: columns name cannot be empty")
			errs = append(errs, err)
		}
	}

	return
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

// AutoID creates an auto-incrementing INTEGER 'id' column and sets it as the primary key.
// This method is a convenience wrapper for adding the most common type of
// surrogate primary key to a table, ensuring unique and non-null identification
// for every row.
func (tbld *TableBuilder) AutoID() *TableBuilder {
	// Create a deafult INTEGER id column as primary key
	return tbld.Column("id", INTEGER, true, false, nil).
		PrimaryKey("id", true)
}

// Text creates a TEXT column named with the input value which is NOT NULL,
// NOT UNIQUE and does not have any default value.
func (tbld *TableBuilder) Text(name string) *TableBuilder {
	return tbld.Column(name, TEXT, true, false, nil)
}

// Text creates a TEXT column named with the input value which is NOT NULL,
// NOT UNIQUE and has the input default value.
func (tbld *TableBuilder) TextWithDefault(name, def string) *TableBuilder {
	return tbld.Column(name, TEXT, true, false, def)
}

// IntegerWithDefault creates an INTEGER column named with the input value which
// is NOT NULL, NOT UNIQUE and has the input default value
func (tbld *TableBuilder) IntegerWithDefault(name string, def int) *TableBuilder {
	return tbld.Column(name, INTEGER, true, false, def)
}

// IntegerWithDefault creates an INTEGER column named with the input value which
// is NOT NULL, NOT UNIQUE and does not have any default value
func (tbld *TableBuilder) Integer(name string) *TableBuilder {
	return tbld.Column(name, INTEGER, true, false, nil)
}

// Blob creates a BLOB column named with the input value which can be NULL,
// it is NOT UNIQUE, and does not have any default value
func (tbld *TableBuilder) Blob(name string) *TableBuilder {
	return tbld.Column(name, BLOB, false, false, nil)
}

// RowScanner is a helper struct designed to facilitate the scanning of a single
// row result retrieved from a database query, typically wrapping *sql.Row.
type RowScanner struct {
	rows    *sql.Rows // The result from Query operations
	columns []string  // The slice of all table columns
}

// Next attempts to advance the scanner to the next result row from the database
// and processes it into a structured *Row object. This method encapsulates the logic
// for iterating through the result set and converting the raw database data into a
// map structure suitable for easy access.
func (rs *RowScanner) Next() (*Row, bool, error) {
	// If the next operation returns false, then it means that
	// there are no more rows, or an error occurred
	if !rs.rows.Next() {
		return nil, false, rs.rows.Err()
	}

	// Fill the values by scanning the current row. First initialize
	// two vectors: the first one will contains the actual value,
	// while the second one will only contains the pointers to values
	col_values := make([]any, len(rs.columns))
	col_values_ptr := make([]any, len(rs.columns))
	for idx := range col_values {
		col_values_ptr[idx] = &col_values[idx]
	}

	// Scan the current selected row
	if err := rs.rows.Scan(col_values_ptr...); err != nil {
		return nil, false, err
	}

	// Create the mapping between column and values
	row := &Row{row: make(map[string]any)}
	for idx, column := range rs.columns {
		curr_value := col_values[idx]
		if bytes, ok := curr_value.([]byte); ok {
			curr_value = string(bytes)
		}
		row.row[column] = curr_value
	}

	return row, true, nil
}

// Close closes the row scanner handler
func (rs *RowScanner) Close() {
	utils.ErrorHandler(rs.rows.Close)
}

// ValueOf returns the value associated with the input column name
func (r *Row) ValueOf(name string) (any, error) {
	curr_row_value, ok := r.row[name]
	if !ok {
		return nil, fmt.Errorf("unmatched column name %q", name)
	}
	return curr_row_value, nil
}

type Row struct {
	row map[string]any // The values for each column of the current row
}

// Values returns all values for this row
func (r *Row) Values() map[string]any {
	return r.row
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

// typeCompatibilityCheck validates that the data types of the local columns
// and the referenced columns in a foreign key constraint are identical. It
// iterates through the column pairs defined in the foreign key object (fk)
// and compares their respective data types.
func (t *Table) typeCompatibilityCheck(fk *foreignKey_t) (errs []error) {
	ref_table := t.sch.Tables[fk.RefTable]
	for i, localName := range fk.Columns {
		localCol := t.Columns[t.ColumnsIndex[localName]]
		refCol := ref_table.Columns[ref_table.ColumnsIndex[fk.RefColumns[i]]]

		if localCol.Type != refCol.Type {
			errs = append(errs, fmt.Errorf(
				"type mismatch in foreign key between %q.%q (%s) and %q.%q (%s)",
				t.Name, localCol.Name, localCol.Type,
				ref_table.Name, refCol.Name, refCol.Type,
			))
		}
	}

	return
}

// uniqueKeyCheck validates that all columns referenced by the foreign key
// in the target table are defined as either a Primary Key or a Unique
// constraint. If one or more non-unique columns are found, an error is returned
// detailing the names of the invalid columns. Returns nil if the constraint is valid.
func (t *Table) uniqueKeyCheck(fk *foreignKey_t) error {
	ref_table := t.sch.Tables[fk.RefTable]
	isNonUniqueColumn := func(c string) bool {
		curr_col := ref_table.Columns[ref_table.ColumnsIndex[c]]
		return !curr_col.PrimaryKey && !curr_col.Unique
	}

	unique_cols := utils.Filter(isNonUniqueColumn, fk.RefColumns)
	if len(unique_cols) > 0 {
		return fmt.Errorf(
			"foreign key in table %q references non-unique columns in table %q: %v",
			t.Name, ref_table.Name, unique_cols)
	}

	return nil
}

// checkFKAction validates that the specified foreign key actions are compatible
// with the column definitions in the current table.
//
// Specifically, it checks for two scenarios:
//  1. If 'ON DELETE SET NULL' is specified (fk.OnDelete is SET_NULL), it ensures
//     that none of the foreign key columns (fk.Columns) are defined as NOT NULL.
//  2. If 'ON DELETE SET DEFAULT' is specified (fk.OnDelete is SET_DEFAULT), it
//     ensures that all foreign key columns have a defined default value.
func (t *Table) checkFKAction(fk *foreignKey_t) (errs []error) {
	for _, colName := range fk.Columns {
		switch fk.OnDelete {
		case SET_NULL:
			col := t.Columns[t.ColumnsIndex[colName]]
			if !col.NotNull {
				continue
			}

			errs = append(errs, fmt.Errorf(
				"ON DELETE SET NULL invalid for NOT NULL column %q in table %q",
				col.Name, t.Name))

		case SET_DEFAULT:
			col := t.Columns[t.ColumnsIndex[colName]]
			if col.Default != nil {
				continue
			}

			errs = append(errs, fmt.Errorf(
				"ON DELETE SET DEFAULT requires DEFAULT for column %q in table %q",
				col.Name, t.Name))
		}
	}

	return
}

// checkOnePK checks if the table has only one primary key
func (t *Table) checkOnePK() error {
	cols := utils.Filter(func(c *column_t) bool { return c.PrimaryKey }, t.Columns)
	if len(cols) > 1 {
		return errors.New("table can have only one primary key")
	}
	return nil
}

func (t *Table) Validate() (errs []error) {
	// First validate all columns and collect all errors
	for _, column := range t.Columns {
		errs = append(errs, column.Validate()...)
	}

	// Check the uniqueness of the primary key columns
	if err := t.checkOnePK(); err != nil {
		errs = append(errs, err)
	}

	// Then validate the foreign keys and collect all errors
	for _, fk := range t.ForeignKeys {
		// Simple sanity check for foreign keys
		errs = append(errs, fk.Validate()...)

		// Now we need to validate foreign keys with respect to table columns
		// and the entire schema, since it references external tables and
		// corresponding columns that must both exists in the schema.
		existsColumns := func(cols []string, t *Table) (res bool) {
			res = true // initialize the return value
			for _, name := range cols {
				// If the column name does not exists in the input table
				// append an error to the list of the cumulative errors
				if _, res = t.ColumnsIndex[name]; !res {
					err := fmt.Errorf("column %s not in table %s", name, t.Name)
					errs = append(errs, err)
				}
			}
			return
		}

		// First check on columns of the curren table
		local_ok := existsColumns(fk.Columns, t)

		// Check the reference table if exists. In case of positive
		// result than we can check the referenced columns
		ref_table, ok := t.sch.Tables[fk.RefTable]
		if !ok {
			err := fmt.Errorf("referenced table %s not in the schema", t.Name)
			errs = append(errs, err)
			continue
		}

		ref_ok := existsColumns(fk.RefColumns, ref_table)
		if !local_ok || !ref_ok {
			continue // Skip deep validation if basic structure is broken
		}

		// Type compatibility check
		errs = append(errs, t.typeCompatibilityCheck(fk)...)

		// Unique or Primary Key column check
		if unique_err := t.uniqueKeyCheck(fk); unique_err != nil {
			errs = append(errs, unique_err)
		}

		// Validate Foreign Key actions against current table columns
		errs = append(errs, t.checkFKAction(fk)...)

		// Validate deferrable consistency
		if fk.InitiallyDeferred && !fk.Deferrable {
			errs = append(errs, fmt.Errorf(
				"foreign key in table %q marked as INITIALLY DEFERRED but not DEFERRABLE",
				t.Name,
			))
		}

		// Detect circular dependencies (simple heuristic)
		if fk.RefTable == t.Name {
			errs = append(errs, fmt.Errorf(
				"self-referencing foreign key detected in table %q", t.Name,
			))
		}
	}

	return
}

// TODO: Adds timestamp utility
// TODO: We can also add triggers to the Table

// Update initializes and returns a new UpdateBuilder instance for constructing
// an SQL UPDATE statement against the associated table.
func (t *Table) Update() *UpdateBuilder {
	builder := &UpdateBuilder{
		columns:      make(map[string]any),
		parameters:   []string{},
		table:        t,
		preparator_t: preparator_t{table: t},
		range_clause_t: range_clause_t{
			order: make(map[string]OrderByType),
		},
	}

	builder.parent = builder

	return builder
}

// Insert initializes and returns a new InsertBuilder instance for constructing
// an SQL INSERT statement against the associated table.
func (t *Table) Insert() *InsertBuilder {
	return &InsertBuilder{
		columns:      map[string]any{},
		table:        t,
		preparator_t: preparator_t{table: t},
	}
}

// String returns a formatted ASCII table describing the table schema.
// It includes the column name, type, and key/not-null/default attributes.
func (t *Table) String() string {
	if len(t.Columns) == 0 {
		return fmt.Sprintf("Table '%s' has no columns defined.\n", t.Name)
	}

	headers := []string{"Column Name", "Type", "Primary Key", "Not Null", "Unique", "Default"}
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
			fmt.Sprintf("%v", col.Unique),
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

// ColumnNames returns the names of all column in the table as they
// actually appear inside that table
func (t *Table) ColumnNames() []string {
	return utils.Map(func(c *column_t) string { return c.Name }, t.Columns)
}
