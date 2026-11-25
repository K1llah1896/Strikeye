package storage

import (
	"context"
	"testing"
)

func TestMapViewPrefsDefaultsAndUpsert(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDefault(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()

	prefs, err := db.MapViewPrefs().Get(ctx, "de_dust2")
	if err != nil {
		t.Fatalf("get default: %v", err)
	}
	if prefs.MapName != "de_dust2" || prefs.LowerOpacity != 1 || prefs.LowerOffsetX != 0 {
		t.Fatalf("unexpected defaults: %+v", prefs)
	}

	if err := db.MapViewPrefs().Upsert(ctx, MapViewPrefs{
		MapName: "de_dust2", LowerOffsetX: 24, LowerOffsetY: -8, LowerOpacity: 0.45,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := db.MapViewPrefs().Get(ctx, "de_dust2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.LowerOffsetX != 24 || got.LowerOffsetY != -8 || got.LowerOpacity != 0.45 || got.UpdatedAt == "" {
		t.Fatalf("stored values wrong: %+v", got)
	}

	// Out-of-range input is clamped rather than stored verbatim.
	if err := db.MapViewPrefs().Upsert(ctx, MapViewPrefs{
		MapName: "de_mirage", LowerOffsetX: 9999, LowerOffsetY: -9999, LowerOpacity: 7,
	}); err != nil {
		t.Fatalf("upsert clamped: %v", err)
	}
	mirage, _ := db.MapViewPrefs().Get(ctx, "de_mirage")
	if mirage.LowerOffsetX != maxViewOffset || mirage.LowerOffsetY != -maxViewOffset || mirage.LowerOpacity != 1 {
		t.Fatalf("clamp failed: %+v", mirage)
	}
	if err := db.MapViewPrefs().Upsert(ctx, MapViewPrefs{MapName: "de_nuke", LowerOpacity: 0.01}); err != nil {
		t.Fatalf("upsert min opacity: %v", err)
	}
	nuke, _ := db.MapViewPrefs().Get(ctx, "de_nuke")
	if nuke.LowerOpacity != minViewOpacity {
		t.Fatalf("min opacity clamp failed: %+v", nuke)
	}
	if err := db.MapViewPrefs().Upsert(ctx, MapViewPrefs{}); err == nil {
		t.Fatal("expected error for empty map name")
	}

	list, err := db.MapViewPrefs().List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 || list[0].MapName != "de_dust2" || list[1].MapName != "de_mirage" || list[2].MapName != "de_nuke" {
		t.Fatalf("list ordering wrong: %+v", list)
	}
	db.Close()

	// Re-opening must keep the stored alignment.
	reopened, err := OpenDefault(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	again, err := reopened.MapViewPrefs().Get(ctx, "de_dust2")
	if err != nil || again.LowerOffsetX != 24 || again.LowerOpacity != 0.45 {
		t.Fatalf("persistence lost: %+v %v", again, err)
	}
}
