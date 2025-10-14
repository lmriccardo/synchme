package main

import (
	"fmt"

	"github.com/lmriccardo/synchme/internal/utils/dbutils"
)

func main() {
	sch, builder, err := dbutils.NewSchema("index.sqlite")
	if err != nil {
		fmt.Println("Error when creating the schema: ", err)
		return
	}

	builder.AddTable("other_table").
		Column("id", dbutils.INTEGER, true, false, nil).
		PrimaryKey("id", true)

	// Let's add a table
	builder.AddTable("files").
		Column("id", dbutils.INTEGER, true, false, nil).
		Column("name", dbutils.TEXT, true, false, nil).
		Column("age", dbutils.INTEGER, false, false, 0).
		Column("ref_id", dbutils.INTEGER, false, true, nil).
		PrimaryKey("id", true).
		ForeignKey("ref_id", "other_table", "id", dbutils.NO_ACTION, dbutils.NO_ACTION, false, false)

	if err := builder.Build(); err != nil {
		fmt.Println("Error when building the schema: ", err)
		return
	}

	fmt.Println(sch.String())
}
