package highlights

import (
	"testing"

	"strikeye/internal/composer"
)

func TestToAudioTracksDropsEmptyRowsAndNormalises(t *testing.T) {
	drafts := []audioTrackDraft{
		{Path: "  voice.wav  ", OffsetText: "0.5", VolumeText: "0.7"},
		{Path: "", OffsetText: "2", VolumeText: "1"},
		{Path: "crowd.wav", OffsetText: "", VolumeText: ""},
		{Path: "loud.wav", VolumeText: "9", Mute: true},
	}
	tracks := toAudioTracks(drafts)
	if len(tracks) != 3 {
		t.Fatalf("tracks = %+v", tracks)
	}
	if tracks[0].Path != "voice.wav" || tracks[0].OffsetSec != 0.5 || tracks[0].Volume != 0.7 {
		t.Fatalf("first = %+v", tracks[0])
	}
	// Empty volume falls back to unity; empty offset falls back to zero.
	if tracks[1].OffsetSec != 0 || tracks[1].Volume != composer.DefaultTrackVolume {
		t.Fatalf("second = %+v", tracks[1])
	}
	// Volume above 4 is clamped, mute is preserved.
	if tracks[2].Volume != 4 || !tracks[2].Mute {
		t.Fatalf("third = %+v", tracks[2])
	}
}

func TestToAudioTracksNegativeOffsetKept(t *testing.T) {
	tracks := toAudioTracks([]audioTrackDraft{{Path: "a.wav", OffsetText: "-1.25"}})
	if len(tracks) != 1 || tracks[0].OffsetSec != -1.25 {
		t.Fatalf("tracks = %+v", tracks)
	}
}

func TestParseTrackFloatFallsBack(t *testing.T) {
	cases := []struct {
		raw  string
		def  float64
		want float64
	}{
		{"", 1, 1},
		{"  ", 0.5, 0.5},
		{"abc", 2, 2},
		{"3.5", 0, 3.5},
		{"-2", 0, -2},
	}
	for _, tc := range cases {
		if got := parseTrackFloat(tc.raw, tc.def); got != tc.want {
			t.Fatalf("parseTrackFloat(%q, %v) = %v, want %v", tc.raw, tc.def, got, tc.want)
		}
	}
}

func TestDefaultAudioTracksShape(t *testing.T) {
	drafts := defaultAudioTracks()
	if len(drafts) != maxAudioTracks {
		t.Fatalf("rows = %d", len(drafts))
	}
	for _, d := range drafts {
		if d.Path != "" || d.OffsetText != "" || d.VolumeText != "" || d.Mute {
			t.Fatalf("row not empty: %+v", d)
		}
	}
	if got := toAudioTracks(drafts); len(got) != 0 {
		t.Fatalf("empty drafts produced %+v", got)
	}
}
