package dbutils

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/lmriccardo/synchme/internal/utils"
)

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

// checkValueType checks if the value associated with the input column name
// in the input table, match the expected association with the SQL type
func checkValueType(value any, column *column_t) error {
	// Using reflections take the type of the value
	value_t := reflect.TypeOf(value).Kind()
	sql_type, ok := TYPE_MAP[value_t]
	if !ok {
		return fmt.Errorf(
			"value %v does not match any SQL valid type association: %s",
			value, value_t.String(),
		)
	}

	if sql_type != column.Type {
		return fmt.Errorf(
			"value %v does not match the expected type for column %q: %s != %s",
			value, column.Name, sql_type.String(), column.Type.String(),
		)
	}

	return nil
}

// ValidateTypes takes as input a mapping between parameters and assigned
// values and check if the assigned value types correspond to the releated
// column types
func ValidateTypes(columns map[string]string, params map[string]any,
	t *Table) (bool, []error) {

	errs := []error{}
	for name, value := range params {
		column_name, ok := columns[name]

		// Check if the parameter is in the mapping
		if !ok {
			errs = append(errs, fmt.Errorf(
				"parameter %q does not match any column in table %q",
				name, t.Name,
			))
			continue
		}

		// Check the type of the value against the column type
		column := t.Columns[t.ColumnsIndex[column_name]]
		if err := checkValueType(value, column); err != nil {
			errs = append(errs, err)
		}
	}

	return len(errs) == 0, errs
}

// extractParamMatches finds all parameter occurrences in the given SQL string.
func extractParamMatches(sql string) [][]string {
	pattern := regexp.MustCompile(
		`(?:[,\(]?)\s*(:[a-zA-Z_][a-zA-Z0-9_]*)\s*(?:[,\)]?)|` + // Matches (:param1, :param2, ...)
			`\b([a-zA-Z_][a-zA-Z0-9_\.]*)\b` + // Matches left-hand variable (column)
			`(?:\s*(?:=|!=|<>|<|>|<=|>=)\s*|\s+(?:LIKE|IN|IS\s+(?:NOT\s+)?NULL)\s+)` + // Matches operator
			`(:[a-zA-Z_][a-zA-Z0-9_]*|\S+)`, // Matches right-hand operand
	)

	return pattern.FindAllStringSubmatch(sql, -1)
}

// normalizeArguments validates and converts the input arguments into a standardized
// map[string]any format, where keys are parameter names and values are their
// corresponding values. It converts only structure or pointer to structure.
// Any other input type (e.g., slice, int, string) is considered invalid.
func normalizeArguments(args any) (map[string]any, error) {
	// Input arguments can be either a struct with defined parameters or a map from
	// parameter name to values. Any other input type is not a valid arguments type
	args_v := reflect.ValueOf(args)

	// If the input is a pointer, dereference it and get the concrete type
	if args_v.Kind() == reflect.Pointer {
		if args_v.IsNil() {
			return nil, errors.New("input argument pointer is nil")
		}

		args_v = args_v.Elem()
	}

	// Get the actual concrete type after possible dereference
	args_t := args_v.Kind()

	if args_t != reflect.Map && args_t != reflect.Struct {
		return nil, fmt.Errorf("unallowed input argument type %q", args_t.String())
	}

	// Otherwise, if it is a struct we need to check for tags
	if args_t == reflect.Struct {
		var err error
		if args, err = utils.StructToMap(args, "dbutils"); err != nil {
			return nil, err
		}
	}

	return args.(map[string]any), nil
}
