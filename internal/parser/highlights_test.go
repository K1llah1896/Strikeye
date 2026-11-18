package parser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeduplicateTagsPriorityCoverage(t *testing.T) {
	in := []string{"爆头", "💥 颗秒", "🙈 盲狙", "✈️ 飞天盲狙", "💥 颗秒", "🔙 偷背身", "🔙 背刺"}
	got := DeduplicateTags(in)
	want := []string{"💥 颗秒", "✈️ 飞天盲狙", "🔙 背刺"}

	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("at %d expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestDetectCompetitiveHighlightsExcludesFailAndMemeTags(t *testing.T) {
	teamA := "team_a"
	rounds := []*RoundWindow{
		{
			RoundNumber:      5,
			StartTick:        1000,
			EndTick:          6000,
			WinnerTeamKey:    &teamA,
			TeamAEconomyType: EconomyEco,
			TeamBEconomyType: EconomyFull,
		},
	}

	kills := []KillEvent{
		{RoundNumber: 5, Tick: 2000, AttackerSteamID: "A1", AttackerName: "Carry", AttackerTeamKey: "team_a", VictimSteamID: "B1", VictimTeamKey: "team_b", Weapon: "deagle", IsHeadshot: true, Distance: 1650.0, AttackerHP: 100},
		{RoundNumber: 5, Tick: 2128, AttackerSteamID: "A1", AttackerName: "Carry", AttackerTeamKey: "team_a", VictimSteamID: "B2", VictimTeamKey: "team_b", Weapon: "deagle", IsHeadshot: true, Distance: 800.0, AttackerHP: 12},
		// Ordinary single kill by A2 -> should NOT produce a highlight clip.
		{RoundNumber: 5, Tick: 3000, AttackerSteamID: "A2", AttackerName: "Support", AttackerTeamKey: "team_a", VictimSteamID: "B3", VictimTeamKey: "team_b", Weapon: "ak47", IsHeadshot: false, Distance: 500.0, AttackerHP: 100},
	}

	clips := DetectCompetitiveHighlights(HighlightInput{
		ContentSHA256: "abc123",
		TickRate:      64.0,
		PreBufferSec:  3.0,
		PostBufferSec: 2.0,
		Rounds:        rounds,
		Kills:         kills,
		CreatedAt:     "2026-11-18T00:00:00Z",
	})

	if len(clips) != 1 {
		t.Fatalf("expected 1 competitive highlight clip, got %d", len(clips))
	}
	c := clips[0]
	if c.PlayerSteamID64 != "A1" || c.KillCount != 2 {
		t.Fatalf("unexpected clip metadata: %+v", c)
	}
	var tags []string
	if err := json.Unmarshal([]byte(c.TagsJSON), &tags); err != nil {
		t.Fatalf("unmarshal tags: %v", err)
	}

	joined := strings.Join(tags, "|")
	for _, forbidden := range []string{"下饭", "211", "首席研发工程师"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("forbidden fail/meme tag %q found in %v", forbidden, tags)
		}
	}
	// Check expected competitive tags (and ensure "爆头" was covered by "💥 颗秒"/"枪枪爆头").
	for _, required := range []string{"双杀", "爆发刷屏", "枪枪爆头", "💥 颗秒", "🔭 百步穿杨", "❤️ 极限锁血", "💸 ECO翻盘"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("expected tag %q in %v", required, tags)
		}
	}
	for _, tag := range tags {
		if tag == "爆头" {
			t.Fatalf("broad tag 爆头 should have been deduplicated by 💥 颗秒/枪枪爆头")
		}
	}
}
