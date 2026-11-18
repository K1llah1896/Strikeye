package components

import (
	"testing"

	"strikeye/assets/maps"
	"strikeye/internal/parser"
)

func TestRadar2DCanvasReplayAndKDE(t *testing.T) {
	canvas := NewRadar2DCanvas(256)
	canvas.ConfigureMap("de_mirage", false, [4]float64{-2000, 2000, -2000, 2000})

	round := parser.ReplayRoundData{
		RoundNumber: 1,
		StartTick:   100,
		EndTick:     200,
		Frames: []parser.ReplayFrame{
			{
				Tick: 120,
				Players: []parser.ReplayPlayerPose{
					{Slot: 1, Team: 1, Alive: true, HasBomb: true, Health: 85, FlashDur: 0.5, X: -500, Y: -400, Yaw: 45},
					{Slot: 6, Team: 2, Alive: true, Health: 100, X: 200, Y: 300, Yaw: 180},
				},
			},
		},
		Smokes: []parser.SmokeEffect{
			{StartTick: 110, EndTick: 190, Pos: parser.Point3D{X: -200, Y: 100}},
		},
		Infernos: []parser.InfernoEffect{
			{StartTick: 110, EndTick: 190, Hull: []parser.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 50, Y: 100}}},
		},
	}

	img := canvas.RenderReplayFrame(round, 0)
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("unexpected rendered bounds: %v", img.Bounds())
	}

	meta, _ := maps.LookupMap("de_mirage")
	grid := ComputeKDEGrid(meta, []HeatmapPoint{{X: -500, Y: -400, Weight: 1}}, 32, 2.0)
	maxVal := 0.0
	for _, row := range grid {
		for _, v := range row {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal < 0.99 || maxVal > 1.01 {
		t.Fatalf("expected normalized KDE peak ~1.0, got %v", maxVal)
	}

	hmImg := canvas.RenderHeatmap([]HeatmapPoint{{X: -500, Y: -400, Weight: 1}})
	if hmImg.Bounds().Dx() != 256 {
		t.Fatalf("unexpected heatmap bounds: %v", hmImg.Bounds())
	}
}
