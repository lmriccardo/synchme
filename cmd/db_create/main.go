package main

import (
	"context"
	"fmt"

	"github.com/lmriccardo/synchme/internal/utils/dbutils"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sch, builder, err := dbutils.NewSchema("index.sqlite", ctx)
	if err != nil {
		fmt.Println("Error when creating the schema: ", err)
		return
	}

	builder.AddTable("other_table").AutoID()

	// Let's add a table
	builder.AddTable("files").AutoID().
		Text("name").
		IntegerWithDefault("age", 0).
		Blob("data").
		Column("ref_id", dbutils.INTEGER, false, true, nil).
		ForeignKey("ref_id", "other_table", "id",
			dbutils.NO_ACTION, dbutils.NO_ACTION, false, false)

	if err := builder.Build(); err != nil {
		fmt.Println("Error when building the schema: ", err)
		return
	}

	tbl, err := sch.Use("files")
	if err != nil {
		fmt.Println("Error when selecting the table: ", err)
		return
	}

	update := tbl.Update().
		Set("name").
		SetValue("age", 10).
		Where().
		Cond("age > :age1").
		Or().
		Cond("age < :age1").
		Not("id != 11").
		EndGroup().
		EndWhere().Limit(10)

	str, err := update.Build()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(str)

	stmt, _ := update.Prepare()
	if stmt == nil {
		return
	}

	fmt.Println(stmt)
}
