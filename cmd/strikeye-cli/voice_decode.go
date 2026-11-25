package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"strikeye/internal/storage"
	"strikeye/internal/voice"
)

// runVoiceDecode decodes captured voice. With no tool it uses the built-in Opus
// decoder on the .cs2voice container; a tool path still delegates to an external
// Steam decoder for demos that used Steam's wrapped codec.
func runVoiceDecode(ctx context.Context, dataRoot string, demoID int64, tool, outDir string, out io.Writer) error {
	return runVoiceDecodeFiltered(ctx, dataRoot, demoID, tool, outDir, 0, out)
}

// runVoiceDecodeFiltered is runVoiceDecode with an optional single-speaker
// filter: xuid != 0 exports only that speaker's track.
func runVoiceDecodeFiltered(ctx context.Context, dataRoot string, demoID int64, tool, outDir string, xuid uint64, out io.Writer) error {
	// The built-in decoder only needs the capture file, so it stays usable even
	// before the demo is registered in storage.
	if tool == "" {
		return runBuiltinVoiceDecode(ctx, dataRoot, demoID, outDir, xuid, out)
	}
	if xuid != 0 {
		return fmt.Errorf("--xuid requires the built-in decoder (drop --tool)")
	}

	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	demo, err := db.Demos().GetByID(ctx, demoID)
	if err != nil {
		return err
	}
	if demo.FilePath == "" {
		return fmt.Errorf("demo %d not found", demoID)
	}
	if outDir == "" {
		outDir = filepath.Join(dataRoot, "media", "voice", fmt.Sprintf("demo_%d", demo.ID))
	}
	tracks, err := voice.DecodeWithTool(ctx, tool, demo.FilePath, outDir)
	if err != nil {
		return err
	}
	payload := struct {
		DemoID  int64    `json:"demo_id"`
		Tool    string   `json:"tool"`
		OutDir  string   `json:"out_dir"`
		Tracks  int      `json:"tracks"`
		Files   []string `json:"files"`
		HasData bool     `json:"has_voice"`
	}{DemoID: demo.ID, Tool: tool, OutDir: outDir, Tracks: len(tracks), Files: tracks, HasData: len(tracks) > 0}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// runBuiltinVoiceDecode decodes the .cs2voice capture with the pure-Go Opus
// decoder, writing one track per speaker plus a mixed track.
func runBuiltinVoiceDecode(ctx context.Context, dataRoot string, demoID int64, outDir string, xuid uint64, out io.Writer) error {
	if outDir == "" {
		outDir = filepath.Join(dataRoot, "media", "voice", fmt.Sprintf("demo_%s", itoa64(demoID)))
	}
	container, err := findVoiceCapture(dataRoot, demoID)
	if err != nil {
		return err
	}
	f, err := os.Open(container)
	if err != nil {
		return fmt.Errorf("open voice capture (run `voice` first): %w", err)
	}
	defer f.Close()
	frames, err := voice.ReadContainer(f)
	if err != nil {
		return err
	}
	if xuid != 0 {
		filtered := frames[:0:0]
		for _, fr := range frames {
			if fr.XUID == xuid {
				filtered = append(filtered, fr)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("speaker %d did not talk in this capture", xuid)
		}
		frames = filtered
	}
	sum, tracks, err := voice.DecodeOpusDir(frames, outDir)
	if err != nil {
		return err
	}
	payload := struct {
		DemoID   int64                  `json:"demo_id"`
		Engine   string                 `json:"engine"`
		OutDir   string                 `json:"out_dir"`
		File     string                 `json:"file"`
		Frames   int                    `json:"frames"`
		Samples  int                    `json:"samples"`
		Seconds  float64                `json:"seconds"`
		Skipped  int                    `json:"skipped_frames"`
		Speakers []voice.DecodedSpeaker `json:"speakers"`
		Tracks   []voice.DecodedTrack   `json:"tracks"`
		Filter   uint64                 `json:"xuid,omitempty"`
		HasData  bool                   `json:"has_voice"`
	}{DemoID: demoID, Engine: "opus", OutDir: outDir, File: filepath.Join(outDir, "mixed.wav"),
		Filter: xuid,
		Frames: sum.Frames, Samples: sum.Samples, Seconds: float64(sum.Samples) / 48000,
		Skipped: sum.Skipped, Speakers: sum.Speakers, Tracks: tracks, HasData: sum.Frames > 0}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// findVoiceCapture locates a demo's .cs2voice capture. Canonical captures are
// named demo_<id>; older captures used the source file name, so storage is
// consulted for that fallback.
func findVoiceCapture(dataRoot string, demoID int64) (string, error) {
	dir := filepath.Join(dataRoot, "media", "voice")
	candidates := []string{filepath.Join(dir, "demo_"+itoa64(demoID)+".cs2voice")}
	if db, err := storage.OpenDefault(dataRoot); err == nil {
		if demo, err := db.Demos().GetByID(context.Background(), demoID); err == nil && demo.Filename != "" {
			candidates = append(candidates, filepath.Join(dir, demo.Filename+".cs2voice"))
		}
		db.Close()
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.Size() > 0 {
			return c, nil
		}
	}
	return "", fmt.Errorf("open voice capture (run `voice` first): no .cs2voice for demo %d", demoID)
}

// itoa64 formats an int64 without importing strconv at every call site.
func itoa64(v int64) string { return strconv.FormatInt(v, 10) }

// voiceDecodeUsage is referenced by the CLI usage text.
const voiceDecodeUsage = "voice-decode --demo-id <id> [--tool <path>] [--out <dir>]"
