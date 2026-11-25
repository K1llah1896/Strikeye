package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MapAnnotation is a user-placed tactical marker pinned to a callout of one
// demo (round 0 = the whole match).
type MapAnnotation struct {
	ID          int64  `json:"id"`
	DemoID      int64  `json:"demo_id"`
	RoundNumber int    `json:"round_number"`
	CalloutID   string `json:"callout_id"`
	Note        string `json:"note"`
	Color       string `json:"color"`
	CreatedAt   string `json:"created_at"`
}

// MapAnnotationsRepo stores tactical annotations.
type MapAnnotationsRepo struct{ db *DB }

// MapAnnotations returns the annotation repository.
func (db *DB) MapAnnotations() *MapAnnotationsRepo { return &MapAnnotationsRepo{db: db} }

// Add upserts an annotation keyed by (demo, round, callout) and returns the
// stored row.
func (r *MapAnnotationsRepo) Add(ctx context.Context, a MapAnnotation) (MapAnnotation, error) {
	if strings.TrimSpace(a.CalloutID) == "" {
		return MapAnnotation{}, fmt.Errorf("annotation callout is required")
	}
	if a.Color = normalizeAnnotationColor(a.Color); a.Color == "" {
		a.Color = "amber"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO map_annotations (demo_id, round_number, callout_id, note, color, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(demo_id, round_number, callout_id) DO UPDATE SET
			note = excluded.note,
			color = excluded.color`,
		a.DemoID, a.RoundNumber, a.CalloutID, strings.TrimSpace(a.Note), a.Color, now); err != nil {
		return MapAnnotation{}, err
	}
	return r.Get(ctx, a.DemoID, a.RoundNumber, a.CalloutID)
}

// Get returns one annotation by its key.
func (r *MapAnnotationsRepo) Get(ctx context.Context, demoID int64, round int, calloutID string) (MapAnnotation, error) {
	row := r.db.sql.QueryRowContext(ctx, `
		SELECT id, demo_id, round_number, callout_id, note, color, created_at
		FROM map_annotations WHERE demo_id = ? AND round_number = ? AND callout_id = ?`,
		demoID, round, calloutID)
	var a MapAnnotation
	if err := row.Scan(&a.ID, &a.DemoID, &a.RoundNumber, &a.CalloutID, &a.Note, &a.Color, &a.CreatedAt); err != nil {
		return MapAnnotation{}, err
	}
	return a, nil
}

// ListByDemo returns every annotation of a demo, oldest first.
func (r *MapAnnotationsRepo) ListByDemo(ctx context.Context, demoID int64) ([]MapAnnotation, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT id, demo_id, round_number, callout_id, note, color, created_at
		FROM map_annotations WHERE demo_id = ? ORDER BY id`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MapAnnotation
	for rows.Next() {
		var a MapAnnotation
		if err := rows.Scan(&a.ID, &a.DemoID, &a.RoundNumber, &a.CalloutID, &a.Note, &a.Color, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteByKey removes an annotation by its key.
func (r *MapAnnotationsRepo) DeleteByKey(ctx context.Context, demoID int64, round int, calloutID string) (bool, error) {
	res, err := r.db.sql.ExecContext(ctx, `
		DELETE FROM map_annotations WHERE demo_id = ? AND round_number = ? AND callout_id = ?`,
		demoID, round, calloutID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Delete removes an annotation by id.
func (r *MapAnnotationsRepo) Delete(ctx context.Context, id int64) (bool, error) {
	res, err := r.db.sql.ExecContext(ctx, `DELETE FROM map_annotations WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func normalizeAnnotationColor(c string) string {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "amber", "yellow":
		return "amber"
	case "red", "danger":
		return "red"
	case "green", "ok":
		return "green"
	case "blue", "info":
		return "blue"
	case "":
		return ""
	}
	return ""
}
