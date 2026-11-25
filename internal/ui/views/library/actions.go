package library

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"strikeye/internal/export"
	"strikeye/internal/launcher"
	"strikeye/internal/playback"
	"strikeye/internal/storage"
)

// libraryExportFormats is what the card button writes: every format the export
// package supports, so the user can pick a consumer afterwards.
var libraryExportFormats = export.Formats{CSV: true, JSON: true, HTML: true, XLSX: true}

// exportDemo writes the full match bundle for one demo under
// <DataRoot>/media/exports and reports the directory and file count.
func exportDemo(ctx context.Context, db *storage.DB, dataRoot string, demoID int64) (string, int, error) {
	if db == nil {
		return "", 0, fmt.Errorf("storage unavailable")
	}
	bundle, err := export.LoadMatchBundle(ctx, db, demoID)
	if err != nil {
		return "", 0, err
	}
	dir := filepath.Join(dataRoot, "media", "exports")
	files, err := export.WriteMatchBundle(bundle, dir, libraryExportFormats)
	if err != nil {
		return dir, len(files), err
	}
	return dir, len(files), nil
}

// demoLaunchURL builds the steam:// URL that opens this demo in CS2.
func demoLaunchURL(demo storage.Demo) string {
	return playback.URL(demo.FilePath)
}

// openDemoInCS2 hands the steam:// URL to the OS handler. The URL is validated
// first so a demo row without a path cannot spawn a handler with garbage.
func openDemoInCS2(demo storage.Demo, open func(string) error) error {
	if strings.TrimSpace(demo.FilePath) == "" {
		return fmt.Errorf("demo has no file path")
	}
	url := demoLaunchURL(demo)
	if open == nil {
		open = launcher.Open
	}
	return open(url)
}
