package main

import (
	"io"

	"github.com/olekukonko/tablewriter"
)

func getTable(headers []string, out io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(out)
	table.SetHeader(headers)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	table.SetAutoFormatHeaders(false)

	opts := []tablewriter.Colors{}
	for range headers {
		opts = append(opts, tablewriter.Colors{tablewriter.FgHiYellowColor})
	}
	table.SetHeaderColor(opts...)
	return table
}
