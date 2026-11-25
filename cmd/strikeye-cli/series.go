package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"strikeye/internal/series"
	"strikeye/internal/storage"
)

func runSeries(ctx context.Context, dataRoot string, minMaps int, out io.Writer) error {
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return fmt.Errorf("list demos: %w", err)
	}
	groups := series.Group(demos)
	if minMaps > 0 {
		filtered := groups[:0]
		for _, g := range groups {
			if len(g.Maps) >= minMaps {
				filtered = append(filtered, g)
			}
		}
		groups = filtered
	}
	if groups == nil {
		groups = []series.Series{}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"series": groups, "count": len(groups)})
}
