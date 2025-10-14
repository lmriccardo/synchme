package main

import (
	"context"
	"fmt"

	"github.com/lmriccardo/synchme/internal/utils/dbutils"
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

	fmt.Println(sch.String())
}
