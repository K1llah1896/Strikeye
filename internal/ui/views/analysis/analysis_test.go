package analysis

import (
	"testing"
	"time"

	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

func TestSortPlayersAndHeatmapPoints(t *testing.T) {
	players := []storage.MatchPlayer{
		{PlayerName: "p1", Kills: 15, ADR: 72.5, StrikeyeRating: 1.05},
		{PlayerName: "p2", Kills: 28, ADR: 108.4, StrikeyeRating: 1.48},
		{PlayerName: "p3", Kills: 19, ADR: 85.0, StrikeyeRating: 1.12},
	}

	byRating := SortPlayers(players, "rating", true)
	if byRating[0].PlayerName != "p2" || byRating[2].PlayerName != "p1" {
		t.Fatalf("unexpected rating sort order: %+v", byRating)
	}

	byKillsAsc := SortPlayers(players, "kills", false)
	if byKillsAsc[0].PlayerName != "p1" || byKillsAsc[2].PlayerName != "p2" {
		t.Fatalf("unexpected kills asc sort order: %+v", byKillsAsc)
	}

	rd := parser.ReplayRoundData{
		Frames: []parser.ReplayFrame{
			{
				Tick: 100,
				Players: []parser.ReplayPlayerPose{
					{Alive: true, Team: 1, X: -400, Y: 200},
					{Alive: true, Team: 2, X: 300, Y: -100},
					{Alive: false, Team: 1, X: 0, Y: 0},
				},
			},
		},
	}
	pts := BuildHeatmapPoints(nil, nil, rd, HeatmapFilter{Team: "T", Mode: "position"})
	if len(pts) != 1 || pts[0].X != -400 {
		t.Fatalf("expected 1 alive T point, got %+v", pts)
	}
}

func TestAnalyticsV2SideSplitEconomyKPIsAndH2HMatrix(t *testing.T) {
	players := []storage.MatchPlayer{
		{SteamID64: "A1", PlayerName: "Alpha1", TeamKey: "team_a", StrikeyeRating: 1.20, EcoAdjRating: 1.35, RoundSwing: 3.1, ClutchBreakdownJSON: `{"1v1":{"attempts":2,"wins":1},"1v2":{"attempts":1,"wins":1}}`},
		{SteamID64: "B1", PlayerName: "Bravo1", TeamKey: "team_b", StrikeyeRating: 1.00, EcoAdjRating: 0.95, RoundSwing: -1.4},
	}
	sides := []storage.MatchPlayerSide{
		{SteamID64: "A1", Side: "CT", Kills: 12, StrikeyeRating: 1.45, RoundSwing: 4.0},
		{SteamID64: "A1", Side: "T", Kills: 6, StrikeyeRating: 0.95, RoundSwing: 1.2},
	}

	// Side merge with CT data
	ctPlayers, ok := MergeSideStats(players, sides, "CT")
	if !ok || ctPlayers[0].Kills != 12 || ctPlayers[0].StrikeyeRating != 1.45 {
		t.Fatalf("unexpected CT merge: ok=%v players=%+v", ok, ctPlayers)
	}
	// Legacy fallback when sides is empty
	_, okEmpty := MergeSideStats(players, nil, "CT")
	if okEmpty {
		t.Fatalf("expected okEmpty=false when sides slice is empty")
	}

	// Sort by swing & eco_rating
	bySwing := SortPlayers(players, "swing", true)
	if bySwing[0].SteamID64 != "A1" {
		t.Fatalf("expected A1 first by swing, got %+v", bySwing)
	}

	// Clutch breakdown
	cb := ParseClutchBreakdown(players[0].ClutchBreakdownJSON)
	if len(cb) != 2 || cb[0].Tier != "1v1" || cb[1].Tier != "1v2" || cb[1].Wins != 1 {
		t.Fatalf("unexpected clutch breakdown: %+v", cb)
	}

	// Economy KPIs
	winA := "team_a"
	winB := "team_b"
	sideCT := "CT"
	sideT := "T"
	siteA := "A"
	rounds := []storage.MatchRound{
		{RoundNumber: 1, WinnerTeamKey: &winA, WinnerSide: &sideCT, TeamASide: "CT", TeamBSide: "T", TeamAEconomyType: "pistol", TeamBEconomyType: "pistol"},
		{RoundNumber: 2, WinnerTeamKey: &winA, WinnerSide: &sideCT, TeamASide: "CT", TeamBSide: "T", TeamAEconomyType: "full", TeamBEconomyType: "eco"},
		{RoundNumber: 3, WinnerTeamKey: &winB, WinnerSide: &sideT, TeamASide: "CT", TeamBSide: "T", TeamAEconomyType: "full", TeamBEconomyType: "force", BombSite: &siteA},
	}
	kpis := ComputeEconomyKPIs(rounds, "Team A")
	if kpis.PistolAWins != 1 || kpis.PistolTotal != 1 || kpis.ConversionAWins != 1 || kpis.ForceVsFullWins != 1 || kpis.SiteAWins != 1 {
		t.Fatalf("unexpected EconomyKPIs: %+v", kpis)
	}

	// H2H Matrix & Weapons
	a1, b1 := "A1", "B1"
	ak, awp := "ak47", "awp"
	events := []storage.MatchEvent{
		{RoundNumber: 1, EventType: "kill", ActorSteamID64: &a1, TargetSteamID64: &b1, Weapon: &awp, IsHeadshot: false},
		{RoundNumber: 2, EventType: "kill", ActorSteamID64: &b1, TargetSteamID64: &a1, Weapon: &ak, IsHeadshot: true},
	}
	pm := BuildPerformanceMatrix(players, events, "Team A")
	if pm.H2H["A1"]["B1"].Kills != 1 || pm.H2H["A1"]["B1"].Deaths != 1 {
		t.Fatalf("unexpected H2H cell: %+v", pm.H2H["A1"]["B1"])
	}
	if pm.OpeningH2H["A1"]["B1"].Kills != 1 || pm.OpeningH2H["A1"]["B1"].Deaths != 1 {
		t.Fatalf("unexpected OpeningH2H cell: %+v", pm.OpeningH2H["A1"]["B1"])
	}
	if len(pm.Weapons) != 2 || pm.TotalKills != 2 {
		t.Fatalf("unexpected weapon distribution: %+v", pm.Weapons)
	}
}

func TestReplay2DSpeedAdvance(t *testing.T) {
	v := NewView(nil, t.TempDir(), "")
	v.replayRound = parser.ReplayRoundData{
		Frames: make([]parser.ReplayFrame, 300),
	}
	v.replayPlaying = true
	v.replaySpeed = 4.0

	if changed := v.advanceReplay(500 * time.Millisecond); !changed {
		t.Fatalf("expected frame index to advance")
	}
	if v.replayFrameIdx != 64 {
		t.Fatalf("expected 64 frames after 0.5s at 4x 32Hz, got %d", v.replayFrameIdx)
	}
}
