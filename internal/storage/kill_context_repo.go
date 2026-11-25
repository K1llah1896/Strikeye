package storage

import (
	"context"
	"fmt"
)

// KillContext is one stored per-kill geometry annotation.
type KillContext struct {
	DemoID           int64   `json:"demo_id"`
	RoundNumber      int     `json:"round_number"`
	Tick             int64   `json:"tick"`
	TimeSec          float64 `json:"time_sec"`
	KillerSlot       int     `json:"killer_slot"`
	VictimSlot       int     `json:"victim_slot"`
	KillerName       string  `json:"killer_name"`
	VictimName       string  `json:"victim_name"`
	Weapon           string  `json:"weapon"`
	Headshot         bool    `json:"headshot"`
	Distance         float64 `json:"distance"`
	Visible          bool    `json:"visible"`
	GeometryChecked  bool    `json:"geometry_checked"`
	SmokeBlocked     bool    `json:"smoke_blocked"`
	OpticsChecked    bool    `json:"optics_checked"`
	AimOffDeg        float64 `json:"aim_off_deg"`
	InFOV            bool    `json:"in_fov"`
	VictimSeesKiller bool    `json:"victim_sees_killer"`
	AttackerBlind    bool    `json:"attacker_blind"`
	VictimBlind      bool    `json:"victim_blind"`
	AttackerZoom     int     `json:"attacker_zoom"`
	VictimZoom       int     `json:"victim_zoom"`
	KillerX          float64 `json:"killer_x"`
	KillerY          float64 `json:"killer_y"`
	KillerZ          float64 `json:"killer_z"`
	VictimX          float64 `json:"victim_x"`
	VictimY          float64 `json:"victim_y"`
	VictimZ          float64 `json:"victim_z"`
}

// KillContextRepo stores kill geometry rows.
type KillContextRepo struct{ db *DB }

// KillContext returns the stored kill context repository.
func (db *DB) KillContext() *KillContextRepo { return &KillContextRepo{db: db} }

// ReplaceMatchKillContext rewrites every stored row for one demo in a single
// transaction; an empty slice simply clears the demo.
func (r *KillContextRepo) ReplaceMatchKillContext(ctx context.Context, demoID int64, rows []KillContext) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM kill_context WHERE demo_id = ?`, demoID); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO kill_context (
			demo_id, round_number, tick, time_sec, killer_slot, victim_slot,
			killer_name, victim_name, weapon, headshot, distance, visible,
			geometry_checked, smoke_blocked, optics_checked, aim_off_deg, in_fov,
			victim_sees_killer, attacker_blind, victim_blind, attacker_zoom,
			victim_zoom, killer_x, killer_y, killer_z, victim_x, victim_y, victim_z
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, err := stmt.ExecContext(ctx,
			demoID, row.RoundNumber, row.Tick, row.TimeSec, row.KillerSlot, row.VictimSlot,
			row.KillerName, row.VictimName, row.Weapon, boolToInt(row.Headshot), row.Distance,
			boolToInt(row.Visible), boolToInt(row.GeometryChecked), boolToInt(row.SmokeBlocked),
			boolToInt(row.OpticsChecked), row.AimOffDeg, boolToInt(row.InFOV),
			boolToInt(row.VictimSeesKiller), boolToInt(row.AttackerBlind), boolToInt(row.VictimBlind),
			row.AttackerZoom, row.VictimZoom, row.KillerX, row.KillerY, row.KillerZ,
			row.VictimX, row.VictimY, row.VictimZ,
		); err != nil {
			return fmt.Errorf("insert kill context: %w", err)
		}
	}
	return tx.Commit()
}

// ListMatchKillContext returns the stored rows for one demo ordered by round and
// tick.
func (r *KillContextRepo) ListMatchKillContext(ctx context.Context, demoID int64) ([]KillContext, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT round_number, tick, time_sec, killer_slot, victim_slot, killer_name,
		       victim_name, weapon, headshot, distance, visible, geometry_checked,
		       smoke_blocked, optics_checked, aim_off_deg, in_fov, victim_sees_killer,
		       attacker_blind, victim_blind, attacker_zoom, victim_zoom,
		       killer_x, killer_y, killer_z, victim_x, victim_y, victim_z
		FROM kill_context WHERE demo_id = ?
		ORDER BY round_number ASC, tick ASC`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]KillContext, 0, 32)
	for rows.Next() {
		var k KillContext
		var headshot, visible, geometryChecked, smoke, optics, inFOV, victimSees, aBlind, vBlind int
		k.DemoID = demoID
		if err := rows.Scan(&k.RoundNumber, &k.Tick, &k.TimeSec, &k.KillerSlot, &k.VictimSlot,
			&k.KillerName, &k.VictimName, &k.Weapon, &headshot, &k.Distance, &visible,
			&geometryChecked, &smoke, &optics, &k.AimOffDeg, &inFOV, &victimSees,
			&aBlind, &vBlind, &k.AttackerZoom, &k.VictimZoom,
			&k.KillerX, &k.KillerY, &k.KillerZ, &k.VictimX, &k.VictimY, &k.VictimZ); err != nil {
			return nil, err
		}
		k.Headshot = headshot != 0
		k.Visible = visible != 0
		k.GeometryChecked = geometryChecked != 0
		k.SmokeBlocked = smoke != 0
		k.OpticsChecked = optics != 0
		k.InFOV = inFOV != 0
		k.VictimSeesKiller = victimSees != 0
		k.AttackerBlind = aBlind != 0
		k.VictimBlind = vBlind != 0
		out = append(out, k)
	}
	return out, rows.Err()
}

// KillContextTotals summarises the stored rows for one demo.
type KillContextTotals struct {
	Kills        int     `json:"kills"`
	BlindSpotPct float64 `json:"blind_spot_pct"`
	SmokePct     float64 `json:"smoke_pct"`
	AvgDistance  float64 `json:"avg_distance"`
}

// Totals aggregates the stored rows without loading them into Go.
func (r *KillContextRepo) Totals(ctx context.Context, demoID int64) (KillContextTotals, error) {
	var out KillContextTotals
	var occluded, smoke int
	err := r.db.sql.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(AVG(distance), 0),
		       COALESCE(SUM(CASE WHEN geometry_checked = 1 AND visible = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN smoke_blocked = 1 THEN 1 ELSE 0 END), 0)
		FROM kill_context WHERE demo_id = ?`, demoID).
		Scan(&out.Kills, &out.AvgDistance, &occluded, &smoke)
	if err != nil {
		return out, err
	}
	if out.Kills > 0 {
		out.BlindSpotPct = float64(occluded) / float64(out.Kills) * 100
		out.SmokePct = float64(smoke) / float64(out.Kills) * 100
	}
	return out, nil
}
