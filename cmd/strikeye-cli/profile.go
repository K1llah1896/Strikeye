package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"strikeye/internal/profile"
	"strikeye/internal/storage"
)

func runProfile(ctx context.Context, dataRoot, steamID, name, note, tagsRaw string, out io.Writer) error {
	if strings.TrimSpace(steamID) == "" {
		return fmt.Errorf("--steamid is required")
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	if strings.TrimSpace(name) != "" || strings.TrimSpace(note) != "" || strings.TrimSpace(tagsRaw) != "" {
		var tags []string
		if strings.TrimSpace(tagsRaw) != "" {
			tags = storage.ParseTagList(tagsRaw)
			if tags == nil {
				tags = []string{}
			}
		}
		prof, err := profile.Save(ctx, db, steamID, name, note, tags)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"saved": true, "profile": prof})
	}

	report, err := profile.Build(ctx, db, steamID)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
