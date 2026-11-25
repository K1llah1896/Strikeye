package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"strikeye/internal/storage"
)

// runStrokes lists or clears freehand map annotations for a demo.
func runStrokes(ctx context.Context, dataRoot string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("strokes", flag.ContinueOnError)
	fs.SetOutput(out)
	demoID := fs.Int64("demo-id", 0, "Demo id")
	round := fs.Int("round", -1, "Only strokes of this round (0 = match-wide)")
	clear := fs.Bool("clear", false, "Delete matching strokes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *demoID <= 0 {
		return fmt.Errorf("--demo-id is required")
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	if *clear {
		if *round >= 0 {
			removed, err := db.MapStrokes().DeleteByRound(ctx, *demoID, *round)
			if err != nil {
				return err
			}
			return json.NewEncoder(out).Encode(map[string]any{"removed": removed, "round": *round})
		}
		rows, err := db.MapStrokes().List(ctx, *demoID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := db.MapStrokes().Delete(ctx, row.ID); err != nil {
				return err
			}
		}
		return json.NewEncoder(out).Encode(map[string]any{"removed": len(rows)})
	}

	rows, err := db.MapStrokes().List(ctx, *demoID)
	if err != nil {
		return err
	}
	filtered := make([]storage.MapStroke, 0, len(rows))
	for _, row := range rows {
		if *round >= 0 && row.RoundNumber != *round {
			continue
		}
		filtered = append(filtered, row)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"demo_id": *demoID, "strokes": filtered, "count": len(filtered)})
}
