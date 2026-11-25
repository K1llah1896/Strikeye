package tendency

import (
	"context"
	"testing"

	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

func seedDemo(t *testing.T, db *storage.DB, hash, teamA, teamB string, rounds int, mapName string) int64 {
	t.Helper()
	demo, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
		FilePath: "/demos/" + hash + ".dem", Filename: hash + ".dem",
		ContentSHA256: hash, TeamAName: teamA, TeamBName: teamB,
		TeamAScore: 13, TeamBScore: 6, TotalRounds: rounds,
		MapName: &mapName, ParseStatus: "parsed", TickRate: 64,
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	return demo.ID
}

func TestBuildProfilesSidesEconomySitesAndOpenings(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	demoID := seedDemo(t, db, "hash1", "FURIA", "Falcons", 4, "de_dust2")
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demoID, []storage.MatchPlayer{
		{SteamID64: "1", PlayerName: "KSCERATO", TeamKey: "team_a"},
		{SteamID64: "2", PlayerName: "NiKo", TeamKey: "team_b"},
	}); err != nil {
		t.Fatalf("players: %v", err)
	}
	slot1, slot2 := 1, 2
	players, err := db.Analysis().ListMatchPlayers(ctx, demoID)
	if err != nil {
		t.Fatalf("list players: %v", err)
	}
	for i := range players {
		switch players[i].SteamID64 {
		case "1":
			players[i].SpecSlot = &slot1
		case "2":
			players[i].SpecSlot = &slot2
		}
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demoID, players); err != nil {
		t.Fatalf("re-seed players: %v", err)
	}

	siteA, siteB := "A", "B"
	winnerA, winnerB := "team_a", "team_b"
	reasonPlant, reasonElim := "bomb_exploded", "elimination"
	rounds := []storage.MatchRound{
		// FURIA on CT, full buy, wins by elimination, no plant.
		{RoundNumber: 1, StartTick: 0, FreezeEndTick: 600, EndTick: 6000, TeamASide: "CT", TeamBSide: "T",
			WinnerTeamKey: &winnerA, WinnerSide: strPtr("CT"), WinReason: &reasonElim,
			TeamAEconomyType: "pistol", TeamBEconomyType: "pistol", TeamAEquipValue: 4000, TeamBEquipValue: 4000},
		// FURIA on T, full buy, plant B and win.
		{RoundNumber: 2, StartTick: 7000, FreezeEndTick: 7600, EndTick: 13000, TeamASide: "T", TeamBSide: "CT",
			WinnerTeamKey: &winnerA, WinnerSide: strPtr("T"), WinReason: &reasonPlant, BombSite: &siteB,
			TeamAEconomyType: "full", TeamBEconomyType: "full", TeamAEquipValue: 20000, TeamBEquipValue: 20000},
		// FURIA on T, eco, plant A and lose.
		{RoundNumber: 3, StartTick: 14000, FreezeEndTick: 14600, EndTick: 20000, TeamASide: "T", TeamBSide: "CT",
			WinnerTeamKey: &winnerB, WinnerSide: strPtr("CT"), WinReason: &reasonElim, BombSite: &siteA,
			TeamAEconomyType: "eco", TeamBEconomyType: "full", TeamAEquipValue: 3000, TeamBEquipValue: 20000},
		// FURIA on CT, force, loses.
		{RoundNumber: 4, StartTick: 21000, FreezeEndTick: 21600, EndTick: 26000, TeamASide: "CT", TeamBSide: "T",
			WinnerTeamKey: &winnerB, WinnerSide: strPtr("T"), WinReason: &reasonPlant,
			TeamAEconomyType: "force", TeamBEconomyType: "full", TeamAEquipValue: 9000, TeamBEquipValue: 20000},
	}
	if err := db.Analysis().ReplaceMatchRounds(ctx, demoID, rounds); err != nil {
		t.Fatalf("rounds: %v", err)
	}

	purchases := []storage.MatchPurchase{
		{RoundNumber: 1, SteamID64: "1", Weapon: "Smoke Grenade", Cost: 300},
		{RoundNumber: 1, SteamID64: "1", Weapon: "AK-47", Cost: 2700},
		{RoundNumber: 2, SteamID64: "1", Weapon: "Flashbang", Cost: 200},
		// Opponent purchase must not count toward FURIA's utility.
		{RoundNumber: 2, SteamID64: "2", Weapon: "Smoke Grenade", Cost: 300},
	}
	if err := db.Analysis().ReplaceMatchPurchases(ctx, demoID, purchases); err != nil {
		t.Fatalf("purchases: %v", err)
	}

	replays := map[int64][]parser.ReplayRoundData{demoID: {
		{RoundNumber: 1, Deaths: []parser.ReplayDeathEvent{
			{Tick: 1000, KillerSlot: 1, VictimSlot: 2, Victim: parser.Point3D{X: 0, Y: 0, Z: 0}},
			{Tick: 1200, KillerSlot: 2, VictimSlot: 1},
		}},
		{RoundNumber: 2, Deaths: []parser.ReplayDeathEvent{
			{Tick: 8000, KillerSlot: 2, VictimSlot: 1}, // opponent first blood
		}},
		{RoundNumber: 3, Deaths: []parser.ReplayDeathEvent{
			{Tick: 15000, KillerSlot: 1, VictimSlot: 2},
		}},
	}}

	rep, err := Build(ctx, db, Input{Team: "furia", Replays: replays, TopN: 5})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rep.Rounds != 4 || rep.Matches != 1 || rep.OpeningKills != 2 {
		t.Fatalf("totals = %+v", rep)
	}
	if len(rep.Sides) != 2 {
		t.Fatalf("sides = %+v", rep.Sides)
	}
	var ct, tv *SideProfile
	for i := range rep.Sides {
		switch rep.Sides[i].Side {
		case "CT":
			ct = &rep.Sides[i]
		case "T":
			tv = &rep.Sides[i]
		}
	}
	if ct == nil || tv == nil {
		t.Fatalf("missing side profile: %+v", rep.Sides)
	}
	if ct.Rounds != 2 || ct.Wins != 1 || ct.Pistols != 1 || ct.PistolWins != 1 || ct.UtilityPerRound != 0.5 || ct.AvgEquip != 6500 {
		t.Fatalf("CT profile = %+v", *ct)
	}
	if tv.Rounds != 2 || tv.Wins != 1 || tv.AvgEquip != 11500 || tv.UtilityPerRound != 0.5 {
		t.Fatalf("T profile = %+v", *tv)
	}

	// Only FURIA's T-side plants describe their site preference: B once, A once.
	if len(rep.Sites) != 2 {
		t.Fatalf("sites = %+v", rep.Sites)
	}
	bySite := map[string]SiteStat{}
	for _, s := range rep.Sites {
		bySite[s.Site] = s
	}
	if bySite["B"].Plants != 1 || bySite["B"].Wins != 1 || bySite["B"].WinPct != 100 {
		t.Fatalf("B site = %+v", bySite["B"])
	}
	if bySite["A"].Plants != 1 || bySite["A"].Wins != 0 {
		t.Fatalf("A site = %+v", bySite["A"])
	}

	econ := map[string]EconomyStat{}
	for _, e := range rep.Economy {
		econ[e.Side+"|"+e.Type] = e
	}
	if econ["CT|pistol"].Rounds != 1 || econ["T|full"].Wins != 1 || econ["T|eco"].WinPct != 0 {
		t.Fatalf("economy = %+v", rep.Economy)
	}
	if econ["CT|force"].Rounds != 1 || econ["CT|force"].Wins != 0 {
		t.Fatalf("force economy = %+v", econ["CT|force"])
	}

	if rep.CalloutsUsed || !rep.CalloutsMissing() {
		t.Fatalf("callouts should be reported missing when no table is supplied")
	}
	if len(rep.Opening) != 0 {
		t.Fatalf("opening callouts without a table = %+v", rep.Opening)
	}
	if rep.OpeningSecAvg != 6.25 { // (1000-600)/64 and (15000-14600)/64, averaged
		t.Fatalf("opening seconds = %v", rep.OpeningSecAvg)
	}
}

func TestBuildRequiresTeamAndIgnoresUnparsed(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := Build(ctx, db, Input{Team: "  "}); err == nil {
		t.Fatal("expected an error for an empty team")
	}
	mapName := "de_dust2"
	if _, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/pending.dem", Filename: "pending.dem", ContentSHA256: "pending",
		TeamAName: "FURIA", TeamBName: "Falcons", MapName: &mapName, ParseStatus: "pending",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rep, err := Build(ctx, db, Input{Team: "FURIA"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rep.Rounds != 0 || rep.Matches != 0 {
		t.Fatalf("unparsed demos must be ignored: %+v", rep)
	}
}

func strPtr(s string) *string { return &s }
