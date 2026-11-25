package storage

import (
	"context"
	"testing"
)

func TestReplaceAndListMatchPurchases(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	rows := []MatchPurchase{
		{RoundNumber: 2, Tick: 6400, TimeSec: -3.5, SteamID64: "1", PlayerName: "karrigan", Weapon: "Smoke Grenade", Cost: 300},
		{RoundNumber: 1, Tick: 6100, TimeSec: -8.2, SteamID64: "2", PlayerName: "NiKo", Weapon: "AK-47", Cost: 2700},
		{RoundNumber: 1, Tick: 6050, TimeSec: -9.0, SteamID64: "2", PlayerName: "NiKo", Weapon: "Kevlar + Helmet", Cost: 1000},
	}
	if err := db.Analysis().ReplaceMatchPurchases(ctx, 5, rows); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := db.Analysis().ListMatchPurchases(ctx, 5)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("rows = %+v", got)
	}
	if got[0].RoundNumber != 1 || got[0].Weapon != "Kevlar + Helmet" {
		t.Fatalf("ordering = %+v", got)
	}
	if got[2].RoundNumber != 2 || got[2].PlayerName != "karrigan" {
		t.Fatalf("last = %+v", got[2])
	}

	totals, err := db.Analysis().PurchaseTotals(ctx, 5)
	if err != nil || totals["2"] != 3700 || totals["1"] != 300 {
		t.Fatalf("totals = %+v err=%v", totals, err)
	}

	// Replacing drops the previous rows for the same demo only.
	if err := db.Analysis().ReplaceMatchPurchases(ctx, 6, []MatchPurchase{{RoundNumber: 1, Tick: 1, SteamID64: "9", PlayerName: "x", Weapon: "P250", Cost: 300}}); err != nil {
		t.Fatalf("replace other demo: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPurchases(ctx, 5, []MatchPurchase{{RoundNumber: 3, Tick: 2, SteamID64: "3", PlayerName: "y", Weapon: "AWP", Cost: 4750}}); err != nil {
		t.Fatalf("replace again: %v", err)
	}
	if got, _ := db.Analysis().ListMatchPurchases(ctx, 5); len(got) != 1 || got[0].Weapon != "AWP" {
		t.Fatalf("after replace = %+v", got)
	}
	if got, _ := db.Analysis().ListMatchPurchases(ctx, 6); len(got) != 1 {
		t.Fatalf("other demo affected: %+v", got)
	}
}
