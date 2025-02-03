package util

import (
	"fmt"
	"io"
)

var (
	CellPadding = 2
	LeftPadding = 0
)

type Table struct {
	CellPadding int
	LeftPadding int

	headers []string
	rows    [][]string
}

func toString(obj any) string {
	if str, ok := obj.(string); ok {
		return str
	} else {
		return fmt.Sprintf("%v", obj)
	}
}

func NewTable(headers ...any) (table *Table) {
	table = &Table{
		CellPadding: CellPadding,
		LeftPadding: LeftPadding,
	}
	table.rows = [][]string{}
	if len(headers) == 0 {
		return
	}

	table.headers = []string{}
	for _, header := range headers {
		table.headers = append(table.headers, toString(header))
	}
	return
}

func (t *Table) WithLeftPadding(padding int) *Table {
	t.LeftPadding = padding
	return t
}

func (t *Table) WithCellPadding(padding int) *Table {
	t.CellPadding = padding
	return t
}

func (t *Table) AddRow(datas ...any) {
	row := []string{}
	for _, data := range datas {
		row = append(row, toString(data))
	}
	t.rows = append(t.rows, row)
}

func (t *Table) Print(w io.Writer) {
	maxWidths := []int{}

	for _, row := range t.rows {
		for j, data := range row {
			if len(maxWidths) <= j {
				for range j - len(maxWidths) {
					maxWidths = append(maxWidths, 0)
				}
				maxWidths = append(maxWidths, len(data))
			} else {
				maxWidths[j] = max(maxWidths[j], len(data))
			}
		}
	}

	if t.headers != nil {
		fmt.Fprintf(w, "%-*s", t.LeftPadding, "")
		for j, header := range t.headers {
			fmt.Fprintf(w, "%-*s%-*s", maxWidths[j], header, t.CellPadding, "")
		}
		fmt.Fprintf(w, "\n")
	}

	for _, row := range t.rows {
		fmt.Fprintf(w, "%-*s", t.LeftPadding, "")
		for j, data := range row {
			fmt.Fprintf(w, "%-*s", maxWidths[j], data)
			if j != len(data)-1 {
				fmt.Fprintf(w, "%-*s", t.CellPadding, "")
			}
		}
		fmt.Fprintf(w, "\n")
	}
}
