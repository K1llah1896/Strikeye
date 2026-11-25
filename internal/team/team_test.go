package team

import (
	"context"
	"testing"

	"strikeye/internal/storage"
)

func seedDemo(t *testing.T, db *storage.DB, hash, teamA, teamB string, scoreA, scoreB, rounds int, mapName string) int64 {
	t.Helper()
	demo, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
		FilePath: "/demos/" + hash + ".dem", Filename: hash + ".dem",
		ContentSHA256: hash, TeamAName: teamA, TeamBName: teamB,
		TeamAScore: scoreA, TeamBScore: scoreB, TotalRounds: rounds,
		MapName: &mapName, ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	return demo.ID
}

func TestBuildAggregatesMapsAndRoster(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	id1 := seedDemo(t, db, "hash1", "FURIA", "Falcons", 13, 6, 19, "de_dust2")
	id2 := seedDemo(t, db, "hash2", "Falcons", "FURIA", 14, 16, 30, "de_ancient")

	if err := db.Analysis().ReplaceMatchPlayers(ctx, id1, []storage.MatchPlayer{
		{SteamID64: "1", PlayerName: "KSCERATO", TeamKey: "team_a", Kills: 20, Deaths: 10, Assists: 5, TotalDamage: 1900, ADR: 100, KASTPercent: 80, StrikeyeRating: 1.5},
		{SteamID64: "2", PlayerName: "NiKo", TeamKey: "team_b", Kills: 9, Deaths: 18, TotalDamage: 900, ADR: 47.4, KASTPercent: 60, StrikeyeRating: 0.8},
	}); err != nil {
		t.Fatalf("seed players 1: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, id2, []storage.MatchPlayer{
		{SteamID64: "1", PlayerName: "KSCERATO", TeamKey: "team_b", Kills: 25, Deaths: 15, Assists: 3, TotalDamage: 3200, ADR: 106.7, KASTPercent: 70, StrikeyeRating: 1.2},
		{SteamID64: "3", PlayerName: "m0NESY", TeamKey: "team_a", Kills: 40, Deaths: 12, TotalDamage: 4000, ADR: 133, KASTPercent: 90, StrikeyeRating: 2.0},
	}); err != nil {
		t.Fatalf("seed players 2: %v", err)
	}

	rep, err := Build(ctx, db, " furia ")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(rep.Maps) != 2 || rep.MapWins != 2 || rep.MapLosses != 0 || rep.RoundsWon != 29 || rep.RoundsLost != 20 {
		t.Fatalf("maps summary = %+v", rep)
	}
	if len(rep.Roster) != 1 || rep.Roster[0].SteamID64 != "1" {
		t.Fatalf("roster should only contain FURIA players in both maps: %+v", rep.Roster)
	}
	r := rep.Roster[0]
	if r.Maps != 2 || r.Rounds != 49 || r.Kills != 45 || r.Deaths != 25 {
		t.Fatalf("roster totals = %+v", r)
	}
	if r.ADR != 104.1 {
		t.Fatalf("weighted ADR = %v, want 104.1", r.ADR)
	}
	if rep.MatchDates != nil {
		t.Fatalf("expected no dates, got %v", rep.MatchDates)
	}
}

func TestBuildRequiresName(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := Build(context.Background(), db, "  "); err == nil {
		t.Fatalf("expected error for empty name")
	}
}
