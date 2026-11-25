package parser

import (
	"math"
	"path/filepath"
	"testing"

	"strikeye/internal/mapgeom"
)

func zoomRound() ReplayRoundData {
	return ReplayRoundData{
		RoundNumber: 1,
		StartTick:   64,
		EndTick:     640,
		Frames: []ReplayFrame{{
			Tick: 128,
			Players: []ReplayPlayerPose{
				{Slot: 1, Team: 1, Alive: true, X: 100, Y: 200, Z: 10, Yaw: 45, Pitch: -2, Zoom: 2, ActiveWeapon: "awp"},
				{Slot: 2, Team: 2, Alive: true, X: 300, Y: 400, Z: 10, Yaw: 180, Zoom: 0, ActiveWeapon: "ak-47"},
			},
		}},
	}
}

func TestReplayZoomRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zoom.rpl")
	if err := WriteReplayFileAtomic(path, 64, InputCapEntityButtons, []ReplayRoundData{zoomRound()}); err != nil {
		t.Fatalf("write: %v", err)
	}
	hdr, rd, err := ReadReplayRound(path, 1)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if hdr.Version != 3 {
		t.Fatalf("version = %d, want 3", hdr.Version)
	}
	players := rd.Frames[0].Players
	if len(players) != 2 {
		t.Fatalf("players = %d", len(players))
	}
	if players[0].Zoom != 2 || players[1].Zoom != 0 {
		t.Fatalf("zoom levels = %d/%d, want 2/0", players[0].Zoom, players[1].Zoom)
	}
	if players[0].Yaw != 45 || players[0].ActiveWeapon != "awp" {
		t.Fatalf("pose fields damaged by zoom byte: %+v", players[0])
	}
}

func TestReplayV2FilesReadZoomAsZero(t *testing.T) {
	prev := ReplayFormatVersion
	ReplayFormatVersion = 2
	t.Cleanup(func() { ReplayFormatVersion = prev })

	path := filepath.Join(t.TempDir(), "legacy.rpl")
	if err := WriteReplayFileAtomic(path, 64, InputCapEntityButtons, []ReplayRoundData{zoomRound()}); err != nil {
		t.Fatalf("write: %v", err)
	}
	hdr, rd, err := ReadReplayRound(path, 1)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if hdr.Version != 2 {
		t.Fatalf("version = %d, want 2", hdr.Version)
	}
	for _, p := range rd.Frames[0].Players {
		if p.Zoom != 0 {
			t.Fatalf("v2 file produced zoom %d for slot %d", p.Zoom, p.Slot)
		}
	}
}

func TestScopedFOVHalfAngle(t *testing.T) {
	cases := []struct {
		zoom uint8
		want float64
	}{
		{0, ttdFOVHalfAngle},
		{1, ttdScopedHalfAngle},
		{2, ttdScopedFullHalfAngle},
		{9, ttdFOVHalfAngle},
	}
	for _, c := range cases {
		if got := scopedFOVHalfAngle(ttdFOVHalfAngle, c.zoom); got != c.want {
			t.Fatalf("zoom %d -> %v, want %v", c.zoom, got, c.want)
		}
	}
}

// TestVisibleRunRespectsZoom checks that zooming narrows the visible cone.
func TestVisibleRunRespectsZoom(t *testing.T) {
	const tickRate = 64.0
	const attacker, victim = "att", "vic"
	// The victim sits 30° off the attacker's view: inside the 53° hip cone,
	// outside the 20° half-zoom cone.
	build := func() []visSample {
		samples := make([]visSample, 0, 64)
		off := 30 * math.Pi / 180
		for tick := int64(0); tick < 64; tick++ {
			samples = append(samples, visSample{
				Tick:  tick,
				Pos:   map[string][3]float64{attacker: {0, 0, 64}, victim: {100 * math.Cos(off), 100 * math.Sin(off), 64}},
				Yaw:   map[string]float64{attacker: 0},
				Blind: map[string]float64{attacker: 0},
			})
		}
		return samples
	}
	tracer := mapgeom.NewTracer(nil) // no geometry: nothing is occluded

	samples := build()
	if _, ok := visibleRunMSOptics(tracer, samples, attacker, victim, 63, 64, tickRate, ttdFOVHalfAngle); !ok {
		t.Fatal("hip fire should see a target 30° off")
	}
	for i := range samples {
		samples[i].Zoom = map[string]uint8{attacker: 2}
	}
	if _, ok := visibleRunMSOptics(tracer, samples, attacker, victim, 63, 64, tickRate, ttdFOVHalfAngle); ok {
		t.Fatal("full zoom should lose a target 30° off")
	}
	for i := range samples {
		samples[i].Zoom = map[string]uint8{attacker: 1}
		samples[i].Yaw[attacker] = 15 // 15° off, inside the 20° half-zoom cone
	}
	if _, ok := visibleRunMSOptics(tracer, samples, attacker, victim, 63, 64, tickRate, ttdFOVHalfAngle); !ok {
		t.Fatal("half zoom should keep a target 15° off")
	}
}
