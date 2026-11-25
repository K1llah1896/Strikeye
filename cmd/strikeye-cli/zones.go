package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"strikeye/internal/callouts"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
	"strikeye/internal/zones"
)

type zonesOutput struct {
	DemoID        int64         `json:"demo_id"`
	Map           string        `json:"map"`
	SampleSeconds float64       `json:"setup_sample_seconds"`
	Rounds        int           `json:"rounds"`
	Sampled       int           `json:"sampled_positions"`
	Unnamed       int           `json:"unnamed_positions"`
	NamedPct      float64       `json:"named_percent"`
	ExactHits     int           `json:"exact_hits"`
	SnappedHits   int           `json:"snapped_hits"`
	SnapMeanUnits float64       `json:"snap_mean_units"`
	Setup         []zones.Row   `json:"setup"`
	DeathCount    int           `json:"death_count"`
	Deaths        []zones.Death `json:"deaths"`
	CalloutSource string        `json:"callout_source"`
}

func runZones(ctx context.Context, dataRoot string, demoID int64, out io.Writer) error {
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	demo, err := pickDemo(ctx, db, demoID)
	if err != nil {
		return err
	}
	mapName := ""
	if demo.MapName != nil {
		mapName = *demo.MapName
	}
	tickRate := demo.TickRate
	if tickRate <= 0 {
		tickRate = 64
	}

	players, err := db.Analysis().ListMatchPlayers(ctx, demo.ID)
	if err != nil {
		return err
	}
	rounds, err := db.Analysis().ListMatchRounds(ctx, demo.ID)
	if err != nil {
		return err
	}

	replays, err := loadReplayRounds(dataRoot, demo)
	if err != nil {
		return err
	}

	calloutPath := filepath.Join(dataRoot, "cache", "map_data", "callouts.csv")
	table, err := callouts.LoadCSV(calloutPath)
	if err != nil {
		return fmt.Errorf("%w (run: strikeye-cli callouts-fetch)", err)
	}
	set, err := table.For(mapName)
	if err != nil {
		return fmt.Errorf("%w (covered maps: %v)", err, table.Maps())
	}

	slotNames := map[uint8]string{}
	for _, p := range players {
		if p.SpecSlot != nil && *p.SpecSlot >= 0 && *p.SpecSlot <= 255 {
			slotNames[uint8(*p.SpecSlot)] = p.PlayerName
		}
	}

	totals := zones.Setup(replays, rounds, tickRate, set)
	deaths := zones.Deaths(replays, rounds, tickRate, set, slotNames)
	named := totals.Sampled - totals.Unnamed
	pct := 0.0
	if totals.Sampled > 0 {
		pct = float64(named) / float64(totals.Sampled) * 100
	}
	payload := zonesOutput{
		DemoID:        demo.ID,
		Map:           mapName,
		SampleSeconds: zones.SetupSampleSeconds,
		Rounds:        totals.Rounds,
		Sampled:       totals.Sampled,
		Unnamed:       totals.Unnamed,
		NamedPct:      pct,
		ExactHits:     totals.Exact,
		SnappedHits:   totals.Snapped,
		SnapMeanUnits: meanSnap(totals),
		Setup:         totals.Rows(),
		DeathCount:    len(deaths),
		Deaths:        deaths,
		CalloutSource: "cs2-server-admin-callouts/nav_places.csv",
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// loadReplayRounds decodes every round of a demo's 32 Hz replay cache.
func loadReplayRounds(dataRoot string, demo *storage.Demo) (map[int]parser.ReplayRoundData, error) {
	return parser.LoadReplayCache(dataRoot, demo)
}

// resolveReplayCache prefers the stored path but falls back to the same file
// name under the current DataRoot, since demo rows can outlive a moved install.
func resolveReplayCache(dataRoot string, stored *string) string {
	return parser.ResolveReplayCachePath(dataRoot, stored)
}

func meanSnap(t zones.SetupTotals) float64 {
	if t.Snapped == 0 {
		return 0
	}
	return t.SnapDistSum / float64(t.Snapped)
}

// pickDemo resolves a demo by ID, or falls back to the first parsed demo.
func pickDemo(ctx context.Context, db *storage.DB, demoID int64) (*storage.Demo, error) {
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range demos {
		if demoID > 0 && demos[i].ID == demoID {
			return &demos[i], nil
		}
	}
	if demoID > 0 {
		return nil, fmt.Errorf("demo %d not found", demoID)
	}
	for i := range demos {
		if demos[i].ParseStatus == "parsed" {
			return &demos[i], nil
		}
	}
	return nil, fmt.Errorf("no parsed demo found")
}
