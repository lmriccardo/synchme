package dbutils

import (
	"container/list"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
)

// Validate performs a series of semantic and structural checks on a column definition
// to ensure that it is valid and consistent with SQLite’s rules and constraints.
//
// It validates the following aspects:
//   - The DEFAULT value’s Go type correctly maps to the declared SQLite data type.
//   - Unsupported DEFAULT types or invalid DEFAULTs on BLOB columns are flagged.
//   - AUTOINCREMENT usage conforms to SQLite rules (only on INTEGER PRIMARY KEY, no DEFAULT, no UNIQUE).
//   - Redundant or conflicting constraints are detected (e.g., UNIQUE + PRIMARY KEY, redundant NOT NULL).
//   - Basic column sanity checks, such as non-empty names and valid data types.
//
// The method returns a slice of errors describing all validation issues found.
// If the returned slice is empty, the column definition is considered valid.
func (c *column_t) Validate() (errs []error) {
	// The column name cannot be empty
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, fmt.Errorf("column name cannot be empty"))
	}

	// Check that the default type maps exactly the input data type
	if c.Default != nil {
		t := reflect.TypeOf(c.Default)
		dt, ok := TYPE_MAP[t.Kind()]

		// Check if the type of the data appears in the type map
		if !ok {
			err := fmt.Errorf("unsupported default type for %q: %v", c.Name, t)
			errs = append(errs, err)
		}

		// Check if the mapped type matches the expected one
		if dt != c.Type {
			err := fmt.Errorf("type mismatch for %q: expected %s but got %s",
				c.Name, c.Type.String(), dt.String())

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

// validateConditionColumns checks if a raw SQL condition string contains valid
// column names. It uses a provided regular expression pattern to identify column
// references within the condition.
func validateConditionColumns(condition string, t *Table,
	pattern *regexp.Regexp) (errs []error) {
	// Apply the input pattern and check for any columns in the condition
	if matches := pattern.FindStringSubmatch(condition); len(matches) > 1 {
		// Extract and normalize column name
		parts := strings.Split(matches[1], ".")
		col_name := strings.ToLower(parts[len(parts)-1])

		// If we have found columns in the format table.col_name
		// than we can also check whether that table acutally
		// exists in the schema
		if len(parts) > 1 {
			table_name := strings.ToLower(parts[0])
			if _, ok := t.sch.Tables[table_name]; !ok {
				errs = append(errs, fmt.Errorf(
					"in condition %q ref. column %q, the table %q does not exists",
					condition, matches[1], table_name,
				))
				return
			}
		}

		// Check if the column exists in the table
		if _, ok := t.ColumnsIndex[col_name]; !ok {
			errs = append(errs, fmt.Errorf(
				"column %q in condition %q not a table column",
				col_name, condition,
			))
		}
	} else {
		errs = append(errs, fmt.Errorf(
			"cannot parse column from condition: %q",
			condition))
	}

	return
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
		for _, condition := range group.Conds {
			errs = append(errs, validateConditionColumns(condition, t, pattern)...)
		}

		for _, subgroup := range group.SubGroups {
			queue.PushBack(subgroup)
		}
	}

	return
}

// Validate validates the UPDATE SQL Query
func (u *UpdateBuilder) Validate() error {
	// Initialize the list of all errors detected during validation
	errors := []error{}

	// First we would like to check if columns in each suboperation
	// exists in the actual table.

	// First check columns used for the SET clause
	for _, col := range utils.MapKeys(u.Columns) {
		if _, ok := u.table.ColumnsIndex[col]; !ok {
			errors = append(errors, fmt.Errorf(
				"column used for SET %q not a table column", col))
		}
	}

	// Check WHERE clause
	errors = append(errors, u.ValidateColumns(u.table)...)

	if len(errors) == 0 {
		return nil
	}

	// Format the error string and return all errors
	h := func(e error) string { return e.Error() }
	return fmt.Errorf("%s", strings.Join(utils.Map(h, errors), "\n"))
}

func (s *SelectBuilder) Validate() error {
	return nil
}

// ValidateTypes takes as input a mapping between parameters and assigned
// values and check if the assigned value types correspond to the releated
// column types
func ValidateTypes(columns map[string]string, params map[string]any,
	t *Table) (bool, []error) {

	errors := []error{}
	for name, value := range params {
		column, ok := columns[name]

		// Check if the parameter is in the mapping
		if !ok {
			errors = append(errors, fmt.Errorf(
				"parameter %q does not match any column in table %q",
				name, t.Name,
			))
			continue
		}

		// If the parameter exists in the map, check using reflection
		// the value type is exactly the one specified in the table
		value_t := reflect.TypeOf(value).Kind()
		sql_type, ok := TYPE_MAP[value_t]
		if !ok {
			errors = append(errors, fmt.Errorf(
				"value %v associated with parameter %q does not match "+
					"any SQL valid type association: %s",
				value, name, value_t.String(),
			))
			continue
		}

		target := t.Columns[t.ColumnsIndex[column]].Type
		if target != sql_type {
			errors = append(errors, fmt.Errorf(
				"value %v associated with parameter %q does not match "+
					"the expected type for column %q: %s != %s",
				value, name, column, sql_type.String(), target.String(),
			))
		}
	}

	return len(errors) == 0, errors
}
