package mapgeom

import (
	"bytes"
	"testing"
)

func TestIsOccludedThroughQuad(t *testing.T) {
	bvh, err := Read(bytes.NewReader(writeQuad(t)))
	if err != nil {
		t.Fatal(err)
	}
	tr := NewTracer(bvh)
	if !tr.IsOccluded([3]float64{0.5, 0.5, -1}, [3]float64{0.5, 0.5, 1}) {
		t.Fatal("expected segment through the quad to be occluded")
	}
	if tr.IsOccluded([3]float64{2, 2, -1}, [3]float64{2, 2, 1}) {
		t.Fatal("segment away from the quad must be clear")
	}
	if tr.IsOccluded([3]float64{0.5, 0.5, 1}, [3]float64{0.5, 0.5, 2}) {
		t.Fatal("segment above the quad must be clear")
	}
}

func TestIsOccludedHandlesDopplerTie(t *testing.T) {
	bvh, err := Read(bytes.NewReader(writeQuad(t)))
	if err != nil {
		t.Fatal(err)
	}
	tr := NewTracer(bvh)
	// Hits exactly at the start point: treated as clear because of the epsilon.
	if tr.IsOccluded([3]float64{0.5, 0.5, 0}, [3]float64{0.5, 0.5, 2}) {
		t.Fatal("intersection at the segment start should be ignored")
	}
}
