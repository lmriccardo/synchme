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

type ColumnOp int

const (
	NONE  ColumnOp = 0
	COUNT ColumnOp = 1 << iota
	SUM
	AVG
	MIN
	MAX
	UPPER
	LOWER
	DISTINCT
)

type JoinType int

const (
	INNER JoinType = iota
	LEFT
	RIGHT
	FULL
	CROSS
)

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

func (op ColumnOp) String() string {
	switch {
	case op&COUNT != 0:
		return "COUNT"
	case op&SUM != 0:
		return "SUM"
	case op&AVG != 0:
		return "AVG"
	case op&MIN != 0:
		return "MIN"
	case op&MAX != 0:
		return "MAX"
	case op&UPPER != 0:
		return "UPPER"
	case op&LOWER != 0:
		return "LOWER"
	default:
		return ""
	}
}

func (op ColumnOp) HasDistinct() bool {
	return op&DISTINCT != 0
}

func (j JoinType) String() string {
	switch j {
	case INNER:
		return "INNER JOIN"
	case LEFT:
		return "LEFT JOIN"
	case RIGHT:
		return "RIGHT JOIN"
	case FULL:
		return "FULL JOIN"
	case CROSS:
		return "CROSS JOIN"
	default:
		return "UNKNOWN JOIN"
	}
}
