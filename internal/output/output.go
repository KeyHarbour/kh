package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

type Printer struct {
	Format string // "table" or "json"
	W      io.Writer
}

func (p Printer) JSON(v any) error {
	enc := json.NewEncoder(p.W)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (p Printer) Table(headers []string, rows [][]string) error {
	if p.Format == "json" {
		return p.JSON(struct {
			Headers []string   `json:"headers"`
			Rows    [][]string `json:"rows"`
		}{Headers: headers, Rows: rows})
	}
	w := tabwriter.NewWriter(p.W, 0, 0, 2, ' ', 0)
	for i, h := range headers {
		fmt.Fprint(w, h)
		if i < len(headers)-1 {
			fmt.Fprint(w, "\t")
		}
	}
	fmt.Fprint(w, "\n")
	for _, r := range rows {
		for i, c := range r {
			fmt.Fprint(w, c)
			if i < len(r)-1 {
				fmt.Fprint(w, "\t")
			}
		}
		fmt.Fprint(w, "\n")
	}
	return w.Flush()
}
