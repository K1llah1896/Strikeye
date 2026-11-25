package storage

import "context"

// MatchPurchase is one item bought in a round's buy window.
type MatchPurchase struct {
	RoundNumber int     `json:"round_number"`
	Tick        int64   `json:"tick"`
	TimeSec     float64 `json:"time_sec"`
	SteamID64   string  `json:"steam_id64"`
	PlayerName  string  `json:"player_name"`
	Weapon      string  `json:"weapon"`
	Cost        int     `json:"cost"`
}

// ReplaceMatchPurchases swaps the purchase list of one demo.
func (r *AnalysisRepository) ReplaceMatchPurchases(ctx context.Context, demoID int64, purchases []MatchPurchase) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM match_purchases WHERE demo_id = ?`, demoID); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO match_purchases (demo_id, round_number, tick, time_sec, steam_id64, player_name, weapon, cost)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, p := range purchases {
		if _, err := stmt.ExecContext(ctx, demoID, p.RoundNumber, p.Tick, p.TimeSec, p.SteamID64, p.PlayerName, p.Weapon, p.Cost); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListMatchPurchases returns purchases ordered by round then tick.
func (r *AnalysisRepository) ListMatchPurchases(ctx context.Context, demoID int64) ([]MatchPurchase, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT round_number, tick, time_sec, steam_id64, player_name, weapon, cost
		FROM match_purchases WHERE demo_id = ? ORDER BY round_number, tick, id`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchPurchase
	for rows.Next() {
		var p MatchPurchase
		if err := rows.Scan(&p.RoundNumber, &p.Tick, &p.TimeSec, &p.SteamID64, &p.PlayerName, &p.Weapon, &p.Cost); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PurchaseTotals sums spend per player for one demo.
func (r *AnalysisRepository) PurchaseTotals(ctx context.Context, demoID int64) (map[string]int, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT steam_id64, SUM(cost) FROM match_purchases WHERE demo_id = ? GROUP BY steam_id64`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var sid string
		var sum int
		if err := rows.Scan(&sid, &sum); err != nil {
			return nil, err
		}
		out[sid] = sum
	}
	return out, rows.Err()
}
