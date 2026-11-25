// Package renderutil provides a headless renderer for view smoke tests: mygo's
// ui.Render builds a view into an image without opening a window, which lets the
// test suite catch nil-pointer and layout panics in UI code.
package renderutil

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Probe renders a view at 1280x800 and returns the image plus the number of
// distinct sampled colours. Views always paint a background/gradient, so a very
// small count means nothing rendered.
func Probe(t *testing.T, view func(c *ui.Context)) (*image.RGBA, int) {
	t.Helper()
	img := ui.Render(view, 1280, 800, 1)
	if img == nil {
		t.Fatal("ui.Render returned nil")
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		t.Fatalf("render bounds %v", b)
	}
	seen := make(map[uint32]struct{}, 64)
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := b.Min.X; x < b.Max.X; x += 4 {
			r, g, bl, _ := img.At(x, y).RGBA()
			seen[uint32(r)<<16|uint32(g)<<8|uint32(bl)] = struct{}{}
		}
	}
	return img, len(seen)
}

// AssertRendered renders a view and fails when the frame looks blank.
func AssertRendered(t *testing.T, minColors int, view func(c *ui.Context)) int {
	t.Helper()
	_, colors := Probe(t, view)
	if colors < minColors {
		t.Fatalf("render looks blank: %d distinct colours (want >= %d)", colors, minColors)
	}
	return colors
}
