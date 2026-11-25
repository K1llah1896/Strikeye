package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"strikeye/internal/callouts"
	"strikeye/internal/mapcontrol"
	"strikeye/internal/navmesh"
	"strikeye/internal/storage"
)

func runMapControl(ctx context.Context, dataRoot string, demoID int64, stepSeconds, radius float64, effects bool, out io.Writer) error {
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

	navPath := filepath.Join(dataRoot, "cache", "map_nav", mapName+".nav")
	mesh, err := navmesh.Load(navPath)
	if err != nil {
		return fmt.Errorf("%w (run: strikeye-cli navs-fetch --map %s)", err, mapName)
	}

	var set *callouts.Set
	if table, err := callouts.LoadCSV(filepath.Join(dataRoot, "cache", "map_data", "callouts.csv")); err == nil {
		if s, err := table.For(mapName); err == nil {
			set = s
		}
	}

	rounds, err := db.Analysis().ListMatchRounds(ctx, demo.ID)
	if err != nil {
		return err
	}

	replays, err := loadReplayRounds(dataRoot, demo)
	if err != nil {
		return err
	}

	cfg := mapcontrol.DefaultConfig()
	if stepSeconds > 0 {
		cfg.StepSeconds = stepSeconds
	}
	if radius > 0 {
		cfg.ReachRadius = radius
	}
	cfg.ApplyEffects = effects
	res := mapcontrol.Build(replays, rounds, mesh, set, tickRate, cfg)
	res.Map = mapName

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}
