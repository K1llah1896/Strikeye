package analysis

import (
	"context"
	"testing"
	"time"

	"strikeye/internal/profile"
	"strikeye/internal/storage"
	"strikeye/internal/ui/components"
)

func TestStrokesForRoundIncludesMatchWideRows(t *testing.T) {
	rows := []storage.MapStroke{
		{RoundNumber: 0, Color: "amber", Points: []storage.StrokePoint{{X: 0.1, Y: 0.1}, {X: 0.2, Y: 0.2}}},
		{RoundNumber: 3, Color: "red", Points: []storage.StrokePoint{{X: 0.3, Y: 0.3}, {X: 0.4, Y: 0.4}}},
		{RoundNumber: 5, Color: "green", Points: []storage.StrokePoint{{X: 0.5, Y: 0.5}, {X: 0.6, Y: 0.6}}},
		{RoundNumber: 3, Color: "blue", Points: []storage.StrokePoint{{X: 0.7, Y: 0.7}}}, // too short
	}
	got := strokesForRound(rows, 3)
	if len(got) != 2 {
		t.Fatalf("expected match-wide + round 3 strokes, got %d", len(got))
	}
	if got[0].Color != "amber" || got[1].Color != "red" {
		t.Fatalf("unexpected strokes: %+v", got)
	}
	if len(got[1].Points) != 2 || got[1].Points[1].X != 0.4 {
		t.Fatalf("points = %+v", got[1].Points)
	}
}

func TestHandleBrushPointPersistsOnRelease(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/a.dem", Filename: "a.dem", ContentSHA256: "a",
		TeamAName: "A", TeamBName: "B", ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}

	v := &View{
		db: db, demo: &demo, zoneRound: 2, brushActive: true,
		radarCanvas: components.NewRadar2DCanvas(64),
		trends:      map[string][]profile.TrendPoint{},
	}

	// Pressing appends points, ignoring ones closer than the minimum step.
	v.handleBrushPoint(0.1, 0.1, true)
	v.handleBrushPoint(0.1005, 0.1005, true)
	v.handleBrushPoint(0.5, 0.5, true)
	v.handleBrushPoint(0.9, 0.9, true)
	v.mu.RLock()
	n := len(v.brushPts)
	v.mu.RUnlock()
	if n != 3 {
		t.Fatalf("expected 3 sampled points, got %d", n)
	}

	// Releasing persists the stroke.
	v.handleBrushPoint(0.9, 0.9, false)
	deadline := time.Now().Add(2 * time.Second)
	for {
		rows, err := db.MapStrokes().List(ctx, demo.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(rows) == 1 {
			if rows[0].RoundNumber != 2 || rows[0].Color != brushColor || len(rows[0].Points) != 3 {
				t.Fatalf("stored stroke = %+v", rows[0])
			}
			if rows[0].Points[0].X != 0.1 || rows[0].Points[2].X != 0.9 {
				t.Fatalf("stored points = %+v", rows[0].Points)
			}
			v.mu.RLock()
			cached := len(v.zoneStrokes)
			pending := len(v.brushPts)
			v.mu.RUnlock()
			if cached != 1 || pending != 0 {
				t.Fatalf("view cache = %d strokes, %d pending", cached, pending)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("stroke was not persisted")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestToggleBrushDiscardsPendingStroke(t *testing.T) {
	v := &View{brushActive: false, brushPts: []components.StrokePoint{{X: 0.2, Y: 0.2}}}
	v.toggleBrush()
	v.mu.RLock()
	active, pending := v.brushActive, len(v.brushPts)
	v.mu.RUnlock()
	if !active || pending != 0 {
		t.Fatalf("toggle = active:%v pending:%d", active, pending)
	}
	v.toggleBrush()
	v.mu.RLock()
	active = v.brushActive
	v.mu.RUnlock()
	if active {
		t.Fatal("second toggle should disable the brush")
	}
}
