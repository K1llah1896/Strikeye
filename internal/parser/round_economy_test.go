package parser

import "testing"

func TestRoundTrackerHalftimeAndEOFClosure(t *testing.T) {
	rt := NewRoundTracker(64.0)

	// Simulate warmup round reset on MatchStart.
	rt.StartRound(100, "CT", "T")
	rt.EndRound(400, "CT", "Elimination", "")
	rt.ResetMatch()

	// Official Round 1: Team A is CT, wins.
	rt.StartRound(1000, "CT", "T")
	rt.MarkFreezeEnd(1960)
	rt.EndRound(5000, "CT", "BombDefused", "A")

	// Official Round 2: Halftime swap (Team A becomes T), Team B wins, closed only by EOF.
	rt.StartRound(5200, "T", "CT")
	rt.MarkFreezeEnd(6160)
	rt.EndRound(9000, "CT", "Elimination", "")
	rt.FinalizeEOF(9320)

	rounds := rt.Rounds()
	if len(rounds) != 2 {
		t.Fatalf("expected 2 official rounds, got %d", len(rounds))
	}
	if rt.HalftimeRound() == nil || *rt.HalftimeRound() != 2 {
		t.Fatalf("expected halftime_round = 2, got %v", rt.HalftimeRound())
	}
	if rounds[0].TeamAScoreAfter != 1 || rounds[0].TeamBScoreAfter != 0 {
		t.Fatalf("unexpected r1 scores: %d-%d", rounds[0].TeamAScoreAfter, rounds[0].TeamBScoreAfter)
	}
	if rounds[1].EndTick != 9320 {
		t.Fatalf("expected EOF end_tick 9320, got %d", rounds[1].EndTick)
	}
	if rounds[1].TeamAScoreAfter != 1 || rounds[1].TeamBScoreAfter != 1 {
		t.Fatalf("unexpected r2 scores: %d-%d", rounds[1].TeamAScoreAfter, rounds[1].TeamBScoreAfter)
	}
}

func TestClassifyPlayerEconomyAndTeamVote(t *testing.T) {
	ht := 13
	if got := ClassifyPlayerEconomy(1, &ht, 800, 800, 650); got != EconomyPistol {
		t.Fatalf("round 1 should be pistol, got %s", got)
	}
	if got := ClassifyPlayerEconomy(13, &ht, 800, 800, 650); got != EconomyPistol {
		t.Fatalf("halftime round 13 should be pistol, got %s", got)
	}
	if got := ClassifyPlayerEconomy(4, &ht, 4750, 5000, 4100); got != EconomyFull {
		t.Fatalf("expected full, got %s", got)
	}
	if got := ClassifyPlayerEconomy(5, &ht, 650, 2200, 0); got != EconomyEco {
		t.Fatalf("expected eco, got %s", got)
	}
	if got := ClassifyPlayerEconomy(6, &ht, 2900, 2500, 2300); got != EconomyForce {
		t.Fatalf("expected force, got %s", got)
	}
	if got := ClassifyPlayerEconomy(7, &ht, 2200, 3500, 1200); got != EconomySemi {
		t.Fatalf("expected semi, got %s", got)
	}

	// Tie-break between force and eco: force has higher priority in _ECONOMY_ORDER.
	if got := VoteTeamEconomy([]string{EconomyEco, EconomyEco, EconomyForce, EconomyForce, EconomySemi}); got != EconomyForce {
		t.Fatalf("expected force on tie-break, got %s", got)
	}
}
