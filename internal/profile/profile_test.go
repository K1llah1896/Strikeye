package profile

import (
	"context"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func seed(t *testing.T) (*storage.DB, string) {
	t.Helper()
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:      "/demos/a.dem",
		Filename:      "a.dem",
		ContentSHA256: strings.Repeat("b", 64),
		ParseStatus:   "parsed",
		TotalRounds:   20,
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, []storage.MatchPlayer{{
		SteamID64: "76561198000000001", PlayerName: "ParsedName", TeamKey: "team_a",
		Kills: 20, Deaths: 10, TotalDamage: 2000, ADR: 100, KASTPercent: 80,
		StrikeyeRating: 1.4, Headshots: 10,
	}}); err != nil {
		t.Fatalf("seed player: %v", err)
	}
	return db, "76561198000000001"
}

func TestBuildWithoutAndWithProfile(t *testing.T) {
	db, id := seed(t)
	ctx := context.Background()

	report, err := Build(ctx, db, id)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if report.HasProfile || report.Aggregate.PlayerName != "ParsedName" || report.Profile.SteamID64 != id {
		t.Errorf("report = %+v", report)
	}

	if _, err := Save(ctx, db, id, "donk", "entry star", []string{"star", "Star", "awp"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	report, err = Build(ctx, db, id)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !report.HasProfile || report.Aggregate.PlayerName != "donk" {
		t.Errorf("override not applied: %+v", report)
	}
	if len(report.Profile.Tags) != 2 || report.Profile.Tags[0] != "star" {
		t.Errorf("tags = %v", report.Profile.Tags)
	}

	// A save with only a note keeps the existing display name and tags.
	if _, err := Save(ctx, db, id, "", "new note", nil); err != nil {
		t.Fatalf("save note: %v", err)
	}
	got, _, _ := db.PlayerProfiles().Get(ctx, id)
	if got.DisplayName != "donk" || len(got.Tags) != 2 || got.Note != "new note" {
		t.Errorf("merge semantics = %+v", got)
	}

	if _, err := Build(ctx, db, "missing"); err == nil {
		t.Error("expected error for player without matches")
	}
	if _, err := Save(ctx, db, "", "x", "", nil); err == nil {
		t.Error("expected error for empty steamid")
	}
}

func seedTrendDemo(t *testing.T, db *storage.DB, hash, teamA, teamB string, scoreA, scoreB int, mapName, date string) int64 {
	t.Helper()
	demo, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
		FilePath: "/demos/" + hash + ".dem", Filename: hash + ".dem", ContentSHA256: hash,
		TeamAName: teamA, TeamBName: teamB, TeamAScore: scoreA, TeamBScore: scoreB,
		MapName: &mapName, MatchDate: &date, ParseStatus: "parsed", TotalRounds: scoreA + scoreB,
	})
	if err != nil {
		t.Fatalf("seed %s: %v", hash, err)
	}
	return demo.ID
}

func TestTrendReturnsChronologicalMatchesWithOpponent(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	// Later date seeded first so the sort is actually exercised.
	later := seedTrendDemo(t, db, "t2", "Falcons", "FURIA", 14, 16, "de_dust2", "2026-11-03")
	earlier := seedTrendDemo(t, db, "t1", "FURIA", "Falcons", 13, 9, "de_ancient", "2026-11-01")
	if err := db.Analysis().ReplaceMatchPlayers(ctx, earlier, []storage.MatchPlayer{
		{SteamID64: "42", PlayerName: "KSCERATO", TeamKey: "team_a", Kills: 20, Deaths: 12, Assists: 4, ADR: 95.5, KASTPercent: 78, HSPercent: 50, StrikeyeRating: 1.3},
	}); err != nil {
		t.Fatalf("players1: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, later, []storage.MatchPlayer{
		{SteamID64: "42", PlayerName: "KSCERATO", TeamKey: "team_b", Kills: 25, Deaths: 18, Assists: 6, ADR: 104.2, KASTPercent: 70, HSPercent: 44, StrikeyeRating: 1.1},
	}); err != nil {
		t.Fatalf("players2: %v", err)
	}

	points, err := Trend(ctx, db, " 42 ")
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("points = %+v", points)
	}
	if points[0].Date != "2026-11-01" || points[0].Opponent != "Falcons" || !points[0].Won {
		t.Fatalf("first point = %+v", points[0])
	}
	if points[1].Date != "2026-11-03" || points[1].Opponent != "Falcons" || !points[1].Won {
		t.Fatalf("second point = %+v", points[1])
	}
	if points[1].Rounds != 30 || points[1].Kills != 25 || points[1].Rating != 1.1 {
		t.Fatalf("second point stats = %+v", points[1])
	}

	if _, err := Trend(ctx, db, " "); err == nil {
		t.Fatal("expected an error for an empty steamid")
	}
	if pts, err := Trend(ctx, db, "999"); err != nil || len(pts) != 0 {
		t.Fatalf("unknown player = %v, %v", pts, err)
	}
}
