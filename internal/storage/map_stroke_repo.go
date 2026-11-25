package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// StrokePoint is one sampled point of a freehand annotation, normalised to the
// [0,1] radar image space so it survives window resizing.
type StrokePoint struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

// MapStroke is a freehand annotation drawn over the 2D replay radar.
type MapStroke struct {
	ID          int64         `json:"id"`
	DemoID      int64         `json:"demo_id"`
	RoundNumber int           `json:"round_number"`
	Color       string        `json:"color"`
	Points      []StrokePoint `json:"points"`
	CreatedAt   string        `json:"created_at"`
}

// MapStrokesRepo stores freehand drawings.
type MapStrokesRepo struct{ db *DB }

// MapStrokes returns the freehand stroke repository.
func (db *DB) MapStrokes() *MapStrokesRepo { return &MapStrokesRepo{db: db} }

// Add stores a stroke and returns the row with its new ID.
func (r *MapStrokesRepo) Add(ctx context.Context, s MapStroke) (MapStroke, error) {
	if s.DemoID <= 0 {
		return MapStroke{}, fmt.Errorf("stroke demo id is required")
	}
	if len(s.Points) < 2 {
		return MapStroke{}, fmt.Errorf("stroke needs at least two points")
	}
	points := make([]StrokePoint, 0, len(s.Points))
	for _, p := range s.Points {
		points = append(points, StrokePoint{X: clampUnit(p.X), Y: clampUnit(p.Y)})
	}
	color := normalizeAnnotationColor(s.Color)
	if color == "" {
		color = "amber"
	}
	blob, err := json.Marshal(points)
	if err != nil {
		return MapStroke{}, fmt.Errorf("encode stroke: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO map_strokes (demo_id, round_number, color, points_json, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		s.DemoID, s.RoundNumber, color, string(blob), now)
	if err != nil {
		return MapStroke{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return MapStroke{}, err
	}
	return MapStroke{ID: id, DemoID: s.DemoID, RoundNumber: s.RoundNumber, Color: color, Points: points, CreatedAt: now}, nil
}

// List returns every stroke of a demo (all rounds) ordered by creation.
func (r *MapStrokesRepo) List(ctx context.Context, demoID int64) ([]MapStroke, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT id, demo_id, round_number, color, points_json, created_at
		FROM map_strokes WHERE demo_id = ? ORDER BY id ASC`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MapStroke
	for rows.Next() {
		var s MapStroke
		var blob string
		if err := rows.Scan(&s.ID, &s.DemoID, &s.RoundNumber, &s.Color, &blob, &s.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(blob), &s.Points); err != nil {
			// A corrupt row must not break the whole overlay.
			s.Points = nil
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Delete removes one stroke by ID.
func (r *MapStrokesRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.sql.ExecContext(ctx, `DELETE FROM map_strokes WHERE id = ?`, id)
	return err
}

// DeleteByRound removes every stroke of one round (round 0 = whole match).
func (r *MapStrokesRepo) DeleteByRound(ctx context.Context, demoID int64, round int) (int64, error) {
	res, err := r.db.sql.ExecContext(ctx, `DELETE FROM map_strokes WHERE demo_id = ? AND round_number = ?`, demoID, round)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func clampUnit(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
