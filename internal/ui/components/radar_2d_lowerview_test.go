package components

import (
	"testing"

	"strikeye/internal/parser"
)

func emptyRound() parser.ReplayRoundData {
	return parser.ReplayRoundData{RoundNumber: 1, StartTick: 0, EndTick: 640}
}

func TestLowerViewOffsetAndOpacityAffectBackdrop(t *testing.T) {
	canvas := NewRadar2DCanvas(240)
	canvas.ConfigureMap("de_dust2", true, [4]float64{-2500, 2500, -2500, 2500})
	base := canvas.RenderReplayFrame(emptyRound(), -1)

	canvas.SetLowerView(24, -16, 1)
	shifted := canvas.RenderReplayFrame(emptyRound(), -1)
	if diff := pixelDiff(base, shifted); diff < 500 {
		t.Fatalf("offset did not shift the lower layer (diff %d)", diff)
	}

	canvas.SetLowerView(0, 0, 0.35)
	faded := canvas.RenderReplayFrame(emptyRound(), -1)
	if diff := pixelDiff(base, faded); diff < 5000 {
		t.Fatalf("opacity did not fade the lower layer (diff %d)", diff)
	}

	// Restoring defaults must reproduce the original backdrop byte for byte.
	canvas.SetLowerView(0, 0, 1)
	restored := canvas.RenderReplayFrame(emptyRound(), -1)
	if diff := pixelDiff(base, restored); diff != 0 {
		t.Fatalf("defaults not restored (diff %d)", diff)
	}
}

func TestLowerViewClampsInputs(t *testing.T) {
	canvas := NewRadar2DCanvas(120)
	canvas.SetLowerView(9999, -9999, 5)
	canvas.mu.Lock()
	x, y, op := canvas.lowerOffsetX, canvas.lowerOffsetY, canvas.lowerOpacity
	canvas.mu.Unlock()
	if x != 512 || y != -512 || op != 1 {
		t.Fatalf("clamp failed: %v %v %v", x, y, op)
	}
	canvas.SetLowerView(0, 0, 0)
	canvas.mu.Lock()
	op = canvas.lowerOpacity
	canvas.mu.Unlock()
	if op != 1 {
		t.Fatalf("zero opacity should fall back to 1, got %v", op)
	}
}
