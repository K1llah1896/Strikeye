package analysis

import (
	"context"
	"testing"

	"strikeye/internal/storage"
)

// seedTeamDemo registers one parsed match between two teams with a full roster,
// so the team tab has something to aggregate.
func seedTeamDemo(t *testing.T, db *storage.DB, ctx context.Context) storage.Demo {
	t.Helper()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/team.dem", Filename: "team.dem", ContentSHA256: "team-demo",
		MapName: stringPtr("de_dust2"), TickRate: 64, TotalRounds: 1,
		TeamAName: "FURIA", TeamBName: "Falcons", TeamAScore: 1, TeamBScore: 0,
		ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	players := []storage.MatchPlayer{
		{DemoID: demo.ID, SteamID64: "1", PlayerName: "KSCERATO", TeamKey: "team_a", Kills: 2, TotalDamage: 200},
		{DemoID: demo.ID, SteamID64: "2", PlayerName: "yuurih", TeamKey: "team_a", Kills: 1, TotalDamage: 100},
		{DemoID: demo.ID, SteamID64: "3", PlayerName: "NiKo", TeamKey: "team_b", Kills: 1, TotalDamage: 150},
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, players); err != nil {
		t.Fatalf("players: %v", err)
	}
	rounds := []storage.MatchRound{{
		DemoID: demo.ID, RoundNumber: 1, StartTick: 100, FreezeEndTick: 200, EndTick: 5000,
		WinnerSide: stringPtr("CT"), WinnerTeamKey: stringPtr("team_a"), WinReason: stringPtr("elimination"),
	}}
	if err := db.Analysis().ReplaceMatchRounds(ctx, demo.ID, rounds); err != nil {
		t.Fatalf("rounds: %v", err)
	}
	return demo
}

func stringPtr(s string) *string { return &s }

func TestTeamCandidatesDedupesAndSkipsPlaceholders(t *testing.T) {
	demo := &storage.Demo{TeamAName: "FURIA", TeamBName: "furia"}
	got := teamCandidates(demo)
	if len(got) != 1 || got[0] != "FURIA" {
		t.Fatalf("candidates = %v", got)
	}
	demo = &storage.Demo{TeamAName: "FURIA", TeamBName: ""}
	if got := teamCandidates(demo); len(got) != 1 {
		t.Fatalf("blank team should be skipped: %v", got)
	}
	if got := teamCandidates(nil); got != nil {
		t.Fatalf("nil demo = %v", got)
	}
}

func TestLoadTeamBundleAggregatesDossierAndH2H(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demo := seedTeamDemo(t, db, ctx)

	bundle, err := loadTeamBundle(ctx, db, t.TempDir(), "FURIA", demo.TeamBName)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if bundle.Team.Team != "FURIA" || len(bundle.Team.Maps) != 1 {
		t.Fatalf("dossier = %+v", bundle.Team)
	}
	if bundle.Team.MapWins != 1 || bundle.Team.RoundsWon != 1 {
		t.Fatalf("dossier totals = %+v", bundle.Team)
	}
	if len(bundle.Team.Roster) != 2 {
		t.Fatalf("roster = %+v", bundle.Team.Roster)
	}
	// No replay cache in the temp dir, so the scouting profile degrades to the
	// dossier-backed numbers instead of failing.
	if bundle.H2H.TeamA == "" || bundle.H2H.Matches != 1 {
		t.Fatalf("h2h = %+v", bundle.H2H)
	}
	if bundle.H2H.WinsA != 1 || bundle.H2H.RoundsA != 1 {
		t.Fatalf("h2h totals = %+v", bundle.H2H)
	}
}

// TestLoadTeamBundleUnknownTeamIsEmpty checks that an unknown team yields an
// empty dossier rather than an error, so the tab can show guidance text.
func TestLoadTeamBundleUnknownTeamIsEmpty(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	bundle, err := loadTeamBundle(context.Background(), db, t.TempDir(), "Nobody", "")
	if err != nil {
		t.Fatalf("unknown team should not fail: %v", err)
	}
	if bundle.Team.Team != "Nobody" || len(bundle.Team.Maps) != 0 {
		t.Fatalf("dossier = %+v", bundle.Team)
	}
}
