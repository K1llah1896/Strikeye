package components

import (
	"testing"

	"strikeye/internal/parser"
)

func TestResolveButtonsAtTick(t *testing.T) {
	edges := []parser.InputEdge{
		{Tick: 100, PlayerSlot: 2, Buttons: parser.KeyW},
		{Tick: 150, PlayerSlot: 2, Buttons: parser.KeyW | parser.KeyFire},
		{Tick: 180, PlayerSlot: 3, Buttons: parser.KeyJump},
		{Tick: 220, PlayerSlot: 2, Buttons: parser.KeyCrouch},
	}

	if _, ok := ResolveButtonsAtTick(edges, 2, 160, 0); ok {
		t.Fatalf("expected ok=false when inputCap == 0")
	}

	mask, ok := ResolveButtonsAtTick(edges, 2, 160, 2)
	if !ok {
		t.Fatalf("expected ok=true for inputCap=2")
	}
	if mask != (parser.KeyW | parser.KeyFire) {
		t.Fatalf("expected KeyW|KeyFire at tick 160, got %#x", mask)
	}

	mask2, _ := ResolveButtonsAtTick(edges, 2, 220, 1)
	if mask2 != parser.KeyCrouch {
		t.Fatalf("expected KeyCrouch at tick 220, got %#x", mask2)
	}
}
