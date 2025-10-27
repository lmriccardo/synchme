package main

import (
	"context"
	"fmt"
	"log"

	_ "modernc.org/sqlite"

	"github.com/lmriccardo/synchme/internal/utils/dbutils"
)

func PrintOperation(b dbutils.Buildable) {
	_, err := b.Build()
	if err != nil {
		log.Fatal(err)
	}

	stmt := b.Prepare()
	if stmt == nil {
		return
	}

	fmt.Println(stmt)
}

type InsertInput struct {
	Name string `dbutils:"name"`
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
		SetValue("age", 10)

	update.Where(dbutils.AND).
		Cond("age > :min_age1").
		StartOr().
		Cond("age < :max_age1").
		Not("other_table.id != 11").
		EndGroup()

	update.OrderBy("name", dbutils.ASC).Limit(10)

	PrintOperation(update)

	insert := tbl.Insert().Columns("name").ColumnWithValue("age", 10)
	PrintOperation(insert)

	stmt := insert.Prepare()
	result1, err := stmt.Exec(InsertInput{Name: "file1.txt"})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(result1.RowsAffected())

	result, err := stmt.Query(InsertInput{Name: "file1.txt"})
	if err != nil {
		log.Fatal(err)
	}

	defer result.Close()

	for {
		row, ok, err := result.Next()
		if err != nil {
			log.Fatal(err)
		}

		if !ok {
			break
		}

		fmt.Println(row.Values())
	}

	select_bld := sch.Select().Distinct().
		Columns("name", "id", "other_table.id").
		ColumnWithAlias("age", "table_0_age").
		ColumnWithOp("age", "max_age", dbutils.MAX).
		From("files", "other_table").
		LeftJoin("departments", "d", "u.department_id = d.id").
		InnerJoin("orders", "o", "o.user_id = u.id")

	select_bld.Where(dbutils.AND).
		Cond("age > :min_age1").
		StartOr().
		Cond("age < :max_age1").
		Not("other_table.id != 11").
		EndGroup()

	select_bld.GroupBy("u.id", "u.name", "d.name").
		OrderBy("order_count", dbutils.DESC).
		Limit(20)

	sql, err := select_bld.Build()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Final SQL:")
	fmt.Println(sql)
}
