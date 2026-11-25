package components

import (
	"testing"

	"strikeye/internal/parser"
)

func TestRadarPinsDrawAtCalloutPosition(t *testing.T) {
	c := newTestCanvas()
	c.ConfigureMap("de_dust2", false, [4]float64{-2476, -1150, 3239, 1908})

	empty := c.RenderReplayFrame(parser.ReplayRoundData{}, -1)
	baseline := countPixels(empty, isPinPixel)

	c.SetPins([]Pin{{X: 900, Y: 1300, Label: "A Site", Color: "red"}})
	withPin := c.RenderReplayFrame(parser.ReplayRoundData{}, -1)
	after := countPixels(withPin, isPinPixel)

	if after <= baseline {
		t.Fatalf("pin not drawn: baseline=%d after=%d", baseline, after)
	}

	c.SetPins(nil)
	cleared := c.RenderReplayFrame(parser.ReplayRoundData{}, -1)
	if got := countPixels(cleared, isPinPixel); got != baseline {
		t.Fatalf("pins not cleared: %d vs baseline %d", got, baseline)
	}
}

// isPinPixel matches the pin colours (amber/red/green/blue) blended over the
// dark backdrop; the replay itself draws none of them with this saturation.
func isPinPixel(r, g, b uint8) bool {
	amber := r > 150 && g > 110 && b < 90
	red := r > 150 && g < 120 && b < 120
	green := g > 150 && r < 130
	blue := b > 150 && r < 140
	return amber || red || green || blue
}
