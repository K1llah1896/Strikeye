package analysis

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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

func TestReplayAndHeatmapTiming(t *testing.T) {
	v := NewView(nil, "", "")
	mapName := "de_mirage"
	v.demo = &storage.Demo{ID: 1, MapName: &mapName}
	var frames []parser.ReplayFrame
	for i := 0; i < 3200; i++ {
		frames = append(frames, parser.ReplayFrame{
			Tick: int64(i * 2),
			Players: []parser.ReplayPlayerPose{
				{Alive: true, Team: 1, X: float32(-500 + (i % 400)), Y: float32(200 - (i % 300))},
				{Alive: true, Team: 2, X: float32(300 - (i % 350)), Y: float32(-100 + (i % 250))},
			},
		})
	}
	v.replayRound = parser.ReplayRoundData{Frames: frames}

	t0 := time.Now()
	v.ensureReplayReady()
	dReplay := time.Since(t0)

	t1 := time.Now()
	v.ensureHeatmapReady()
	dHeatmap := time.Since(t1)

	t.Logf("ensureReplayReady=%v, ensureHeatmapReady=%v", dReplay, dHeatmap)
}

func TestAsyncTabSwitchAndPlaybackLifecycle(t *testing.T) {
	v := NewView(nil, "", "")
	defer v.Close()

	mapName := "de_mirage"
	v.demo = &storage.Demo{ID: 1, MapName: &mapName}
	frames := make([]parser.ReplayFrame, 128)
	for i := range frames {
		frames[i] = parser.ReplayFrame{
			Tick: int64(i * 2),
			Players: []parser.ReplayPlayerPose{
				{Slot: 1, Alive: true, Team: 1, Health: 100, X: float32(-500 + i), Y: 200},
				{Slot: 2, Alive: true, Team: 2, Health: 100, X: float32(300 - i), Y: -100},
			},
		}
	}
	v.replayRound = parser.ReplayRoundData{Frames: frames}

	doneCh := make(chan struct{}, 16)
	v.SetInvalidate(func() {
		select {
		case doneCh <- struct{}{}:
		default:
		}
	})

	// Switching tabs must return immediately without blocking on rasterization/KDE.
	v.SetActiveTab(TabReplay2D)
	v.SetActiveTab(TabHeatmaps)
	v.SetActiveTab(TabReplay2D)

	// Start background 32Hz playback and confirm it advances frames and stops cleanly.
	v.mu.Lock()
	v.startPlaybackLocked()
	v.mu.Unlock()

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected background invalidation callback")
	}

	v.SetActiveTab(TabOverview)
	v.mu.RLock()
	stillPlaying := v.replayPlaying
	v.mu.RUnlock()
	if stillPlaying {
		t.Fatalf("expected playback to stop when leaving TabReplay2D")
	}
}

func TestTrimLeadingStationaryFrames(t *testing.T) {
	rd := parser.ReplayRoundData{
		Frames: []parser.ReplayFrame{
			{Tick: 10, Players: []parser.ReplayPlayerPose{{Slot: 1, Alive: true, X: 100, Y: 200}}},
			{Tick: 12, Players: []parser.ReplayPlayerPose{{Slot: 1, Alive: true, X: 100.5, Y: 200.5}}},
			{Tick: 14, Players: []parser.ReplayPlayerPose{{Slot: 1, Alive: true, X: 101, Y: 200}}},
			{Tick: 16, Players: []parser.ReplayPlayerPose{{Slot: 1, Alive: true, X: 120, Y: 200}}},
			{Tick: 18, Players: []parser.ReplayPlayerPose{{Slot: 1, Alive: true, X: 140, Y: 200}}},
		},
	}
	trimmed := TrimLeadingStationaryFrames(rd)
	if len(trimmed.Frames) != 3 || trimmed.Frames[0].Tick != 14 {
		t.Fatalf("expected 3 frames starting at tick 14 (1 frame before move), got len=%d firstTick=%d",
			len(trimmed.Frames), trimmed.Frames[0].Tick)
	}
}

func TestRoundEconomySideLoadoutAndRemainingMoney(t *testing.T) {
	r := storage.MatchRound{RoundNumber: 5, TeamASide: "T", TeamBSide: "CT"}

	peExplicit := parser.PlayerRoundEconomy{
		PlayerName:     "donk",
		TeamKey:        "team_a",
		Side:           "T",
		EquipValue:     4700,
		AccountStart:   5400,
		CashSpent:      4200,
		MoneyRemaining: 1200,
		Weapons:        []string{"AK-47", "Glock-18", "Smoke", "Flash"},
		HasArmor:       true,
		HasHelmet:      true,
	}
	if got := resolvePlayerEconomySide(peExplicit, r, "Team A"); got != "T" {
		t.Fatalf("expected T, got %s", got)
	}
	if got := playerRemainingMoney(peExplicit); got != 1200 {
		t.Fatalf("expected remaining 1200, got %d", got)
	}
	if got := formatPlayerLoadout(peExplicit); got != "全甲 · AK-47 · Glock-18 · Smoke · Flash" {
		t.Fatalf("unexpected loadout: %q", got)
	}

	peLegacy := parser.PlayerRoundEconomy{
		PlayerName:   "sh1ro",
		TeamKey:      "team_b",
		EquipValue:   5500,
		AccountStart: 6000,
		CashSpent:    4750,
		EconomyType:  "full",
		HasArmor:     true,
		HasDefuseKit: true,
	}
	if got := resolvePlayerEconomySide(peLegacy, r, "Team A"); got != "CT" {
		t.Fatalf("expected legacy CT fallback, got %s", got)
	}
	if got := playerRemainingMoney(peLegacy); got != 1250 {
		t.Fatalf("expected legacy remaining 1250, got %d", got)
	}
	if got := formatPlayerLoadout(peLegacy); got != "半甲 · 钳" {
		t.Fatalf("unexpected legacy gear loadout: %q", got)
	}
}

func TestBuildHeatmapPointsMultiFilters(t *testing.T) {
	slot1, slot2 := 1, 2
	players := []storage.MatchPlayer{
		{SteamID64: "S1", PlayerName: "donk", TeamKey: "team_a", SpecSlot: &slot1},
		{SteamID64: "S2", PlayerName: "ropz", TeamKey: "team_b", SpecSlot: &slot2},
	}
	rounds := []storage.MatchRound{
		{RoundNumber: 1, TeamASide: "T", TeamBSide: "CT", TeamAEconomyType: "pistol", TeamBEconomyType: "pistol"},
		{RoundNumber: 4, TeamASide: "T", TeamBSide: "CT", TeamAEconomyType: "full", TeamBEconomyType: "full"},
	}
	replays := map[int]parser.ReplayRoundData{
		1: {
			RoundNumber: 1,
			Frames: []parser.ReplayFrame{
				{Tick: 100, Players: []parser.ReplayPlayerPose{
					{Slot: 1, Alive: true, Team: 1, X: -100, Y: 200},
					{Slot: 2, Alive: true, Team: 2, X: 300, Y: -400},
				}},
			},
		},
		4: {
			RoundNumber: 4,
			Frames: []parser.ReplayFrame{
				{Tick: 500, Players: []parser.ReplayPlayerPose{
					{Slot: 1, Alive: true, Team: 1, X: -500, Y: 600},
					{Slot: 2, Alive: true, Team: 2, X: 700, Y: -800},
				}},
			},
		},
	}

	s1, s2 := "S1", "S2"
	tkA, tkB := "team_a", "team_b"
	events := []storage.MatchEvent{
		{RoundNumber: 4, Tick: 500, EventType: "kill", ActorSteamID64: &s1, ActorTeamKey: &tkA, TargetSteamID64: &s2, TargetTeamKey: &tkB},
	}

	// 1. position mode filtered by player + round type
	fPos := resolveHeatmapFilter(players, "donk", "全部阵营", "position", "长枪局", "Team A")
	ptsPos := BuildHeatmapPointsMulti(events, rounds, players, replays, fPos)
	if len(ptsPos) != 1 || ptsPos[0].X != -500 {
		t.Fatalf("expected 1 position point for donk in full round, got %+v", ptsPos)
	}

	// 2. kill mode with nil ExtraJSON falls back to .cs2rpl frames
	fKill := resolveHeatmapFilter(players, "donk", "T", "kill", "长枪局", "Team A")
	ptsKill := BuildHeatmapPointsMulti(events, rounds, players, replays, fKill)
	if len(ptsKill) != 1 || ptsKill[0].X != -500 {
		t.Fatalf("expected 1 kill point at -500, got %+v", ptsKill)
	}

	// 3. death mode for ropz falls back to victim coords (700, -800)
	fDeath := resolveHeatmapFilter(players, "ropz", "CT", "death", "长枪局", "Team A")
	ptsDeath := BuildHeatmapPointsMulti(events, rounds, players, replays, fDeath)
	if len(ptsDeath) != 1 || ptsDeath[0].X != 700 {
		t.Fatalf("expected 1 death point at 700, got %+v", ptsDeath)
	}

	// 4. firefight mode with no matching round type returns 0 points (not all rounds)
	fEco := resolveHeatmapFilter(players, "全部选手", "全部阵营", "firefight", "Eco", "Team A")
	ptsEco := BuildHeatmapPointsMulti(events, rounds, players, replays, fEco)
	if len(ptsEco) != 0 {
		t.Fatalf("expected 0 eco points when no eco rounds exist, got %+v", ptsEco)
	}
}

func TestKillFeedRendersOfficialWeaponAndModifierSVGs(t *testing.T) {
	v := NewView(nil, "", "")
	v.demo = &storage.Demo{ID: 1, TeamAName: "Team A", TeamBName: "Team B"}
	att, vic, wep, ass := "donk", "ropz", "AK-47", "magixx"
	tkA, tkB := "team_a", "team_b"
	v.events = []storage.MatchEvent{
		{
			RoundNumber:     3,
			RoundTimeSec:    24.5,
			EventType:       "kill",
			ActorName:       &att,
			ActorTeamKey:    &tkA,
			TargetName:      &vic,
			TargetTeamKey:   &tkB,
			AssisterName:    &ass,
			Weapon:          &wep,
			IsHeadshot:      true,
			IsWallbang:      true,
			IsThroughSmoke:  true,
			IsAssistedFlash: true,
		},
	}
	r := storage.MatchRound{RoundNumber: 3, TeamASide: "T", TeamBSide: "CT"}

	tt := ui.NewTester(func(c *ui.Context) {
		v.renderRoundDetailDrawer(c, r)
	}, 1100, 600)
	tt.Frame()
	if !tt.HasText("donk") || !tt.HasText("ropz") {
		t.Fatalf("expected attacker and victim names in killfeed")
	}
}

func TestBuildHeatmapPointsSetupSamplesAtTwentySeconds(t *testing.T) {
	rounds := []storage.MatchRound{{RoundNumber: 1, FreezeEndTick: 1000, TeamASide: "T", TeamBSide: "CT"}}
	replays := map[int]parser.ReplayRoundData{
		1: {
			RoundNumber: 1,
			Frames: []parser.ReplayFrame{
				{Tick: 1000, Players: []parser.ReplayPlayerPose{
					{Slot: 1, Alive: true, Team: 1, X: -900, Y: -900},
					{Slot: 2, Alive: true, Team: 2, X: 900, Y: 900},
				}},
				{Tick: 1640, Players: []parser.ReplayPlayerPose{
					{Slot: 1, Alive: true, Team: 1, X: 120, Y: 240},
					{Slot: 2, Alive: false, Team: 2, X: 0, Y: 0},
				}},
			},
		},
	}
	pts := BuildHeatmapPointsMulti(nil, rounds, nil, replays, HeatmapFilter{Mode: "setup", TickRate: 32})
	if len(pts) != 1 || pts[0].X != 120 || pts[0].Y != 240 {
		t.Fatalf("setup points = %+v, want the alive player at 20s", pts)
	}
	ct := BuildHeatmapPointsMulti(nil, rounds, nil, replays, HeatmapFilter{Mode: "setup", TickRate: 32, Team: "CT"})
	if len(ct) != 0 {
		t.Fatalf("dead CT should not be sampled, got %+v", ct)
	}
}
