package components

import (
	"image"
	"testing"

	"strikeye/internal/parser"
)

func killFeedRound() parser.ReplayRoundData {
	return parser.ReplayRoundData{
		RoundNumber: 1,
		StartTick:   0,
		EndTick:     640,
		Frames: []parser.ReplayFrame{{
			Tick: 100,
			Players: []parser.ReplayPlayerPose{
				{Slot: 1, Team: 1, Alive: true, X: 0, Y: 0},
				{Slot: 2, Team: 2, Alive: false, X: 100, Y: 0},
			},
		}},
		Shots: []parser.ReplayShotEvent{{Tick: 98, PlayerSlot: 1, Weapon: "weapon_ak47"}},
		Deaths: []parser.ReplayDeathEvent{{
			Tick: 96, KillerSlot: 1, VictimSlot: 2, Headshot: true,
			Victim: parser.Point3D{X: 100, Y: 0}, Killer: parser.Point3D{X: 0, Y: 0},
		}},
	}
}

// pixelDiff counts differing colour channels between two rendered frames.
func pixelDiff(a, b image.Image) int {
	ra, okA := a.(*image.RGBA)
	rb, okB := b.(*image.RGBA)
	if !okA || !okB || len(ra.Pix) != len(rb.Pix) {
		return -1
	}
	diff := 0
	for i := range ra.Pix {
		if ra.Pix[i] != rb.Pix[i] {
			diff++
		}
	}
	return diff
}

func TestKillFeedRendersRecentKillsOnly(t *testing.T) {
	canvas := NewRadar2DCanvas(320)
	canvas.ConfigureMap("de_dust2", false, [4]float64{-2500, 2500, -2500, 2500})
	canvas.SetTickRate(64)
	canvas.SetSlotNames(map[uint8]string{1: "KSCERATO", 2: "NiKo"})
	round := killFeedRound()

	canvas.SetLayers(ReplayLayers{Route: true})
	off := canvas.RenderReplayFrame(round, 0)

	canvas.SetLayers(ReplayLayers{Route: true, KillFeed: true})
	on := canvas.RenderReplayFrame(round, 0)
	if diff := pixelDiff(off, on); diff < 2000 {
		t.Fatalf("kill feed panel did not render (diff bytes %d)", diff)
	}

	// A frame past the feed window must render identically with and without the layer.
	round.Frames = append(round.Frames, parser.ReplayFrame{Tick: 96 + int64(killFeedWindowSeconds*64) + 5})
	canvas.SetLayers(ReplayLayers{Route: true})
	lateOff := canvas.RenderReplayFrame(round, len(round.Frames)-1)
	canvas.SetLayers(ReplayLayers{Route: true, KillFeed: true})
	lateOn := canvas.RenderReplayFrame(round, len(round.Frames)-1)
	if diff := pixelDiff(lateOff, lateOn); diff != 0 {
		t.Fatalf("expired kills still listed (diff bytes %d)", diff)
	}
}

func TestKillWeaponPrefersClosestShot(t *testing.T) {
	round := parser.ReplayRoundData{
		Shots: []parser.ReplayShotEvent{
			{Tick: 10, PlayerSlot: 1, Weapon: "weapon_awp"},
			{Tick: 95, PlayerSlot: 1, Weapon: "weapon_ak47"},
			{Tick: 95, PlayerSlot: 3, Weapon: "weapon_m4a1"},
		},
	}
	got := parser.NearestShotWeapon(round, 1, 96)
	if got != "ak47" {
		t.Fatalf("weapon = %q, want ak47", got)
	}
	if got := parser.NearestShotWeapon(round, 9, 96); got != "" {
		t.Fatalf("expected no weapon for unknown slot, got %q", got)
	}
}

func TestSlotLabelFallsBackToNumber(t *testing.T) {
	canvas := NewRadar2DCanvas(64)
	canvas.SetSlotNames(map[uint8]string{1: "KSCERATO"})
	if got := canvas.slotLabel(1); got != "KSCERATO" {
		t.Fatalf("label = %q", got)
	}
	if got := canvas.slotLabel(7); got != "#7" {
		t.Fatalf("fallback label = %q", got)
	}
}
