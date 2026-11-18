package radar

import (
	"math"
	"testing"

	"strikeye/internal/storage"
)

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func TestNormalizeClampingAndFullScale(t *testing.T) {
	raw := RawValues{
		KPR:       0.85,  // 1.0
		Surviving: 0.44,  // 1.0
		ADR:       170.0, // 2.0 -> clamped to 1.6
		KAST:      0.78,  // 1.0
		MultiKill: 10,    // 1.25
		Rating:    1.30,  // 1.0
	}
	norm := Normalize(raw)
	if !almostEqual(norm.KPR, 1.0, 1e-6) {
		t.Fatalf("expected KPR=1.0, got %v", norm.KPR)
	}
	if !almostEqual(norm.Surviving, 1.0, 1e-6) {
		t.Fatalf("expected Surviving=1.0, got %v", norm.Surviving)
	}
	if !almostEqual(norm.ADR, NormalizeCeiling, 1e-6) {
		t.Fatalf("expected ADR clamped to %v, got %v", NormalizeCeiling, norm.ADR)
	}
	if !almostEqual(norm.KAST, 1.0, 1e-6) {
		t.Fatalf("expected KAST=1.0, got %v", norm.KAST)
	}
	if !almostEqual(norm.MultiKill, 1.25, 1e-6) {
		t.Fatalf("expected MultiKill=1.25, got %v", norm.MultiKill)
	}
	if !almostEqual(norm.Rating, 1.0, 1e-6) {
		t.Fatalf("expected Rating=1.0, got %v", norm.Rating)
	}
}

func TestDeriveZeroAndNaNSafety(t *testing.T) {
	p := storage.MatchPlayer{
		KPR:            math.NaN(),
		SurvivalRate:   -5,
		ADR:            0,
		KASTPercent:    0,
		TwoKillRounds:  2,
		ThreeKillRounds: 1,
		StrikeyeRating: 0,
	}
	raw := DeriveFromMatchPlayer(p)
	if raw.KPR != 0 || raw.Surviving != 0 || raw.MultiKill != 3 {
		t.Fatalf("unexpected derived raw values: %+v", raw)
	}
	norm := Normalize(raw)
	if norm.KPR != 0 || norm.Surviving != 0 || !almostEqual(norm.MultiKill, 3.0/8.0, 1e-6) {
		t.Fatalf("unexpected normalized values: %+v", norm)
	}
}

func TestComputeMatchAverage(t *testing.T) {
	if avg := ComputeMatchAverage(nil); avg != (NormalizedValues{}) {
		t.Fatalf("expected zero average for empty input, got %+v", avg)
	}
	p1 := RawValues{KPR: 0.85, Surviving: 0.44, ADR: 85, KAST: 0.78, MultiKill: 8, Rating: 1.30}
	p2 := RawValues{KPR: 0.425, Surviving: 0.22, ADR: 42.5, KAST: 0.39, MultiKill: 4, Rating: 0.65}
	avg := ComputeMatchAverage([]RawValues{p1, p2})
	for i, v := range avg.Slice() {
		if !almostEqual(v, 0.75, 1e-6) {
			t.Fatalf("axis %d expected 0.75, got %v", i, v)
		}
	}
}
