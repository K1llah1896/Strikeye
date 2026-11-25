package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func TestRunBuysGroupsByRoundAndPlayer(t *testing.T) {
	dataRoot := t.TempDir()
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:      "/demos/b.dem",
		Filename:      "b.dem",
		ContentSHA256: strings.Repeat("f", 64),
		ParseStatus:   "parsed",
		TotalRounds:   16,
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	rows := []storage.MatchPurchase{
		{RoundNumber: 1, Tick: 100, TimeSec: -12, SteamID64: "1", PlayerName: "NiKo", Weapon: "AK-47", Cost: 2700},
		{RoundNumber: 1, Tick: 130, TimeSec: -11.5, SteamID64: "1", PlayerName: "NiKo", Weapon: "Flashbang", Cost: 200},
		{RoundNumber: 1, Tick: 140, TimeSec: -11.3, SteamID64: "2", PlayerName: "karrigan", Weapon: "Smoke Grenade", Cost: 300},
		{RoundNumber: 2, Tick: 900, TimeSec: -8, SteamID64: "2", PlayerName: "karrigan", Weapon: "AWP", Cost: 4750},
	}
	if err := db.Analysis().ReplaceMatchPurchases(ctx, demo.ID, rows); err != nil {
		t.Fatalf("seed purchases: %v", err)
	}
	db.Close()

	var out bytes.Buffer
	if err := runBuys(ctx, dataRoot, demo.ID, 0, &out); err != nil {
		t.Fatalf("buys: %v", err)
	}
	var payload struct {
		Purchases  int     `json:"purchases"`
		TotalSpend int     `json:"total_spend"`
		Avg        float64 `json:"avg_per_round"`
		Rounds     []struct {
			Round      int `json:"round"`
			TotalSpend int `json:"total_spend"`
			Players    []struct {
				PlayerName string `json:"player_name"`
				Spend      int    `json:"spend"`
				Items      []struct {
					Weapon string `json:"weapon"`
				} `json:"items"`
			} `json:"players"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v (%s)", err, out.String())
	}
	if payload.Purchases != 4 || payload.TotalSpend != 7950 {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.Rounds) != 2 || payload.Rounds[0].TotalSpend != 3200 || payload.Rounds[1].TotalSpend != 4750 {
		t.Fatalf("rounds = %+v", payload.Rounds)
	}
	if payload.Rounds[0].Players[0].PlayerName != "NiKo" || payload.Rounds[0].Players[0].Spend != 2900 {
		t.Fatalf("top spender = %+v", payload.Rounds[0].Players[0])
	}
	if payload.Avg != 3975 {
		t.Fatalf("avg = %v", payload.Avg)
	}

	out.Reset()
	if err := runBuys(ctx, dataRoot, demo.ID, 2, &out); err != nil {
		t.Fatalf("buys round 2: %v", err)
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil || len(payload.Rounds) != 1 || payload.Rounds[0].Round != 2 {
		t.Fatalf("filtered = %s err=%v", out.String(), err)
	}
}
