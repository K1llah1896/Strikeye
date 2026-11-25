package components

import (
	"image"
	"testing"

	"strikeye/internal/parser"
)

func countPixels(img image.Image, pred func(r, g, b uint8) bool) int {
	bounds := img.Bounds()
	n := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if pred(uint8(r>>8), uint8(g>>8), uint8(b>>8)) {
				n++
			}
		}
	}
	return n
}

func combatTestRound() parser.ReplayRoundData {
	return parser.ReplayRoundData{
		RoundNumber: 1,
		StartTick:   100,
		EndTick:     900,
		Frames: []parser.ReplayFrame{{
			Tick: 100,
			Players: []parser.ReplayPlayerPose{{
				Slot: 1, Team: 1, Alive: true, Health: 100, X: -500, Y: 0, Z: 0, Yaw: 0,
			}},
		}},
		Shots: []parser.ReplayShotEvent{{
			Tick: 95, PlayerSlot: 1,
			Start: parser.Point3D{X: -500, Y: 0, Z: 64},
			End:   parser.Point3D{X: 500, Y: 0, Z: 64},
		}},
		Deaths: []parser.ReplayDeathEvent{{
			Tick: 90, KillerSlot: 1, VictimSlot: 2, Headshot: true,
			Victim: parser.Point3D{X: 0, Y: 300, Z: 0},
			Killer: parser.Point3D{X: -500, Y: 0, Z: 0},
		}},
		Bombs: []parser.ReplayBombEvent{{
			Tick: 50, Kind: parser.ReplayBombPlant,
			Pos: parser.Point3D{X: 300, Y: -300, Z: 0},
		}, {
			Tick: 60, Kind: parser.ReplayBombExplode,
			Pos: parser.Point3D{X: 300, Y: -300, Z: 0},
		}},
	}
}

func newTestCanvas() *Radar2DCanvas {
	c := NewRadar2DCanvas(256)
	c.ConfigureMap("__no_such_map__", false, [4]float64{-2500, 2500, -2500, 2500})
	return c
}

func isTracerPixel(r, g, b uint8) bool {
	// Tracer yellow blended over the obsidian backdrop.
	return r > 130 && g > 100 && b < 100 && int(g) > int(b)+40
}

func isBlastPixel(r, g, b uint8) bool {
	// Deep red blast fill blended over the obsidian backdrop.
	return r > 90 && int(r) > int(g)+50 && int(r) > int(b)+50
}

func TestReplayCanvasCombatLayersToggle(t *testing.T) {
	round := combatTestRound()

	all := newTestCanvas()
	withLayers := all.RenderReplayFrame(round, 0)
	tracerOn := countPixels(withLayers, isTracerPixel)
	blastOn := countPixels(withLayers, isBlastPixel)
	if tracerOn == 0 {
		t.Fatalf("expected shot tracer pixels with layers enabled")
	}
	if blastOn == 0 {
		t.Fatalf("expected blast circle pixels with layers enabled")
	}

	all.SetLayers(ReplayLayers{})
	none := all.RenderReplayFrame(round, 0)
	if got := countPixels(none, isTracerPixel); got >= tracerOn {
		t.Fatalf("tracer pixels did not disappear: on=%d off=%d", tracerOn, got)
	}
	if got := countPixels(none, isBlastPixel); got >= blastOn {
		t.Fatalf("blast pixels did not disappear: on=%d off=%d", blastOn, got)
	}

	onlyDeaths := newTestCanvas()
	onlyDeaths.SetLayers(ReplayLayers{Deaths: true})
	deathsOnly := onlyDeaths.RenderReplayFrame(round, 0)
	if got := countPixels(deathsOnly, isTracerPixel); got >= tracerOn {
		t.Fatalf("shots rendered despite toggle off: %d", got)
	}
	if _, ok := deathsOnly.(*image.RGBA); !ok {
		t.Fatalf("expected RGBA frame image")
	}
}

func TestReplayCanvasIgnoresFutureCombatEvents(t *testing.T) {
	c := newTestCanvas()
	round := combatTestRound()
	round.Frames[0].Tick = 10 // before every combat event
	withLayers := c.RenderReplayFrame(round, 0)
	c.SetLayers(ReplayLayers{})
	withoutLayers := c.RenderReplayFrame(round, 0)
	if a, b := countPixels(withLayers, isBlastPixel), countPixels(withoutLayers, isBlastPixel); a != b {
		t.Fatalf("future blast/plant drawn early: %d vs %d pixels", a, b)
	}
	if a, b := countPixels(withLayers, isTracerPixel), countPixels(withoutLayers, isTracerPixel); a != b {
		t.Fatalf("future shot drawn early: %d vs %d pixels", a, b)
	}
}

func TestRouteLayerDrawsTeamTrails(t *testing.T) {
	round := parser.ReplayRoundData{
		RoundNumber: 1,
		Frames: []parser.ReplayFrame{
			{Tick: 0, Players: []parser.ReplayPlayerPose{{Slot: 1, Team: 1, Alive: true, Health: 100, X: -800, Y: 0}}},
			{Tick: 8, Players: []parser.ReplayPlayerPose{{Slot: 1, Team: 1, Alive: true, Health: 100, X: -400, Y: 0}}},
			{Tick: 16, Players: []parser.ReplayPlayerPose{{Slot: 1, Team: 1, Alive: true, Health: 100, X: 0, Y: 0}}},
			{Tick: 24, Players: []parser.ReplayPlayerPose{{Slot: 2, Team: 2, Alive: true, Health: 100, X: 800, Y: 200}}},
			{Tick: 32, Players: []parser.ReplayPlayerPose{{Slot: 2, Team: 2, Alive: true, Health: 100, X: 400, Y: 200}}},
		},
	}
	c := newTestCanvas()
	c.SetLayers(ReplayLayers{})
	off := c.RenderReplayFrame(round, 4)
	offAmber := countPixels(off, func(r, g, b uint8) bool { return r > 100 && g > 60 && b < 60 })

	c.SetLayers(ReplayLayers{Route: true})
	on := c.RenderReplayFrame(round, 4)
	onAmber := countPixels(on, func(r, g, b uint8) bool { return r > 100 && g > 60 && b < 60 })
	if onAmber <= offAmber {
		t.Fatalf("route layer added %d amber pixels (off %d)", onAmber, offAmber)
	}
	onCyan := countPixels(on, func(r, g, b uint8) bool { return b > 150 && g > 120 && r < 120 })
	if onCyan == 0 {
		t.Fatalf("expected CT route pixels")
	}
}

func TestRouteLayerSkipsFutureFrames(t *testing.T) {
	round := parser.ReplayRoundData{
		Frames: []parser.ReplayFrame{
			{Tick: 0, Players: []parser.ReplayPlayerPose{{Slot: 1, Team: 1, Alive: true, X: -800, Y: 0}}},
			{Tick: 64, Players: []parser.ReplayPlayerPose{{Slot: 1, Team: 1, Alive: true, X: 800, Y: 0}}},
		},
	}
	c := newTestCanvas()
	amber := func(r, g, b uint8) bool { return r > 100 && g > 60 && b < 60 }
	c.SetLayers(ReplayLayers{})
	base := countPixels(c.RenderReplayFrame(round, 0), amber)
	c.SetLayers(ReplayLayers{Route: true})
	early := countPixels(c.RenderReplayFrame(round, 0), amber)
	if early != base {
		t.Fatalf("trail drawn before movement: %d px vs base %d", early, base)
	}
}
