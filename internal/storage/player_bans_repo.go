package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PlayerBans is the cached Steam ban record for one account. Steam only exposes
// these numbers through the Web API with an API key, so the value is a snapshot
// with a checked_at timestamp rather than live state.
type PlayerBans struct {
	SteamID64        string    `json:"steam_id64"`
	VACBanned        bool      `json:"vac_banned"`
	NumberOfVACBans  int       `json:"number_of_vac_bans"`
	GameBans         int       `json:"game_bans"`
	NumberOfGameBans int       `json:"number_of_game_bans"`
	CommunityBanned  bool      `json:"community_banned"`
	DaysSinceLastBan int       `json:"days_since_last_ban"`
	EconomyBan       string    `json:"economy_ban"`
	CheckedAt        time.Time `json:"checked_at"`
}

// PlayerBansRepo stores ban snapshots keyed by SteamID64.
type PlayerBansRepo struct {
	db *DB
}

// PlayerBans returns the ban repository.
func (db *DB) PlayerBans() *PlayerBansRepo { return &PlayerBansRepo{db: db} }

// Upsert writes or replaces a ban snapshot.
func (r *PlayerBansRepo) Upsert(ctx context.Context, b PlayerBans) error {
	if b.SteamID64 == "" {
		return fmt.Errorf("upsert bans: empty steam id")
	}
	checked := b.CheckedAt
	if checked.IsZero() {
		checked = time.Now().UTC()
	}
	_, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO player_bans (
			steam_id64, vac_banned, number_of_vac_bans, number_of_game_bans,
			community_banned, days_since_last_ban, economy_ban, checked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(steam_id64) DO UPDATE SET
			vac_banned = excluded.vac_banned,
			number_of_vac_bans = excluded.number_of_vac_bans,
			number_of_game_bans = excluded.number_of_game_bans,
			community_banned = excluded.community_banned,
			days_since_last_ban = excluded.days_since_last_ban,
			economy_ban = excluded.economy_ban,
			checked_at = excluded.checked_at`,
		b.SteamID64, boolToInt(b.VACBanned), b.NumberOfVACBans, b.NumberOfGameBans,
		boolToInt(b.CommunityBanned), b.DaysSinceLastBan, b.EconomyBan, formatTime(checked))
	if err != nil {
		return fmt.Errorf("upsert bans: %w", err)
	}
	return nil
}

// Get returns the snapshot for one account, if cached.
func (r *PlayerBansRepo) Get(ctx context.Context, steamID string) (PlayerBans, bool, error) {
	row := r.db.sql.QueryRowContext(ctx, `
		SELECT steam_id64, vac_banned, number_of_vac_bans, number_of_game_bans,
		       community_banned, days_since_last_ban, economy_ban, checked_at
		FROM player_bans WHERE steam_id64 = ?`, steamID)
	out, err := scanBans(row)
	if err == sql.ErrNoRows {
		return PlayerBans{}, false, nil
	}
	if err != nil {
		return PlayerBans{}, false, fmt.Errorf("get bans: %w", err)
	}
	return out, true, nil
}

// List returns every cached snapshot, newest first.
func (r *PlayerBansRepo) List(ctx context.Context) ([]PlayerBans, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT steam_id64, vac_banned, number_of_vac_bans, number_of_game_bans,
		       community_banned, days_since_last_ban, economy_ban, checked_at
		FROM player_bans ORDER BY checked_at DESC, steam_id64`)
	if err != nil {
		return nil, fmt.Errorf("list bans: %w", err)
	}
	defer rows.Close()
	var out []PlayerBans
	for rows.Next() {
		b, err := scanBans(rows)
		if err != nil {
			return nil, fmt.Errorf("scan bans: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BannedFor returns the subset of steamIDs that have any ban recorded. It is used
// by the match library to badge players without loading every snapshot.
func (r *PlayerBansRepo) BannedFor(ctx context.Context, steamIDs []string) (map[string]bool, error) {
	if len(steamIDs) == 0 {
		return map[string]bool{}, nil
	}
	placeholders := make([]byte, 0, len(steamIDs)*2)
	args := make([]any, 0, len(steamIDs))
	for i, id := range steamIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, id)
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT steam_id64 FROM player_bans
		WHERE steam_id64 IN (`+string(placeholders)+`)
		  AND (vac_banned = 1 OR community_banned = 1 OR number_of_game_bans > 0)`, args...)
	if err != nil {
		return nil, fmt.Errorf("banned subset: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func scanBans(row rowScanner) (PlayerBans, error) {
	var (
		b       PlayerBans
		vac     int
		comm    int
		checked string
	)
	if err := row.Scan(&b.SteamID64, &vac, &b.NumberOfVACBans, &b.NumberOfGameBans,
		&comm, &b.DaysSinceLastBan, &b.EconomyBan, &checked); err != nil {
		return PlayerBans{}, err
	}
	b.VACBanned = vac != 0
	b.CommunityBanned = comm != 0
	b.CheckedAt = parseTime(checked)
	return b, nil
}
