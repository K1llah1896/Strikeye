package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"strikeye/internal/storage"
	"strikeye/internal/team"
)

func runTeam(ctx context.Context, dataRoot, name string, out io.Writer) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("--name is required")
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()
	report, err := team.Build(ctx, db, name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
