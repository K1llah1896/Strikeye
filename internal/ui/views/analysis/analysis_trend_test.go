package analysis

import (
	"strings"
	"testing"

	"strikeye/internal/profile"
)

func sampleTrend() []profile.TrendPoint {
	return []profile.TrendPoint{
		{Map: "de_ancient", Opponent: "Falcons", Rating: 1.1, ADR: 90, Won: true},
		{Map: "de_dust2", Opponent: "Falcons", Rating: 1.4, ADR: 110, Won: true},
		{Map: "de_mirage", Opponent: "Vitality", Rating: 0.9, ADR: 70, Won: false},
	}
}

func TestTrendSummaryRendersSparklineAndDeltas(t *testing.T) {
	line, adr := trendSummary(sampleTrend(), 10)
	if !strings.Contains(line, "近 3 场 Rating") || !strings.Contains(line, "1.10 → 0.90") {
		t.Fatalf("rating line = %q", line)
	}
	if !strings.Contains(line, "2 胜") {
		t.Fatalf("win count missing: %q", line)
	}
	if !strings.Contains(adr, "90.0 → 70.0") {
		t.Fatalf("adr line = %q", adr)
	}
	if len([]rune(adr)) == 0 || !strings.ContainsAny(adr, "▁▂▃▄▅▆▇█") {
		t.Fatalf("sparkline glyphs missing: %q", adr)
	}
}

func TestTrendSummaryTrimsToWindow(t *testing.T) {
	line, _ := trendSummary(sampleTrend(), 2)
	if !strings.Contains(line, "近 2 场") || !strings.Contains(line, "1.40 → 0.90") {
		t.Fatalf("trimmed line = %q", line)
	}
	if strings.Contains(line, "1.10") {
		t.Fatalf("window should drop the oldest game: %q", line)
	}
}

func TestTrendSummaryHandlesEmpty(t *testing.T) {
	if line, adr := trendSummary(nil, 5); line != "" || adr != "" {
		t.Fatalf("expected empty output, got %q / %q", line, adr)
	}
}

func TestTrendMatchesText(t *testing.T) {
	got := trendMatchesText(sampleTrend(), 3)
	if !strings.Contains(got, "de_ancient FALCONS 胜 1.10") {
		t.Fatalf("text = %q", got)
	}
	if !strings.Contains(got, "de_mirage VITALITY 负 0.90") {
		t.Fatalf("text = %q", got)
	}
	if short := trendMatchesText(sampleTrend(), 1); strings.Contains(short, "ANCIENT") {
		t.Fatalf("window should keep only the newest match: %q", short)
	}
	if none := trendMatchesText(nil, 3); none != "" {
		t.Fatalf("expected empty text, got %q", none)
	}
}

func TestTrendMatchesTextFallsBackForUnknownOpponent(t *testing.T) {
	points := []profile.TrendPoint{{Rating: 1.0}}
	got := trendMatchesText(points, 3)
	if !strings.Contains(got, "未知对手") || !strings.Contains(got, "?") {
		t.Fatalf("fallbacks missing: %q", got)
	}
}
