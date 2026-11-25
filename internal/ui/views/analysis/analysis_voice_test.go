package analysis

import (
	"strings"
	"testing"

	"strikeye/internal/voice"
)

func TestVoiceBarClipsAndMarks(t *testing.T) {
	blocks := []voice.Activity{
		{XUID: 1, StartTick: 90, EndTick: 110},  // straddles the round start
		{XUID: 1, StartTick: 150, EndTick: 160}, // inside
		{XUID: 1, StartTick: 900, EndTick: 950}, // after the round
	}
	bar := voiceBar(blocks, 100, 200, 10)
	if len([]rune(bar)) != 10 {
		t.Fatalf("bar width = %d (%q)", len([]rune(bar)), bar)
	}
	if !strings.HasPrefix(bar, "█") {
		t.Fatalf("clipped leading block missing: %q", bar)
	}
	if strings.Contains(bar, "█") && strings.Trim(bar, "█·") != "" {
		t.Fatalf("unexpected glyphs: %q", bar)
	}
	if strings.Count(bar, "█") < 2 || strings.Count(bar, "█") > 6 {
		t.Fatalf("unexpected talk coverage: %q", bar)
	}
	if got := voiceBar(nil, 100, 100, 6); got != "······" {
		t.Fatalf("degenerate range = %q", got)
	}
	if got := voiceBar(blocks, 100, 200, 0); len([]rune(got)) != voiceBarWidth {
		t.Fatalf("default width not applied: %d", len([]rune(got)))
	}
}

func TestVoiceRowsOrderAndLabels(t *testing.T) {
	blocks := []voice.Activity{
		{XUID: 10, StartTick: 100, EndTick: 140},
		{XUID: 20, StartTick: 100, EndTick: 260},
		{XUID: 10, StartTick: 300, EndTick: 320},
	}
	names := map[uint64]string{10: "KSCERATO"}
	rows := voiceRows(blocks, 100, 400, 64, names, 20)
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	// 20 talks longer, so sorts first even though 10 comes first numerically.
	if rows[0].XUID != 20 || rows[1].XUID != 10 {
		t.Fatalf("order = %+v", rows)
	}
	if rows[1].Label != "KSCERATO" || rows[1].Blocks != 2 {
		t.Fatalf("named row = %+v", rows[1])
	}
	if !strings.Contains(rows[0].Label, "XUID 20") {
		t.Fatalf("fallback label = %q", rows[0].Label)
	}
	if rows[1].Seconds <= 0 {
		t.Fatalf("seconds = %f", rows[1].Seconds)
	}
}

func TestTruncateLabelKeepsRunes(t *testing.T) {
	if got := truncateLabel("KSCERATO", 12); got != "KSCERATO" {
		t.Fatalf("short label changed: %q", got)
	}
	if got := truncateLabel("abcdefghijklmnop", 5); got != "abcd…" {
		t.Fatalf("long label = %q", got)
	}
	if got := truncateLabel("中文名字中文名字", 4); got != "中文名…" {
		t.Fatalf("wide label = %q", got)
	}
}
