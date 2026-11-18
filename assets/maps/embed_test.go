package maps

import "testing"

func TestEmbeddedMapsAndProjection(t *testing.T) {
	meta, ok := LookupMap("de_mirage")
	if !ok {
		t.Fatalf("expected de_mirage in map-data.json")
	}
	if meta.Scale <= 0 {
		t.Fatalf("expected positive scale for de_mirage, got %v", meta.Scale)
	}
	pngData, ok := LoadRadarPNG("de_mirage", false)
	if !ok || len(pngData) == 0 {
		t.Fatalf("expected embedded de_mirage.png")
	}
	if !HasLowerLayer("de_nuke") {
		t.Fatalf("expected de_nuke lower layer to be bundled")
	}
	lowerPNG, ok := LoadRadarPNG("de_nuke", true)
	if !ok || len(lowerPNG) == 0 {
		t.Fatalf("expected embedded de_nuke_lower.png")
	}

	x, y := WorldToRadar(meta, meta.PosX+meta.Scale*512, meta.PosY-meta.Scale*512, 1024, 1024)
	if x < 511.9 || x > 512.1 || y < 511.9 || y > 512.1 {
		t.Fatalf("unexpected radar projection: (%v, %v)", x, y)
	}

	fb := DefaultFallbackMeta(-1000, 1000, -1000, 1000)
	fx, fy := WorldToRadar(fb, 0, 0, 512, 512)
	if fx < 255 || fx > 257 || fy < 255 || fy > 257 {
		t.Fatalf("unexpected fallback center projection: (%v, %v)", fx, fy)
	}
}
