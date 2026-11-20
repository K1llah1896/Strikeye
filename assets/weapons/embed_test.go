package weapons

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAllWeaponAndKillfeedSVGsParseCleanly(t *testing.T) {
	keys, err := ListAllKeys()
	if err != nil {
		t.Fatalf("ListAllKeys: %v", err)
	}
	if len(keys) < 50 {
		t.Fatalf("expected >=50 embedded SVGs, got %d", len(keys))
	}
	for _, key := range keys {
		raw, ok := LoadSVG(key)
		if !ok {
			t.Fatalf("LoadSVG(%q) returned false", key)
		}
		svg, err := ui.ParseSVG(raw)
		if err != nil {
			t.Fatalf("ParseSVG(%q) failed: %v", key, err)
		}
		w, h := ui.ImageSize(svg)
		if w <= 0 || h <= 0 {
			t.Fatalf("SVG %q has invalid size %vx%v", key, w, h)
		}
	}

	// Verify demoinfocs human-readable weapon names and killfeed modifiers resolve cleanly.
	for _, sample := range []string{
		"AK-47", "M4A1-S", "M4A4", "AWP", "Desert Eagle", "USP-S", "Glock-18",
		"HE Grenade", "Molotov", "Incendiary Grenade", "Butterfly Knife",
		"headshot", "wallbang", "through_smoke", "noscope", "attacker_blind", "flashbang_assist",
	} {
		if _, ok := LoadSVG(sample); !ok {
			t.Fatalf("expected LoadSVG(%q) to resolve", sample)
		}
	}
}
