package series

import (
	"testing"

	"strikeye/internal/storage"
)

func ptr(s string) *string { return &s }

func demo(id int64, mapName, teamA, teamB, date string, aScore, bScore int) storage.Demo {
	return storage.Demo{
		ID:          id,
		MapName:     ptr(mapName),
		MatchDate:   ptr(date),
		TeamAName:   teamA,
		TeamBName:   teamB,
		TeamAScore:  aScore,
		TeamBScore:  bScore,
		ParseStatus: "parsed",
		AddedAt:     "2026-11-20T10:00:00Z",
	}
}

func TestGroupOrientsSwappedTeams(t *testing.T) {
	demos := []storage.Demo{
		demo(1, "de_ancient", "FURIA", "Falcons", "2026-11-20", 13, 11),
		demo(2, "de_dust2", "Falcons", "FURIA", "2026-11-20", 10, 13),
	}
	got := Group(demos)
	if len(got) != 1 {
		t.Fatalf("series = %+v, want 1", got)
	}
	s := got[0]
	if s.TeamA != "Falcons" || s.TeamB != "FURIA" {
		t.Errorf("orientation = %s/%s", s.TeamA, s.TeamB)
	}
	if s.MapWinsA != 0 || s.MapWinsB != 2 {
		t.Errorf("map wins = %d-%d, want 0-2", s.MapWinsA, s.MapWinsB)
	}
	if !s.Maps[0].Swapped || s.Maps[0].ScoreA != 11 || s.Maps[0].ScoreB != 13 {
		t.Errorf("map1 = %+v", s.Maps[0])
	}
	if s.Label() != "Falcons 0-2 FURIA" {
		t.Errorf("label = %q", s.Label())
	}
}

func TestGroupSkipsSinglesAndPlaceholders(t *testing.T) {
	demos := []storage.Demo{
		demo(1, "de_mirage", "NAVI", "G2", "2026-11-21", 13, 7),
		demo(2, "de_inferno", "Team A", "Team B", "2026-11-21", 13, 7),
		demo(3, "de_nuke", "Team A", "Team B", "2026-11-21", 13, 7),
		demo(4, "de_vertigo", "NAVI", "G2", "2026-11-22", 13, 7),
	}
	if got := Group(demos); len(got) != 0 {
		t.Fatalf("expected no series, got %+v", got)
	}
}

func TestGroupSeparatesDaysAndSortsMaps(t *testing.T) {
	demos := []storage.Demo{
		demo(2, "de_dust2", "A", "B", "2026-11-20", 13, 5),
		demo(1, "de_ancient", "A", "B", "2026-11-20", 9, 13),
		demo(3, "de_mirage", "A", "B", "2026-11-21", 13, 11),
		demo(4, "de_inferno", "A", "B", "2026-11-21", 13, 9),
	}
	got := Group(demos)
	if len(got) != 2 {
		t.Fatalf("series count = %d, want 2", len(got))
	}
	if got[0].Date != "2026-11-21" {
		t.Errorf("newest first, got %s", got[0].Date)
	}
	if got[1].Maps[0].DemoID != 1 || got[1].Maps[1].DemoID != 2 {
		t.Errorf("map order = %+v", got[1].Maps)
	}
	if got[1].MapWinsA != 1 || got[1].MapWinsB != 1 {
		t.Errorf("draw map wins = %d-%d", got[1].MapWinsA, got[1].MapWinsB)
	}
}

func TestGroupIgnoresUnparsed(t *testing.T) {
	d := demo(1, "de_ancient", "A", "B", "2026-11-20", 13, 5)
	d.ParseStatus = "pending"
	d2 := demo(2, "de_dust2", "A", "B", "2026-11-20", 13, 5)
	if got := Group([]storage.Demo{d, d2}); len(got) != 0 {
		t.Fatalf("unparsed demo participated: %+v", got)
	}
}

func TestByDemoID(t *testing.T) {
	demos := []storage.Demo{
		demo(1, "de_ancient", "A", "B", "2026-11-20", 13, 5),
		demo(2, "de_dust2", "A", "B", "2026-11-20", 10, 13),
		demo(3, "de_mirage", "C", "D", "2026-11-20", 13, 1),
	}
	groups := Group(demos)
	byID := ByDemoID(groups)
	if len(byID) != 2 {
		t.Fatalf("byID size = %d, want 2", len(byID))
	}
	if byID[1].Key != byID[2].Key || byID[1].Key == "" {
		t.Errorf("demoids 1/2 not in same series: %+v", byID)
	}
	if _, ok := byID[3]; ok {
		t.Error("single-map demo should not map to a series")
	}
}
