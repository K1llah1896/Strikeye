package analysis

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/storage"
	"strikeye/internal/ui/renderutil"
)

// seedRenderAnalysis seeds one parsed demo with two players, three rounds, a few
// events and purchases so every analysis tab has something to lay out.
func seedRenderAnalysis(t *testing.T) (*storage.DB, string, int64) {
	t.Helper()
	root := t.TempDir()
	db, err := storage.OpenDefault(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	mapName, reason, side := "de_ancient", "target_bombed", "ct"
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/render.dem", Filename: "render.dem", ContentSHA256: "render",
		TeamAName: "Falcons", TeamBName: "FURIA", MapName: &mapName,
		ParseStatus: "parsed", TickRate: 64, TotalRounds: 3,
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	players := []storage.MatchPlayer{
		{DemoID: demo.ID, SteamID64: "76561198000000001", PlayerName: "KSCERATO", TeamKey: "team_b",
			SpecSlot: intPtr(4), Kills: 20, Deaths: 12, Assists: 4, ADR: 105.5, KASTPercent: 72,
			StrikeyeRating: 1.4, Headshots: 9, HSPercent: 45, FirstKills: 3, TTKAvgMS: 420,
			TTKSamples: 9, TTDAvgMS: 480, TTDSamples: 11, TotalDamage: 2100},
		{DemoID: demo.ID, SteamID64: "76561198000000002", PlayerName: "NiKo", TeamKey: "team_a",
			SpecSlot: intPtr(5), Kills: 15, Deaths: 18, Assists: 2, ADR: 78.2, KASTPercent: 65,
			StrikeyeRating: 1.0, Headshots: 5, HSPercent: 33, FirstKills: 1, TotalDamage: 1500},
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, players); err != nil {
		t.Fatalf("players: %v", err)
	}
	rounds := make([]storage.MatchRound, 0, 3)
	for i := 1; i <= 3; i++ {
		rounds = append(rounds, storage.MatchRound{
			DemoID: demo.ID, RoundNumber: i,
			WinnerTeamKey: strPtr("team_b"), WinnerSide: &side, WinReason: &reason,
		})
	}
	if err := db.Analysis().ReplaceMatchRounds(ctx, demo.ID, rounds); err != nil {
		t.Fatalf("rounds: %v", err)
	}
	if err := db.Analysis().ReplaceMatchEvents(ctx, demo.ID, []storage.MatchEvent{
		{DemoID: demo.ID, RoundNumber: 1, Tick: 6000, EventType: "kill",
			ActorName: strPtr("KSCERATO"), TargetName: strPtr("NiKo"), Weapon: strPtr("ak47")},
	}); err != nil {
		t.Fatalf("events: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPurchases(ctx, demo.ID, []storage.MatchPurchase{
		{RoundNumber: 1, Tick: 5200, TimeSec: -6, SteamID64: "76561198000000001",
			PlayerName: "KSCERATO", Weapon: "AK-47", Cost: 2700},
	}); err != nil {
		t.Fatalf("purchases: %v", err)
	}
	return db, root, demo.ID
}

func strPtr(s string) *string { return &s }

func intPtr(v int) *int { return &v }

// TestRenderAllAnalysisTabs renders every tab of the analysis view: a nil pointer
// or bad nil-guard in any of the eight pages fails here instead of in the app.
func TestRenderAllAnalysisTabs(t *testing.T) {
	db, root, demoID := seedRenderAnalysis(t)
	v := NewView(db, root, "")
	// Rendering must not depend on Steam: the avatar fetch is exercised in
	// analysis_avatars_test.go with an injected client instead.
	v.avatarAutoFetch = false
	if err := v.SelectDemo(context.Background(), demoID); err != nil {
		t.Fatalf("select demo: %v", err)
	}
	for _, tab := range []Tab{TabOverview, TabEconomy, TabPerformance, TabRadar, TabReplay2D, TabHeatmaps, TabZones, TabTeam} {
		v.SetActiveTab(tab)
		tab := tab
		renderutil.AssertRendered(t, 30, func(c *ui.Context) { v.Render(c) })
		_ = tab
	}
}

// TestRenderExpandedPlayerDrawer renders the drawer with the profile editor,
// trend sparkline and purchase rows visible.
func TestRenderExpandedPlayerDrawer(t *testing.T) {
	db, root, demoID := seedRenderAnalysis(t)
	v := NewView(db, root, "")
	// Rendering must not depend on Steam: the avatar fetch is exercised in
	// analysis_avatars_test.go with an injected client instead.
	v.avatarAutoFetch = false
	if err := v.SelectDemo(context.Background(), demoID); err != nil {
		t.Fatalf("select demo: %v", err)
	}
	v.SetActiveTab(TabOverview)
	v.expandedPlayerSID = "76561198000000001"
	v.profileDraft = profileDraft{SteamID: "76561198000000001", DisplayName: "KSC", Note: "anchor", Tags: "clutch,anchor"}
	v.avatarMsg = "头像：新下载 1，已缓存 2"
	renderutil.AssertRendered(t, 30, func(c *ui.Context) { v.Render(c) })
}
