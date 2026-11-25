package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PlayerProfile holds user-maintained identity and tags for one SteamID64.
// Parsed names are never overwritten; DisplayName is an optional override.
type PlayerProfile struct {
	SteamID64   string   `json:"steam_id64"`
	DisplayName string   `json:"display_name"`
	Note        string   `json:"note"`
	Tags        []string `json:"tags"`
	UpdatedAt   string   `json:"updated_at"`
}

// PlayerProfileRepository manages the player_profiles table.
type PlayerProfileRepository struct {
	db *DB
}

// PlayerProfiles returns a repository bound to db.
func (db *DB) PlayerProfiles() *PlayerProfileRepository {
	return &PlayerProfileRepository{db: db}
}

// NormalizeTags trims, drops empty entries, and de-duplicates case-insensitively.
func NormalizeTags(tags []string) []string {
	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

// ParseTagList splits a comma/semicolon separated tag string.
func ParseTagList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '，' || r == '；'
	})
	return NormalizeTags(fields)
}

// Upsert inserts or replaces a profile, normalizing tags and stamping updated_at.
func (r *PlayerProfileRepository) Upsert(ctx context.Context, p PlayerProfile) (PlayerProfile, error) {
	p.SteamID64 = strings.TrimSpace(p.SteamID64)
	if p.SteamID64 == "" {
		return PlayerProfile{}, fmt.Errorf("steam_id64 is required")
	}
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	p.Note = strings.TrimSpace(p.Note)
	p.Tags = NormalizeTags(p.Tags)
	if p.Tags == nil {
		p.Tags = []string{}
	}
	raw, err := json.Marshal(p.Tags)
	if err != nil {
		return PlayerProfile{}, fmt.Errorf("encode tags: %w", err)
	}
	if p.UpdatedAt == "" {
		p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO player_profiles (steam_id64, display_name, note, tags_json, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(steam_id64) DO UPDATE SET
			display_name = excluded.display_name,
			note = excluded.note,
			tags_json = excluded.tags_json,
			updated_at = excluded.updated_at`,
		p.SteamID64, p.DisplayName, p.Note, string(raw), p.UpdatedAt); err != nil {
		return PlayerProfile{}, fmt.Errorf("upsert player profile: %w", err)
	}
	return p, nil
}

// Get returns the profile for steamID; found=false when none exists.
func (r *PlayerProfileRepository) Get(ctx context.Context, steamID string) (PlayerProfile, bool, error) {
	row := r.db.sql.QueryRowContext(ctx, `
		SELECT steam_id64, display_name, note, tags_json, updated_at
		FROM player_profiles WHERE steam_id64 = ?`, strings.TrimSpace(steamID))
	p, err := scanPlayerProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerProfile{}, false, nil
	}
	if err != nil {
		return PlayerProfile{}, false, err
	}
	return p, true, nil
}

// List returns all profiles ordered by most recently updated.
func (r *PlayerProfileRepository) List(ctx context.Context) ([]PlayerProfile, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT steam_id64, display_name, note, tags_json, updated_at
		FROM player_profiles ORDER BY updated_at DESC, steam_id64 ASC`)
	if err != nil {
		return nil, fmt.Errorf("list player profiles: %w", err)
	}
	defer rows.Close()
	var out []PlayerProfile
	for rows.Next() {
		p, err := scanPlayerProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TagsForSteamIDs returns tags keyed by SteamID64 for the requested players.
func (r *PlayerProfileRepository) TagsForSteamIDs(ctx context.Context, steamIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(steamIDs))
	if len(steamIDs) == 0 {
		return out, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(steamIDs)), ",")
	args := make([]any, len(steamIDs))
	for i, id := range steamIDs {
		args[i] = id
	}
	rows, err := r.db.sql.QueryContext(ctx,
		`SELECT steam_id64, tags_json FROM player_profiles WHERE steam_id64 IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		out[id] = decodeTags(raw)
	}
	return out, rows.Err()
}

// Delete removes a profile; missing rows are not an error.
func (r *PlayerProfileRepository) Delete(ctx context.Context, steamID string) error {
	_, err := r.db.sql.ExecContext(ctx, `DELETE FROM player_profiles WHERE steam_id64 = ?`, strings.TrimSpace(steamID))
	if err != nil {
		return fmt.Errorf("delete player profile: %w", err)
	}
	return nil
}

func scanPlayerProfile(s rowScanner) (PlayerProfile, error) {
	var p PlayerProfile
	var raw string
	if err := s.Scan(&p.SteamID64, &p.DisplayName, &p.Note, &raw, &p.UpdatedAt); err != nil {
		return PlayerProfile{}, err
	}
	p.Tags = decodeTags(raw)
	return p, nil
}

func decodeTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return []string{}
	}
	return NormalizeTags(tags)
}
