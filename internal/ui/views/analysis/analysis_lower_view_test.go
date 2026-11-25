package analysis

import (
	"context"
	"testing"

	"strikeye/internal/storage"
	"strikeye/internal/ui/components"
)

func TestLowerViewPrefsRoundTripThroughStorage(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.MapViewPrefs().Upsert(context.Background(), storage.MapViewPrefs{
		MapName: "de_dust2", LowerOffsetX: 32, LowerOffsetY: -12, LowerOpacity: 0.6,
	}); err != nil {
		t.Fatalf("seed prefs: %v", err)
	}

	v := NewView(db, t.TempDir(), "")
	v.radarCanvas = components.NewRadar2DCanvas(64)
	v.heatmapCanvas = components.NewRadar2DCanvas(64)
	v.loadLowerViewPrefs("de_dust2")

	v.mu.RLock()
	got := [3]float64{v.lowerOffsetX, v.lowerOffsetY, v.lowerOpacity}
	v.mu.RUnlock()
	if got != [3]float64{32, -12, 0.6} {
		t.Fatalf("loaded %v", got)
	}

	// Saving a new alignment must persist and be reloadable.
	v.mu.Lock()
	v.lowerOffsetX, v.lowerOffsetY, v.lowerOpacity = -40, 24, 0.3
	v.mu.Unlock()
	v.saveLowerViewPrefs("de_dust2")

	prefs, err := db.MapViewPrefs().Get(context.Background(), "de_dust2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if prefs.LowerOffsetX != -40 || prefs.LowerOffsetY != 24 || prefs.LowerOpacity != 0.3 {
		t.Fatalf("persisted %+v", prefs)
	}

	// A map without stored prefs resets to centred and opaque.
	v.loadLowerViewPrefs("de_mirage")
	v.mu.RLock()
	got = [3]float64{v.lowerOffsetX, v.lowerOffsetY, v.lowerOpacity}
	v.mu.RUnlock()
	if got != [3]float64{0, 0, 1} {
		t.Fatalf("defaults after empty map: %v", got)
	}
}

func TestSaveLowerViewPrefsWithoutDBIsNoop(t *testing.T) {
	v := NewView(nil, "", "")
	v.saveLowerViewPrefs("de_dust2")
	v.loadLowerViewPrefs("de_dust2")
}
