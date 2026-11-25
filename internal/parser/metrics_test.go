package parser

import (
	"encoding/json"
	"math"
	"testing"

	"strikeye/internal/storage"
)

func TestComputeMetricsCappedDamageTradeKASTClutchAndRating(t *testing.T) {
	teamA := "team_a"
	rounds := []*RoundWindow{
		{RoundNumber: 1, StartTick: 100, EndTick: 1000, WinnerTeamKey: &teamA},
		{RoundNumber: 2, StartTick: 1100, EndTick: 2000, WinnerTeamKey: &teamA},
	}

	players := []PlayerMeta{
		{SteamID64: "A1", PlayerName: "Alpha1", TeamKey: "team_a"},
		{SteamID64: "A2", PlayerName: "Alpha2", TeamKey: "team_a"},
		{SteamID64: "B1", PlayerName: "Bravo1", TeamKey: "team_b"},
		{SteamID64: "B2", PlayerName: "Bravo2", TeamKey: "team_b"},
	}

	// Round 1: B1 deals 65 + 108 (capped to 35) = 100 total damage to A1, then kills A1.
	// Within 5 seconds (64*3 = 192 ticks), A2 trades B1, leaving A2 1v1 vs B2 and A2 wins the clutch!
	// Round 2: A1 gets 1 kill (B1), A2 gets 1 kill (B2), both survive.
	damages := []DamageEvent{
		{RoundNumber: 1, Tick: 200, AttackerSteamID: "B1", AttackerTeamKey: "team_b", VictimSteamID: "A1", VictimTeamKey: "team_a", Weapon: "ak47", RawHealthDamage: 65},
		{RoundNumber: 1, Tick: 220, AttackerSteamID: "B1", AttackerTeamKey: "team_b", VictimSteamID: "A1", VictimTeamKey: "team_a", Weapon: "ak47", RawHealthDamage: 108},
		{RoundNumber: 1, Tick: 350, AttackerSteamID: "A2", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimTeamKey: "team_b", Weapon: "hegrenade", RawHealthDamage: 120},
	}

	kills := []KillEvent{
		{RoundNumber: 1, Tick: 220, AttackerSteamID: "B1", AttackerName: "Bravo1", AttackerTeamKey: "team_b", VictimSteamID: "A1", VictimName: "Alpha1", VictimTeamKey: "team_a", Weapon: "ak47"},
		{RoundNumber: 1, Tick: 350, AttackerSteamID: "A2", AttackerName: "Alpha2", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimName: "Bravo1", VictimTeamKey: "team_b", Weapon: "hegrenade"},
		{RoundNumber: 1, Tick: 500, AttackerSteamID: "A2", AttackerName: "Alpha2", AttackerTeamKey: "team_a", VictimSteamID: "B2", VictimName: "Bravo2", VictimTeamKey: "team_b", Weapon: "awp", IsHeadshot: true},
		{RoundNumber: 2, Tick: 1300, AttackerSteamID: "A1", AttackerName: "Alpha1", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimName: "Bravo1", VictimTeamKey: "team_b", Weapon: "ak47", IsHeadshot: true},
		{RoundNumber: 2, Tick: 1500, AttackerSteamID: "A2", AttackerName: "Alpha2", AttackerTeamKey: "team_a", VictimSteamID: "B2", VictimName: "Bravo2", VictimTeamKey: "team_b", Weapon: "awp"},
	}

	res := ComputeMetrics(MetricsInput{
		TickRate:    64.0,
		TotalRounds: 2,
		Players:     players,
		Rounds:      rounds,
		Kills:       kills,
		Damages:     damages,
	})

	byID := make(map[string]int)
	for i, p := range res.Players {
		byID[p.SteamID64] = i
	}

	b1 := res.Players[byID["B1"]]
	if b1.TotalDamage != 100 {
		t.Fatalf("expected B1 capped damage 100, got %d", b1.TotalDamage)
	}
	if b1.ADR != 50.0 {
		t.Fatalf("expected B1 ADR 50.0, got %v", b1.ADR)
	}

	a1 := res.Players[byID["A1"]]
	if a1.TradeDeaths != 1 {
		t.Fatalf("expected A1 TradeDeaths=1, got %d", a1.TradeDeaths)
	}
	if a1.KASTPercent != 100.0 {
		t.Fatalf("expected A1 KAST=100%% (traded r1 + kill/survived r2), got %v", a1.KASTPercent)
	}

	a2 := res.Players[byID["A2"]]
	if a2.TradeKills != 1 || a2.ClutchAttempts != 1 || a2.ClutchWins != 1 || a2.AWPKills != 2 || a2.UtilityDamage != 100 {
		t.Fatalf("unexpected A2 stats: %+v", a2)
	}

	// Verify Strikeye Rating formula directly on A2.
	kpr := float64(a2.Kills) / 2.0
	dpr := float64(a2.Deaths) / 2.0
	apr := float64(a2.Assists) / 2.0
	impact := 2.13*kpr + 0.42*apr - 0.41
	expectedRating := math.Round((0.0073*a2.KASTPercent+0.3591*kpr-0.5329*dpr+0.2372*impact+0.0032*a2.ADR+0.1587)*100) / 100
	if a2.StrikeyeRating != expectedRating {
		t.Fatalf("expected A2 StrikeyeRating %.2f, got %.2f", expectedRating, a2.StrikeyeRating)
	}
}

func TestComputeMetricsV2SideSplitFlashClutchSwingAndEcoAdj(t *testing.T) {
	teamA := "team_a"
	rounds := []*RoundWindow{
		// R1: team_a on CT (eco $1000) vs team_b on T (full $22000) -> eco weight 1.65 for team_a
		{RoundNumber: 1, StartTick: 100, EndTick: 1000, TeamASide: "CT", TeamBSide: "T", TeamAEquipValue: 1000, TeamBEquipValue: 22000, WinnerTeamKey: &teamA},
		// R2: team_a on T (full $22000) vs team_b on CT (eco $1000) -> eco weight 0.45 for team_a
		{RoundNumber: 2, StartTick: 1100, EndTick: 2000, TeamASide: "T", TeamBSide: "CT", TeamAEquipValue: 22000, TeamBEquipValue: 1000, WinnerTeamKey: &teamA},
	}
	players := []PlayerMeta{
		{SteamID64: "A1", PlayerName: "Alpha1", TeamKey: "team_a"},
		{SteamID64: "A2", PlayerName: "Alpha2", TeamKey: "team_a"},
		{SteamID64: "B1", PlayerName: "Bravo1", TeamKey: "team_b"},
		{SteamID64: "B2", PlayerName: "Bravo2", TeamKey: "team_b"},
	}

	flashes := []FlashEvent{
		{RoundNumber: 1, Tick: 150, AttackerSteamID: "A1", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimTeamKey: "team_b", DurationSec: 2.4},
		// Self-flash must be ignored.
		{RoundNumber: 1, Tick: 160, AttackerSteamID: "A1", AttackerTeamKey: "team_a", VictimSteamID: "A1", VictimTeamKey: "team_a", DurationSec: 1.5},
	}
	kills := []KillEvent{
		// R1: B1 kills A2, leaving A1 in a 1v2 clutch on CT; A1 wins!
		{RoundNumber: 1, Tick: 180, AttackerSteamID: "B1", AttackerTeamKey: "team_b", VictimSteamID: "A2", VictimTeamKey: "team_a", Weapon: "ak47"},
		{RoundNumber: 1, Tick: 200, AttackerSteamID: "A1", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimTeamKey: "team_b", Weapon: "usp_silencer", IsAssistedFlash: true, AssisterSteamID: "A1"},
		{RoundNumber: 1, Tick: 300, AttackerSteamID: "A1", AttackerTeamKey: "team_a", VictimSteamID: "B2", VictimTeamKey: "team_b", Weapon: "usp_silencer"},
		// R2: A1 gets 1 kill on T vs eco
		{RoundNumber: 2, Tick: 1300, AttackerSteamID: "A1", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimTeamKey: "team_b", Weapon: "ak47"},
	}
	damages := []DamageEvent{
		{RoundNumber: 1, Tick: 200, AttackerSteamID: "A1", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimTeamKey: "team_b", RawHealthDamage: 100},
		{RoundNumber: 2, Tick: 1300, AttackerSteamID: "A1", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimTeamKey: "team_b", RawHealthDamage: 100},
	}

	res := ComputeMetrics(MetricsInput{
		TickRate:    64.0,
		TotalRounds: 2,
		Players:     players,
		Rounds:      rounds,
		Kills:       kills,
		Damages:     damages,
		Flashes:     flashes,
	})

	for _, p := range res.Players {
		if p.SteamID64 != "A1" {
			continue
		}
		if p.EnemiesFlashed != 1 || p.TotalFlashSec != 2.4 || p.FlashAssists != 1 {
			t.Fatalf("unexpected A1 flash stats: enemies=%d sec=%.1f assists=%d", p.EnemiesFlashed, p.TotalFlashSec, p.FlashAssists)
		}
		if p.RoundSwing <= 0 {
			t.Fatalf("expected positive RoundSwing for A1, got %.2f", p.RoundSwing)
		}
		// Eco-adjusted ADR: (100*1.65 + 100*0.45)/2 = 105.0 > raw ADR 100.0
		if p.EcoAdjADR <= p.ADR {
			t.Fatalf("expected EcoAdjADR (%.1f) > ADR (%.1f)", p.EcoAdjADR, p.ADR)
		}
		var cb map[string]clutchTierCount
		if err := json.Unmarshal([]byte(p.ClutchBreakdownJSON), &cb); err != nil {
			t.Fatalf("unmarshal ClutchBreakdownJSON: %v", err)
		}
		if tc, ok := cb["1v2"]; !ok || tc.Attempts != 1 || tc.Wins != 1 {
			t.Fatalf("expected 1v2 clutch win=1, got %+v", cb)
		}
	}

	// Verify CT and T side rows exist for A1.
	sideCount := 0
	for _, s := range res.Sides {
		if s.SteamID64 == "A1" && (s.Side == "CT" || s.Side == "T") {
			sideCount++
			if s.RoundsPlayed != 1 {
				t.Fatalf("expected 1 round on %s, got %d", s.Side, s.RoundsPlayed)
			}
		}
	}
	if sideCount != 2 {
		t.Fatalf("expected 2 side-split rows for A1, got %d", sideCount)
	}
}

func TestComputeMetricsDuelTiming(t *testing.T) {
	players := []PlayerMeta{
		{SteamID64: "1", PlayerName: "attacker", TeamKey: "team_a"},
		{SteamID64: "2", PlayerName: "victim", TeamKey: "team_b"},
		{SteamID64: "3", PlayerName: "other", TeamKey: "team_a"},
	}
	rounds := []*RoundWindow{{RoundNumber: 1, TeamASide: "CT", TeamBSide: "T"}}
	damages := []DamageEvent{
		{RoundNumber: 1, Tick: 100, AttackerSteamID: "3", AttackerTeamKey: "team_a", VictimSteamID: "2", VictimTeamKey: "team_b", Weapon: "ak47", RawHealthDamage: 20},
		{RoundNumber: 1, Tick: 150, AttackerSteamID: "1", AttackerTeamKey: "team_a", VictimSteamID: "2", VictimTeamKey: "team_b", Weapon: "ak47", RawHealthDamage: 30},
		// Second life: victim takes chip damage from 3 before 1 finishes.
		{RoundNumber: 1, Tick: 400, AttackerSteamID: "3", AttackerTeamKey: "team_a", VictimSteamID: "2", VictimTeamKey: "team_b", Weapon: "hegrenade", RawHealthDamage: 10},
	}
	kills := []KillEvent{
		{RoundNumber: 1, Tick: 214, AttackerSteamID: "1", AttackerName: "attacker", VictimSteamID: "2", Weapon: "ak47"},
		{RoundNumber: 1, Tick: 478, AttackerSteamID: "3", AttackerName: "other", VictimSteamID: "2", Weapon: "ak47"},
	}
	res := ComputeMetrics(MetricsInput{TickRate: 64, TotalRounds: 1, Players: players, Rounds: rounds, Kills: kills, Damages: damages})
	byID := map[string]storage.MatchPlayer{}
	for _, p := range res.Players {
		byID[p.SteamID64] = p
	}
	// 150 -> 214 = 64 ticks = 1000ms at 64 tick; the second life's chip damage at
	// 400 -> 478 = 78 ticks = 1218.75ms credits the chipper.
	if got := byID["1"].TTKAvgMS; got != 1000 {
		t.Fatalf("attacker TTK = %v, want 1000", got)
	}
	if byID["1"].TTKSamples != 1 {
		t.Fatalf("attacker samples = %d, want 1", byID["1"].TTKSamples)
	}
	if byID["3"].TTKSamples != 1 || byID["3"].TTKAvgMS != 1218.8 {
		t.Fatalf("other TTK = %v/%d, want 1218.8/1", byID["3"].TTKAvgMS, byID["3"].TTKSamples)
	}
}

func TestComputeMetricsVisibilityTTD(t *testing.T) {
	players := []PlayerMeta{
		{SteamID64: "1", PlayerName: "attacker", TeamKey: "team_a"},
		{SteamID64: "2", PlayerName: "victim", TeamKey: "team_b"},
	}
	rounds := []*RoundWindow{
		{RoundNumber: 1, TeamASide: "CT", TeamBSide: "T"},
		{RoundNumber: 2, TeamASide: "CT", TeamBSide: "T"},
	}
	ttd := []TTDSample{
		{RoundNumber: 1, AttackerSteamID: "1", DurationMS: 120},
		{RoundNumber: 1, AttackerSteamID: "1", DurationMS: 480},
		{RoundNumber: 2, AttackerSteamID: "1", DurationMS: 300},
		{RoundNumber: 1, AttackerSteamID: "2", DurationMS: 0},
	}
	res := ComputeMetrics(MetricsInput{TickRate: 64, TotalRounds: 2, Players: players, Rounds: rounds, TTDSamples: ttd})
	byID := map[string]storage.MatchPlayer{}
	for _, p := range res.Players {
		byID[p.SteamID64] = p
	}
	if got := byID["1"].TTDAvgMS; got != 300 {
		t.Fatalf("TTD avg = %v, want 300", got)
	}
	if byID["1"].TTDSamples != 3 {
		t.Fatalf("TTD samples = %d, want 3", byID["1"].TTDSamples)
	}
	if byID["2"].TTDSamples != 1 || byID["2"].TTDAvgMS != 0 {
		t.Fatalf("victim TTD = %v/%d, want 0/1", byID["2"].TTDAvgMS, byID["2"].TTDSamples)
	}
}
