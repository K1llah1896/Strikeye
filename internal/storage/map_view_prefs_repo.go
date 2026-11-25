package storage

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"
)

// MapViewPrefs holds per-map 2D viewport adjustments. Lower-layer offset is in
// radar pixels and lets hand-tuned maps line their level up with the upper one;
// opacity keeps the underlay readable instead of hiding the current floor.
type MapViewPrefs struct {
	MapName      string  `json:"map_name"`
	LowerOffsetX float64 `json:"lower_offset_x"`
	LowerOffsetY float64 `json:"lower_offset_y"`
	LowerOpacity float64 `json:"lower_opacity"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
}

const (
	maxViewOffset    = 512.0
	minViewOpacity   = 0.1
	maxViewOpacity   = 1.0
	defaultViewAplha = 1.0
)

// clampViewPrefs keeps stored values inside the ranges the UI can represent, so
// a corrupted row cannot push the overlay off-canvas.
func clampViewPrefs(p MapViewPrefs) MapViewPrefs {
	p.LowerOffsetX = clampFloat(p.LowerOffsetX, -maxViewOffset, maxViewOffset)
	p.LowerOffsetY = clampFloat(p.LowerOffsetY, -maxViewOffset, maxViewOffset)
	if math.IsNaN(p.LowerOpacity) || p.LowerOpacity <= 0 {
		p.LowerOpacity = defaultViewAplha
	}
	p.LowerOpacity = clampFloat(p.LowerOpacity, minViewOpacity, maxViewOpacity)
	return p
}

func clampFloat(v, lo, hi float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// MapViewPrefsRepo persists per-map viewport adjustments.
type MapViewPrefsRepo struct{ db *DB }

// MapViewPrefs returns the repository.
func (db *DB) MapViewPrefs() *MapViewPrefsRepo { return &MapViewPrefsRepo{db: db} }

// Get loads preferences for one map, returning defaults when none are stored.
func (r *MapViewPrefsRepo) Get(ctx context.Context, mapName string) (MapViewPrefs, error) {
	prefs := MapViewPrefs{MapName: mapName, LowerOpacity: defaultViewAplha}
	if mapName == "" {
		return prefs, nil
	}
	row := r.db.sql.QueryRowContext(ctx,
		`SELECT lower_offset_x, lower_offset_y, lower_opacity, updated_at FROM map_view_prefs WHERE map_name = ?`, mapName)
	err := row.Scan(&prefs.LowerOffsetX, &prefs.LowerOffsetY, &prefs.LowerOpacity, &prefs.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return prefs, nil
	}
	if err != nil {
		return prefs, err
	}
	return clampViewPrefs(prefs), nil
}

// Upsert stores clamped preferences for one map.
func (r *MapViewPrefsRepo) Upsert(ctx context.Context, prefs MapViewPrefs) error {
	if prefs.MapName == "" {
		return errors.New("map name is required")
	}
	prefs = clampViewPrefs(prefs)
	_, err := r.db.sql.ExecContext(ctx,
		`INSERT INTO map_view_prefs (map_name, lower_offset_x, lower_offset_y, lower_opacity, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(map_name) DO UPDATE SET
		   lower_offset_x = excluded.lower_offset_x,
		   lower_offset_y = excluded.lower_offset_y,
		   lower_opacity = excluded.lower_opacity,
		   updated_at = excluded.updated_at`,
		prefs.MapName, prefs.LowerOffsetX, prefs.LowerOffsetY, prefs.LowerOpacity,
		time.Now().UTC().Format(time.RFC3339))
	return err
}

// List returns every stored preference ordered by map name.
func (r *MapViewPrefsRepo) List(ctx context.Context) ([]MapViewPrefs, error) {
	rows, err := r.db.sql.QueryContext(ctx,
		`SELECT map_name, lower_offset_x, lower_offset_y, lower_opacity, updated_at FROM map_view_prefs ORDER BY map_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MapViewPrefs
	for rows.Next() {
		var p MapViewPrefs
		if err := rows.Scan(&p.MapName, &p.LowerOffsetX, &p.LowerOffsetY, &p.LowerOpacity, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, clampViewPrefs(p))
	}
	return out, rows.Err()
}
