package dbutils

import (
	"database/sql"
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

// Foreign Key action type for both ON DELETE and ON UPDATEs
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

type WhereOpType int

const (
	AND WhereOpType = iota
	OR
	NOT
)

type OrderByType int

const (
	ASC OrderByType = iota
	DESC
)

type TypeValidatorFunc func(p map[string]any) (bool, []error)

type Statement struct {
	stmt          *sql.Stmt        // The actual sql prepared statement
	positionalMap map[string][]int // A mapping from arguments to position
	nofArgs       int              // Total number of arguments
}
