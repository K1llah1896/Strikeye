package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"strikeye/internal/storage"
)

func TestRunViewPrefsSetAndList(t *testing.T) {
	dataRoot := t.TempDir()
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	var buf bytes.Buffer
	if err := runViewPrefs(ctx, db, viewPrefsOptions{}, &buf); err != nil {
		t.Fatalf("list empty: %v", err)
	}
	var listed struct {
		Count int                    `json:"count"`
		Prefs []storage.MapViewPrefs `json:"prefs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &listed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if listed.Count != 0 {
		t.Fatalf("expected empty list, got %+v", listed)
	}

	buf.Reset()
	if err := runViewPrefs(ctx, db, viewPrefsOptions{mapName: "de_ancient", offsetX: 12, offsetY: -6, opacity: 0.4, set: true}, &buf); err != nil {
		t.Fatalf("set: %v", err)
	}
	var saved struct {
		Saved bool                 `json:"saved"`
		Prefs storage.MapViewPrefs `json:"prefs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &saved); err != nil {
		t.Fatalf("decode saved: %v", err)
	}
	if !saved.Saved || saved.Prefs.MapName != "de_ancient" || saved.Prefs.LowerOffsetX != 12 {
		t.Fatalf("unexpected save result: %+v", saved)
	}

	buf.Reset()
	if err := runViewPrefs(ctx, db, viewPrefsOptions{}, &buf); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := json.Unmarshal(buf.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Count != 1 || listed.Prefs[0].LowerOpacity != 0.4 {
		t.Fatalf("list after set: %+v", listed)
	}

	if err := runViewPrefs(ctx, db, viewPrefsOptions{set: true}, &buf); err == nil {
		t.Fatal("expected error when --set lacks a map")
	}
	if err := runViewPrefs(ctx, nil, viewPrefsOptions{}, &buf); err == nil {
		t.Fatal("expected error without storage")
	}
}
