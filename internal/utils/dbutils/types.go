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

// Maps SQL drivers to placeholder style
var PLACEHOLDER = map[string]string{
	// Uses ? for each placeholder
	"sqlite3":   "?",
	"sqlite":    "?",
	"mysql":     "?",
	"snowflake": "?",
	"vertica":   "?",
	"ibm_db":    "?",

	// Uses $1, $2, ...
	"postgres": "$%d",
	"pq":       "$%d",

	// Uses @p1, @p2, ...
	"sqlserver": "@p%d",

	// Uses :1, :2, ...
	"oracle": ":%d",
}

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

func (w WhereOpType) String() string {
	switch w {
	case AND:
		return "AND"
	case OR:
		return "OR"
	case NOT:
		return "NOT"
	default:
		return fmt.Sprintf("UNKNOWN WHERE OP TYPE(%d)", int(w))
	}
}

func (o OrderByType) String() string {
	switch o {
	case ASC:
		return "ASC"
	case DESC:
		return "DESC"
	default:
		return fmt.Sprintf("UNKNOWN ORDER_BY TYPE(%d)", int(o))
	}
}
