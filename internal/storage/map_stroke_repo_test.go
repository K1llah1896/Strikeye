package storage

import (
	"context"
	"testing"
)

func TestMapStrokesAddListDelete(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	repo := db.MapStrokes()

	stroke, err := repo.Add(ctx, MapStroke{DemoID: 7, RoundNumber: 3, Points: []StrokePoint{
		{X: 0.1, Y: 0.2}, {X: 0.15, Y: 0.25}, {X: -1, Y: 2},
	}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if stroke.ID == 0 || stroke.Color != "amber" {
		t.Fatalf("stroke = %+v", stroke)
	}
	// Coordinates are clamped into the unit square.
	if stroke.Points[2].X != 0 || stroke.Points[2].Y != 1 {
		t.Fatalf("clamped point = %+v", stroke.Points[2])
	}

	if _, err := repo.Add(ctx, MapStroke{DemoID: 7, RoundNumber: 3, Color: "red", Points: []StrokePoint{{X: 0, Y: 0}, {X: 0.5, Y: 0.5}}}); err != nil {
		t.Fatalf("add red: %v", err)
	}
	if _, err := repo.Add(ctx, MapStroke{DemoID: 7, Points: []StrokePoint{{X: 0, Y: 0}}}); err == nil {
		t.Fatal("expected an error for a single-point stroke")
	}
	if _, err := repo.Add(ctx, MapStroke{Points: []StrokePoint{{X: 0, Y: 0}, {X: 1, Y: 1}}}); err == nil {
		t.Fatal("expected an error without a demo id")
	}

	rows, err := repo.List(ctx, 7)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 || rows[1].Color != "red" || len(rows[1].Points) != 2 {
		t.Fatalf("rows = %+v", rows)
	}

	if err := repo.Delete(ctx, rows[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rows, _ = repo.List(ctx, 7)
	if len(rows) != 1 {
		t.Fatalf("expected one stroke left, got %d", len(rows))
	}
	removed, err := repo.DeleteByRound(ctx, 7, 3)
	if err != nil || removed != 1 {
		t.Fatalf("delete by round = %d, %v", removed, err)
	}
	if rows, _ = repo.List(ctx, 7); len(rows) != 0 {
		t.Fatalf("expected no strokes, got %d", len(rows))
	}
}

func TestMapStrokesCorruptRowDoesNotFailList(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.SQL().ExecContext(ctx, `
		INSERT INTO map_strokes (demo_id, round_number, color, points_json, created_at)
		VALUES (3, 0, 'amber', 'not json', '2026-11-25T00:00:00Z')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	rows, err := db.MapStrokes().List(ctx, 3)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Points != nil {
		t.Fatalf("rows = %+v", rows)
	}
}
