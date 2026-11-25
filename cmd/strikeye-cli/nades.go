package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"strikeye/internal/callouts"
	"strikeye/internal/nadebook"
	"strikeye/internal/storage"
)

type nadesOutput struct {
	DemoID      int64             `json:"demo_id"`
	Map         string            `json:"map"`
	TotalThrows int               `json:"total_throws"`
	NamedThrows int               `json:"named_throws"`
	Lineups     []nadebook.Lineup `json:"lineups"`
	Count       int               `json:"count"`
}

// runNades prints the utility line-ups of a demo, clustered by throw spot,
// landing area and side.
func runNades(ctx context.Context, dataRoot string, demoID int64, minThrows int, out io.Writer) error {
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

	var set *callouts.Set
	if table, err := callouts.LoadCSV(filepath.Join(dataRoot, "cache", "map_data", "callouts.csv")); err == nil {
		if s, err := table.For(mapName); err == nil {
			set = s
		}
	}

	rep := nadebook.Build(replays, rounds, players, demo.TeamAName, tickRate, set, nadebook.DefaultConfig())
	lineups := make([]nadebook.Lineup, 0, len(rep.Lineups))
	for _, l := range rep.Lineups {
		if l.Throws >= minThrows {
			lineups = append(lineups, l)
		}
	}
	payload := nadesOutput{
		DemoID:      demo.ID,
		Map:         mapName,
		TotalThrows: rep.TotalThrows,
		NamedThrows: rep.NamedThrows,
		Lineups:     lineups,
		Count:       len(lineups),
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
