package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"strikeye/internal/playback"
	"strikeye/internal/storage"
)

func runPlay(ctx context.Context, dataRoot string, demoID, startTick int64, out io.Writer) error {
	if demoID <= 0 {
		return fmt.Errorf("--demo-id is required")
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()
	demo, err := db.Demos().GetByID(ctx, demoID)
	if err != nil {
		return fmt.Errorf("load demo %d: %w", demoID, err)
	}
	doc := struct {
		DemoID    int64  `json:"demo_id"`
		DemoPath  string `json:"demo_path"`
		FileName  string `json:"file_name"`
		StartTick int64  `json:"start_tick"`
		LaunchURL string `json:"launch_url"`
		Config    string `json:"config,omitempty"`
	}{
		DemoID:    demo.ID,
		DemoPath:  demo.FilePath,
		FileName:  playback.DemoFileName(demo.FilePath),
		StartTick: startTick,
		LaunchURL: playback.URL(demo.FilePath),
		Config:    playback.Config(startTick),
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
