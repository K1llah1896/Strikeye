package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"strikeye/internal/sqlconsole"
	"strikeye/internal/storage"
)

func runSQL(ctx context.Context, dataRoot, query, format string, limit int, out io.Writer) error {
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	conn, err := db.ReadOnly(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	res, err := sqlconsole.Run(ctx, conn, query, limit)
	if err != nil {
		return err
	}

	switch format {
	case "csv":
		w := csv.NewWriter(out)
		if err := w.Write(res.Columns); err != nil {
			return err
		}
		for _, row := range res.Rows {
			record := make([]string, len(row))
			for i, v := range row {
				record[i] = cellString(v)
			}
			if err := w.Write(record); err != nil {
				return err
			}
		}
		w.Flush()
		return w.Error()
	default:
		doc := struct {
			Columns   []string `json:"columns"`
			Rows      [][]any  `json:"rows"`
			RowCount  int      `json:"row_count"`
			Truncated bool     `json:"truncated"`
			Limit     int      `json:"limit"`
		}{res.Columns, res.Rows, len(res.Rows), res.Truncated, limit}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}
}

func cellString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(t)
	}
}
