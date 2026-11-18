package components

import "testing"

func TestVideoPlayerMathAndTrimBounds(t *testing.T) {
	if got := ComputeEventSeekTime(12.5, 2.0); got != 10.5 {
		t.Fatalf("expected 10.5, got %v", got)
	}
	if got := ComputeEventSeekTime(1.2, 2.0); got != 0 {
		t.Fatalf("expected clamped 0, got %v", got)
	}
	if tick := TimeToTick(1000, 2.5, 64.0); tick != 1160 {
		t.Fatalf("expected tick 1160, got %d", tick)
	}

	w := NewVideoPlayerWidget("ffmpeg")
	w.startTick = 640
	w.tickRate = 64
	w.inSec = 1.0
	w.outSec = 4.0
	inT, outT := w.InOutTicks()
	if inT != 704 || outT != 896 {
		t.Fatalf("unexpected trim ticks: in=%d out=%d", inT, outT)
	}
}
