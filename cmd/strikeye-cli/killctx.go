package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"strikeye/internal/killctx"
	"strikeye/internal/mapgeom"
	"strikeye/internal/storage"
)

func runKillContext(ctx context.Context, dataRoot string, demoID int64, store bool, out io.Writer) error {
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

	rounds, err := db.Analysis().ListMatchRounds(ctx, demo.ID)
	if err != nil {
		return err
	}
	players, err := db.Analysis().ListMatchPlayers(ctx, demo.ID)
	if err != nil {
		return err
	}
	replays, err := loadReplayRounds(dataRoot, demo)
	if err != nil {
		return err
	}

	var bvh *mapgeom.BVH
	if mapName != "" {
		if loaded, ok := mapgeom.LoadForMap(dataRoot, mapName); ok {
			bvh = loaded
		}
	}

	rep := killctx.Build(replays, rounds, players, tickRate, bvh, killctx.DefaultConfig())
	rep.Map = mapName
	if store {
		rowCount, err := storeKillContext(ctx, db, demo.ID, rep)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "已存储 %d 条击杀上下文（demo %d）\n", rowCount, demo.ID)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// storeKillContext persists the computed annotations so SQL queries and exports
// can reach them without re-reading the replay cache.
func storeKillContext(ctx context.Context, db *storage.DB, demoID int64, rep killctx.Report) (int, error) {
	rows := make([]storage.KillContext, 0, len(rep.Kills))
	for _, k := range rep.Kills {
		rows = append(rows, storage.KillContext{
			DemoID:           demoID,
			RoundNumber:      k.Round,
			Tick:             k.Tick,
			TimeSec:          k.TimeSec,
			KillerSlot:       int(k.KillerSlot),
			VictimSlot:       int(k.VictimSlot),
			KillerName:       k.Killer,
			VictimName:       k.Victim,
			Weapon:           k.Weapon,
			Headshot:         k.Headshot,
			Distance:         k.Distance,
			Visible:          k.Visible,
			GeometryChecked:  k.Occluded,
			SmokeBlocked:     k.SmokeBlocked,
			OpticsChecked:    k.Optics,
			AimOffDeg:        k.AimOffDeg,
			InFOV:            k.InFOV,
			VictimSeesKiller: k.VictimSeesKiller,
			AttackerBlind:    k.AttackerBlind,
			VictimBlind:      k.VictimBlind,
			AttackerZoom:     int(k.AttackerZoom),
			VictimZoom:       int(k.VictimZoom),
			KillerX:          k.KillerPos[0],
			KillerY:          k.KillerPos[1],
			KillerZ:          k.KillerPos[2],
			VictimX:          k.VictimPos[0],
			VictimY:          k.VictimPos[1],
			VictimZ:          k.VictimPos[2],
		})
	}
	if err := db.KillContext().ReplaceMatchKillContext(ctx, demoID, rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}
