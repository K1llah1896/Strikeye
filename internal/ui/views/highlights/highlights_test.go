package highlights

import (
	"testing"

	"strikeye/internal/storage"
)

func TestFilterClipsAndBuildMontageSpec(t *testing.T) {
	vid := int64(42)
	clips := []storage.HighlightClip{
		{ID: "c1", PlayerName: "donk", RoundNumber: 3, ClipType: "ace", KillCount: 5, TagsJSON: `["ace","5K"]`, MediaVideoID: &vid, IsFavorite: true},
		{ID: "c2", PlayerName: "sh1ro", RoundNumber: 8, ClipType: "clutch", KillCount: 3, TagsJSON: `["1v3"]`, MediaVideoID: nil, IsFavorite: false},
	}

	got := FilterClips(clips, HighlightFilter{RenderedOnly: true, TagQuery: "5K"})
	if len(got) != 1 || got[0].ID != "c1" {
		t.Fatalf("expected c1, got %+v", got)
	}

	spec := BuildMontageSpec(7, "Test Montage", "", "", "", "/tmp/out.mp4", 1.0, 0.3, "fade", 0.5, true, []MontageItem{
		{
			Clip:           clips[0],
			VideoPath:      "/tmp/c1.mp4",
			TrimStartSec:   0,
			TrimEndSec:     12,
			InsertRadar:    true,
			RadarMediaPath: "/tmp/donk_radar.mp4",
			Subtitle:       "donk 5K!",
		},
	})
	if len(spec.Clips) != 1 || len(spec.RadarSegments) != 1 {
		t.Fatalf("expected 1 clip + 1 radar segment, got clips=%d radars=%d", len(spec.Clips), len(spec.RadarSegments))
	}
	if spec.Clips[0].NameCard == nil || spec.Clips[0].NameCard.PlayerName != "donk" {
		t.Fatalf("expected lower-third name card for donk")
	}
}
