package h2h

import (
	"context"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func seed(t *testing.T, db *storage.DB, hash, teamA, teamB string, scoreA, scoreB int, mapName, date, status string) {
	t.Helper()
	var datePtr *string
	if date != "" {
		d := date
		datePtr = &d
	}
	if _, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
		FilePath: "/demos/" + hash + ".dem", Filename: hash + ".dem", ContentSHA256: hash,
		TeamAName: teamA, TeamBName: teamB, TeamAScore: scoreA, TeamBScore: scoreB,
		MapName: &mapName, MatchDate: datePtr, ParseStatus: status, TotalRounds: scoreA + scoreB,
	}); err != nil {
		t.Fatalf("seed %s: %v", hash, err)
	}
}

func TestBuildNormalisesOrientationAndGroupsMaps(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	seed(t, db, "h1", "FURIA", "Falcons", 16, 14, "de_ancient", "2026-11-01", "parsed")
	seed(t, db, "h2", "Falcons", "FURIA", 6, 13, "de_dust2", "2026-11-02", "parsed")
	seed(t, db, "h3", "FURIA", "Falcons", 9, 13, "de_dust2", "2026-11-03", "parsed")
	seed(t, db, "h4", "FURIA", "Falcons", 13, 2, "de_mirage", "2026-11-04", "pending")
	seed(t, db, "h5", "FURIA", "Vitality", 13, 5, "de_dust2", "2026-11-05", "parsed")

	rep, err := Build(ctx, db, "furia", "falcons")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rep.Matches != 3 || rep.WinsA != 2 || rep.WinsB != 1 {
		t.Fatalf("overall = %+v", rep)
	}
	if rep.RoundsA != 16+13+9 || rep.RoundsB != 14+6+13 {
		t.Fatalf("rounds = %d-%d", rep.RoundsA, rep.RoundsB)
	}
	if len(rep.Maps) != 2 || rep.Maps[0].Map != "de_dust2" || rep.Maps[0].Matches != 2 {
		t.Fatalf("maps = %+v", rep.Maps)
	}
	if rep.Maps[0].WinsA != 1 || rep.Maps[0].WinsB != 1 || rep.Maps[0].RoundsA != 22 || rep.Maps[0].RoundsB != 19 {
		t.Fatalf("dust2 record = %+v", rep.Maps[0])
	}
	if len(rep.Recent) != 3 || rep.Recent[0].Date != "2026-11-03" {
		t.Fatalf("recent = %+v", rep.Recent)
	}
	if !strings.EqualFold(rep.Recent[1].Winner, "furia") || rep.Recent[1].ScoreA != 13 || rep.Recent[1].ScoreB != 6 {
		t.Fatalf("recent[1] = %+v", rep.Recent[1])
	}
}

func TestBuildValidatesNames(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := Build(context.Background(), db, "", "Falcons"); err == nil {
		t.Fatal("expected error for missing team")
	}
	if _, err := Build(context.Background(), db, "FURIA", "furia"); err == nil {
		t.Fatal("expected error for identical teams")
	}
}
