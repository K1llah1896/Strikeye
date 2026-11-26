package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"strikeye/internal/storage"
	"strikeye/internal/voice"
)

// voiceActivityBlock is one speech burst with a round-relative clock.
type voiceActivityBlock struct {
	XUID    uint64  `json:"xuid"`
	Start   float64 `json:"start_sec"`
	End     float64 `json:"end_sec"`
	Seconds float64 `json:"seconds"`
	Frames  int     `json:"frames"`
}

// runVoiceActivity reports who talked when, optionally clipped to one round.
func runVoiceActivity(ctx context.Context, dataRoot string, demoID int64, roundNum int, out io.Writer) error {
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	demo, err := pickDemo(ctx, db, demoID)
	if err != nil {
		return err
	}
	blocks, err := loadVoiceActivityFor(dataRoot, demo)
	if err != nil {
		return err
	}

	var startTick, endTick int64
	if roundNum > 0 {
		rounds, err := db.Analysis().ListMatchRounds(ctx, demo.ID)
		if err != nil {
			return err
		}
		found := false
		for _, r := range rounds {
			if int(r.RoundNumber) == roundNum {
				startTick = r.FreezeEndTick
				if startTick <= 0 {
					startTick = r.StartTick
				}
				endTick = r.EndTick
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("round %d not found", roundNum)
		}
		blocks = voice.FilterRange(blocks, startTick, endTick)
	}

	tickRate := demo.TickRate
	if tickRate <= 0 {
		tickRate = 64
	}
	payloadBlocks := make([]voiceActivityBlock, 0, len(blocks))
	total := 0.0
	speakers := map[uint64]bool{}
	for _, b := range blocks {
		secs := float64(b.EndTick-b.StartTick+1) / tickRate
		total += secs
		speakers[b.XUID] = true
		payloadBlocks = append(payloadBlocks, voiceActivityBlock{
			XUID:    b.XUID,
			Start:   roundSeconds(float64(b.StartTick-startTick) / tickRate),
			End:     roundSeconds(float64(b.EndTick-startTick) / tickRate),
			Seconds: roundSeconds(secs),
			Frames:  b.Frames,
		})
	}

	payload := struct {
		DemoID    int64                `json:"demo_id"`
		Round     int                  `json:"round"`
		TickRate  float64              `json:"tick_rate"`
		Blocks    []voiceActivityBlock `json:"blocks"`
		Speakers  int                  `json:"speakers"`
		TotalSecs float64              `json:"total_seconds"`
		HasVoice  bool                 `json:"has_voice"`
	}{DemoID: demo.ID, Round: roundNum, TickRate: tickRate, Blocks: payloadBlocks,
		Speakers: len(speakers), TotalSecs: roundSeconds(total), HasVoice: len(blocks) > 0}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// roundSeconds trims a clock value to 2 decimals for stable output.
func roundSeconds(v float64) float64 { return math.Round(v*100) / 100 }

// loadVoiceActivityFor reads a demo's capture and groups it into speech bursts.
// Missing captures are not an error: a voiceless demo simply has no activity.
func loadVoiceActivityFor(dataRoot string, demo *storage.Demo) ([]voice.Activity, error) {
	if demo == nil {
		return nil, nil
	}
	dir := filepath.Join(dataRoot, "media", "voice")
	candidates := []string{filepath.Join(dir, fmt.Sprintf("demo_%d.cs2voice", demo.ID))}
	if demo.Filename != "" {
		candidates = append(candidates, filepath.Join(dir, demo.Filename+".cs2voice"))
	}
	var path string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.Size() > 0 {
			path = c
			break
		}
	}
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	frames, err := voice.ReadContainer(f)
	if err != nil {
		return nil, err
	}
	return voice.BuildActivity(frames, voice.DefaultActivityJoinTicks), nil
}

// voiceActivityUsage is referenced by the CLI usage text.
const voiceActivityUsage = "voice-activity --demo-id <id> [--round <n>]"
