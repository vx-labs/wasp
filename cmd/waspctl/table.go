package main

import (
	"io"

	"github.com/fatih/color"
	"github.com/rodaine/table"
)

func getTable(out io.Writer, headers ...interface{}) table.Table {
	headerFmt := color.New(color.FgHiYellow).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New(headers...)
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	return tbl
}
