package components

import (
	"math"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"
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

func TestRadar2DCanvasNaNAndConcurrentBitmapSafety(t *testing.T) {
	canvas := NewRadar2DCanvas(256)
	canvas.ConfigureMap("de_mirage", false, [4]float64{-2000, 2000, -2000, 2000})

	// NaN / Inf coordinates must never hang or panic.
	nanRound := parser.ReplayRoundData{
		RoundNumber: 1,
		Frames: []parser.ReplayFrame{
			{
				Tick: 100,
				Players: []parser.ReplayPlayerPose{
					{Slot: 1, Team: 1, Alive: true, Health: 100, X: float32(math.NaN()), Y: float32(math.Inf(1)), Yaw: float32(math.NaN())},
					{Slot: 2, Team: 2, Alive: true, Health: 100, X: -400, Y: 200, Yaw: 90},
				},
			},
		},
		Smokes: []parser.SmokeEffect{
			{StartTick: 50, EndTick: 150, Pos: parser.Point3D{X: float32(math.NaN()), Y: 0}},
		},
	}
	if img := canvas.RenderReplayFrame(nanRound, 0); img == nil {
		t.Fatalf("expected non-nil image for NaN-guarded frame")
	}

	pts := make([]HeatmapPoint, 0, 20000)
	pts = append(pts, HeatmapPoint{X: math.NaN(), Y: math.Inf(-1), Weight: 1})
	for i := 0; i < 19999; i++ {
		pts = append(pts, HeatmapPoint{
			X:      float64(-1500 + (i % 3000)),
			Y:      float64(-1500 + ((i * 7) % 3000)),
			Weight: 1,
		})
	}

	// Concurrent background rasterization + main-thread RenderUI must be race-free.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 8; i++ {
			canvas.RenderReplayFrame(nanRound, 0)
			canvas.RenderHeatmap(pts)
		}
	}()

	tt := ui.NewTester(func(c *ui.Context) {
		canvas.RenderUI(c, 256)
	}, 300, 300)
	for i := 0; i < 8; i++ {
		tt.Frame()
	}
	wg.Wait()
}
