package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strikeye/internal/storage"
	"strikeye/internal/voice"
)

// runVoice extracts voice packets into <DataRoot>/media/voice/<demo>.cs2voice and
// reports who spoke. The payload stays encoded: decoding needs Steam's codec.
func runVoice(ctx context.Context, dataRoot string, demoID int64, out io.Writer) error {
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	demo, err := pickDemo(ctx, db, demoID)
	if err != nil {
		return err
	}

	dir := filepath.Join(dataRoot, "media", "voice")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// The capture is named after the demo id so `voice-decode --demo-id` can
	// find it without knowing the source file name.
	target := filepath.Join(dir, fmt.Sprintf("demo_%d.cs2voice", demo.ID))
	tmp := target + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	sum, extractErr := voice.Extract(ctx, demo.FilePath, f)
	closeErr := f.Close()
	if extractErr != nil {
		_ = os.Remove(tmp)
		return extractErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}

	type output struct {
		voice.Summary
		DemoID int64  `json:"demo_id"`
		Path   string `json:"path"`
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(output{Summary: sum, DemoID: demo.ID, Path: target})
}
