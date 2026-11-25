package storage

import (
	"context"
	"testing"
)

func TestKillContextRepoReplaceListAndTotals(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demo := seedSearchDemo(t, db, "kc", "A", "B", "de_dust2", "")
	demoID := demo.ID

	rows := []KillContext{
		{RoundNumber: 2, Tick: 7000, TimeSec: 20, KillerName: "a", VictimName: "b", Weapon: "awp",
			Distance: 1500, Visible: false, GeometryChecked: true, SmokeBlocked: true, OpticsChecked: true,
			AimOffDeg: 3, InFOV: true, AttackerZoom: 2, KillerX: 10, KillerY: 20, KillerZ: 64},
		{RoundNumber: 1, Tick: 6000, TimeSec: 5, KillerName: "c", VictimName: "d", Weapon: "ak47",
			Distance: 500, Visible: true, GeometryChecked: true, InFOV: true},
	}
	if err := db.KillContext().ReplaceMatchKillContext(ctx, demoID, rows); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := db.KillContext().ListMatchKillContext(ctx, demoID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].RoundNumber != 1 || got[1].RoundNumber != 2 {
		t.Fatalf("rows = %+v", got)
	}
	if !got[1].SmokeBlocked || got[1].Visible || got[1].AttackerZoom != 2 || got[1].KillerX != 10 {
		t.Fatalf("row = %+v", got[1])
	}
	totals, err := db.KillContext().Totals(ctx, demoID)
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if totals.Kills != 2 || totals.SmokePct != 50 || totals.BlindSpotPct != 50 {
		t.Fatalf("totals = %+v", totals)
	}
	if totals.AvgDistance != 1000 {
		t.Fatalf("avg distance = %v", totals.AvgDistance)
	}

	// Replacing with an empty slice clears the demo.
	if err := db.KillContext().ReplaceMatchKillContext(ctx, demoID, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got, err := db.KillContext().ListMatchKillContext(ctx, demoID); err != nil || len(got) != 0 {
		t.Fatalf("after clear = %+v err=%v", got, err)
	}
}
