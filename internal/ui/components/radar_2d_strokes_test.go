package components

import (
	"image"
	"image/draw"
	"testing"
)

func toRGBA(t *testing.T, img image.Image) *image.RGBA {
	t.Helper()
	b := img.Bounds()
	out := image.NewRGBA(b)
	draw.Draw(out, b, img, b.Min, draw.Src)
	return out
}

func TestStrokesChangeCanvasPixels(t *testing.T) {
	c := NewRadar2DCanvas(160)
	c.ConfigureMap("de_dust2", false, [4]float64{-2500, 2500, -2500, 2500})

	base := c.RenderReplayFrame(combatTestRound(), 0)
	baseRGBA := toRGBA(t, base)

	c.SetStrokes([]Stroke{{Color: "amber", Points: []StrokePoint{{X: 0.1, Y: 0.1}, {X: 0.5, Y: 0.5}, {X: 0.9, Y: 0.9}}}})
	withStroke := c.RenderReplayFrame(combatTestRound(), 0)
	strokeRGBA := toRGBA(t, withStroke)

	diff := 0
	for i := 0; i < len(baseRGBA.Pix); i += 4 {
		if baseRGBA.Pix[i] != strokeRGBA.Pix[i] || baseRGBA.Pix[i+1] != strokeRGBA.Pix[i+1] || baseRGBA.Pix[i+2] != strokeRGBA.Pix[i+2] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("strokes did not change any pixel")
	}

	// Single point strokes are ignored rather than drawn as a dot.
	c.SetStrokes([]Stroke{{Color: "red", Points: []StrokePoint{{X: 0.5, Y: 0.5}}}})
	single := toRGBA(t, c.RenderReplayFrame(combatTestRound(), 0))
	same := true
	for i := 0; i < len(baseRGBA.Pix); i += 4 {
		if baseRGBA.Pix[i] != single.Pix[i] || baseRGBA.Pix[i+1] != single.Pix[i+1] || baseRGBA.Pix[i+2] != single.Pix[i+2] {
			same = false
			break
		}
	}
	if !same {
		t.Fatal("a single-point stroke should not draw anything")
	}
}

func TestBrushStateAndClamp(t *testing.T) {
	c := NewRadar2DCanvas(64)
	called := 0
	c.SetBrush(true, func(x, y float32, pressed bool) { called++ })
	c.mu.Lock()
	active := c.brush
	handler := c.onBrush != nil
	c.mu.Unlock()
	if !active || !handler {
		t.Fatal("brush state was not stored")
	}
	c.SetBrush(false, nil)
	c.mu.Lock()
	active = c.brush
	c.mu.Unlock()
	if active {
		t.Fatal("brush should be disabled")
	}
	if clampNorm(-0.5) != 0 || clampNorm(1.5) != 1 || clampNorm(0.25) != 0.25 {
		t.Fatal("clampNorm is wrong")
	}
	_ = called
}
