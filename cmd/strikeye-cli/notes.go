package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"strikeye/internal/storage"
)

// runNotes creates, deletes or lists tactical annotations for a demo.
func runNotes(ctx context.Context, dataRoot string, demoID int64, round int, calloutID, note, color string, deleteID int64, clear bool, out io.Writer) error {
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	demo, err := pickDemo(ctx, db, demoID)
	if err != nil {
		return err
	}
	repo := db.MapAnnotations()

	if clear {
		existing, err := repo.ListByDemo(ctx, demo.ID)
		if err != nil {
			return err
		}
		for _, a := range existing {
			if _, err := repo.Delete(ctx, a.ID); err != nil {
				return err
			}
		}
	}
	if deleteID > 0 {
		if ok, err := repo.Delete(ctx, deleteID); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("annotation %d not found", deleteID)
		}
	}
	if strings.TrimSpace(calloutID) != "" {
		if _, err := repo.Add(ctx, storage.MapAnnotation{
			DemoID: demo.ID, RoundNumber: round, CalloutID: calloutID, Note: note, Color: color,
		}); err != nil {
			return err
		}
	}

	rows, err := repo.ListByDemo(ctx, demo.ID)
	if err != nil {
		return err
	}
	payload := struct {
		DemoID      int64                   `json:"demo_id"`
		Count       int                     `json:"count"`
		Annotations []storage.MapAnnotation `json:"annotations"`
	}{DemoID: demo.ID, Count: len(rows), Annotations: rows}
	if payload.Annotations == nil {
		payload.Annotations = []storage.MapAnnotation{}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
