package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type TagRow struct {
	Addr   string `json:"addr"`
	Name   string `json:"name,omitempty"`
	Type   string `json:"type"`
	Offset string `json:"offset"`
	Size   int    `json:"size"`
	Init   any    `json:"init,omitempty"`
	Desc   string `json:"desc,omitempty"`
}

func WriteTable(w io.Writer, rows []TagRow, columns []string, color bool) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	head := make([]string, 0, len(columns))
	for _, c := range columns {
		u := strings.ToUpper(strings.TrimSpace(c))
		if color {
			u = "\x1b[36m" + u + "\x1b[0m"
		}
		head = append(head, u)
	}
	if _, err := fmt.Fprintln(tw, strings.Join(head, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		values := make([]string, 0, len(columns))
		for _, c := range columns {
			values = append(values, rowValue(row, c))
		}
		if _, err := fmt.Fprintln(tw, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func WriteJSON(w io.Writer, rows []TagRow) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func WriteCSV(w io.Writer, rows []TagRow, columns []string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, 0, len(columns))
		for _, col := range columns {
			record = append(record, rowValue(row, col))
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func rowValue(row TagRow, col string) string {
	switch strings.ToLower(strings.TrimSpace(col)) {
	case "addr":
		return row.Addr
	case "name":
		return row.Name
	case "type":
		return row.Type
	case "offset":
		return row.Offset
	case "size":
		return fmt.Sprintf("%d", row.Size)
	case "init":
		return fmt.Sprintf("%v", row.Init)
	case "desc":
		return row.Desc
	default:
		return ""
	}
}
