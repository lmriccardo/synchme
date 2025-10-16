package main

import (
	"context"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/lmriccardo/synchme/internal/utils/dbutils"
)

func PrintOperation(b dbutils.Buildable) {
	_, err := b.Build()
	if err != nil {
		fmt.Println(err)
		return
	}

	stmt, _ := b.Prepare()
	if stmt == nil {
		return
	}

	fmt.Println(stmt)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sch, builder, err := dbutils.NewSchema("sqlite", "index.sqlite", ctx)
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
		Cond("age > :min_age1").
		Or().
		Cond("age < :max_age1").
		Not("other_table.id != 11").
		EndGroup().
		EndWhere().
		OrderBy("name", dbutils.ASC).
		Limit(10)

	PrintOperation(update)

	insert := tbl.Insert().Columns("name").ColumnWithValue("age", 10)
	PrintOperation(insert)
}
