package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"strikeye/internal/export"
	"strikeye/internal/storage"
)

type exportResult struct {
	Scope  string   `json:"scope"`
	Dir    string   `json:"dir"`
	Format string   `json:"format"`
	Files  []string `json:"files"`
}

func runExport(ctx context.Context, dataRoot string, demoID int64, steamID, outDir, formatRaw string, out io.Writer) error {
	return runExportBatch(ctx, dataRoot, demoID, steamID, outDir, formatRaw, export.BatchFilter{}, false, out)
}

// runExportBatch is runExport plus the multi-match mode selected by --all.
func runExportBatch(ctx context.Context, dataRoot string, demoID int64, steamID, outDir, formatRaw string, filter export.BatchFilter, all bool, out io.Writer) error {
	formats, err := export.ParseFormats(formatRaw)
	if err != nil {
		return err
	}
	if all && (demoID > 0 || strings.TrimSpace(steamID) != "") {
		return fmt.Errorf("--all cannot be combined with --demo-id or --steamid")
	}
	if !all && demoID <= 0 && strings.TrimSpace(steamID) == "" {
		return fmt.Errorf("specify --demo-id, --steamid, or --all")
	}
	if demoID > 0 && strings.TrimSpace(steamID) != "" {
		return fmt.Errorf("--demo-id and --steamid are mutually exclusive")
	}
	if outDir == "" {
		outDir = filepath.Join(dataRoot, "media", "exports")
	}

	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	var result exportResult
	if all {
		batch, err := export.Batch(ctx, db, outDir, formats, filter, formatRaw)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(batch)
	}
	if demoID > 0 {
		bundle, err := export.LoadMatchBundle(ctx, db, demoID)
		if err != nil {
			return err
		}
		files, err := export.WriteMatchBundle(bundle, outDir, formats)
		if err != nil {
			return err
		}
		result = exportResult{Scope: fmt.Sprintf("demo:%d", demoID), Dir: outDir, Format: formatRaw, Files: files}
	} else {
		agg, err := export.AggregatePlayer(ctx, db, steamID)
		if err != nil {
			return err
		}
		files, err := export.WritePlayerAggregate(agg, outDir, formats)
		if err != nil {
			return err
		}
		result = exportResult{Scope: "player:" + agg.SteamID64, Dir: outDir, Format: formatRaw, Files: files}
	}
	if result.Format == "" {
		result.Format = "both"
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
