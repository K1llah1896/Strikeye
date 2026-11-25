package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// SearchHit is one ranked full-text match of a demo.
type SearchHit struct {
	DemoID  int64   `json:"demo_id"`
	Players string  `json:"players"`
	Teams   string  `json:"teams"`
	MapName string  `json:"map_name"`
	Note    string  `json:"note"`
	Rank    float64 `json:"rank"`
}

// SearchRepo maintains the FTS5 library index.
type SearchRepo struct{ db *DB }

// Search returns the full-text search repository.
func (db *DB) Search() *SearchRepo { return &SearchRepo{db: db} }

// RebuildSearchIndex refills the FTS table from demos, players, profiles and
// annotations. The library is small (thousands of rows at most), so a full
// rebuild is cheaper than tracking incremental deltas.
func (r *SearchRepo) RebuildSearchIndex(ctx context.Context) (int, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT d.id,
		       COALESCE(d.team_a_name, ''), COALESCE(d.team_b_name, ''),
		       COALESCE(d.map_name, ''), COALESCE(d.remark, '')
		FROM demos d ORDER BY d.id ASC`)
	if err != nil {
		return 0, err
	}
	type doc struct {
		id      int64
		teams   string
		mapName string
		note    []string
	}
	var docs []doc
	for rows.Next() {
		var d doc
		var teamA, teamB, remark string
		if err := rows.Scan(&d.id, &teamA, &teamB, &d.mapName, &remark); err != nil {
			rows.Close()
			return 0, err
		}
		d.teams = strings.TrimSpace(teamA + " vs " + teamB)
		if r := strings.TrimSpace(remark); r != "" {
			d.note = append(d.note, r)
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	players := map[int64]string{}
	if rows, err := r.db.sql.QueryContext(ctx, `
		SELECT demo_id, GROUP_CONCAT(player_name, ' ') FROM (
			SELECT demo_id, player_name FROM match_players ORDER BY demo_id, player_name
		) GROUP BY demo_id`); err == nil {
		for rows.Next() {
			var id int64
			var names sql.NullString
			if err := rows.Scan(&id, &names); err != nil {
				rows.Close()
				return 0, err
			}
			players[id] = names.String
		}
		rows.Close()
	}

	if rows, err := r.db.sql.QueryContext(ctx, `
		SELECT demo_id, GROUP_CONCAT(callout_id || ' ' || note, ' ')
		FROM map_annotations GROUP BY demo_id`); err == nil {
		notes := map[int64]string{}
		for rows.Next() {
			var id int64
			var note sql.NullString
			if err := rows.Scan(&id, &note); err != nil {
				rows.Close()
				return 0, err
			}
			notes[id] = note.String
		}
		rows.Close()
		for i := range docs {
			if n, ok := notes[docs[i].id]; ok && n != "" {
				docs[i].note = append(docs[i].note, n)
			}
		}
	}

	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM match_search`); err != nil {
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO match_search (demo_id, players, teams, map_name, note)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, d := range docs {
		if _, err := stmt.ExecContext(ctx, d.id, players[d.id], d.teams, d.mapName, strings.Join(d.note, " ")); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(docs), nil
}

// EnsureSearchIndex rebuilds the index when it does not cover every demo.
func (r *SearchRepo) EnsureSearchIndex(ctx context.Context) (bool, error) {
	var demos, indexed int
	if err := r.db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM demos`).Scan(&demos); err != nil {
		return false, err
	}
	if err := r.db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM match_search`).Scan(&indexed); err != nil {
		return false, err
	}
	if demos == indexed {
		return false, nil
	}
	if _, err := r.RebuildSearchIndex(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// Query runs an FTS5 prefix match over the library index, plus a substring pass
// for scripts the FTS tokenizer cannot split (CJK has no spaces, so a note like
// "默认双架" is one token and a query for "双架" would never match it).
func (r *SearchRepo) Query(ctx context.Context, text string, limit int) ([]SearchHit, error) {
	match := ftsQuery(text)
	needle := strings.TrimSpace(text)
	if match == "" && needle == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	hits, err := r.queryFTS(ctx, match, limit)
	if err != nil {
		return nil, err
	}
	if !needsSubstringPass(needle) || len(hits) >= limit {
		return hits, nil
	}
	extra, err := r.querySubstring(ctx, needle, limit-len(hits))
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(hits))
	for _, h := range hits {
		seen[h.DemoID] = true
	}
	for _, h := range extra {
		if seen[h.DemoID] {
			continue
		}
		hits = append(hits, h)
	}
	return hits, nil
}

// needsSubstringPass reports whether the query contains characters FTS5's
// unicode61 tokenizer keeps inside a single token, i.e. CJK text.
func needsSubstringPass(text string) bool {
	for _, r := range text {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

// querySubstring does a case-insensitive LIKE over the indexed columns.
func (r *SearchRepo) querySubstring(ctx context.Context, needle string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		return nil, nil
	}
	pattern := "%" + strings.ToLower(needle) + "%"
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT demo_id, players, teams, map_name, note, 0.0
		FROM match_search
		WHERE lower(players) LIKE ? OR lower(teams) LIKE ? OR lower(map_name) LIKE ? OR lower(note) LIKE ?
		ORDER BY demo_id ASC
		LIMIT ?`, pattern, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("search substring: %w", err)
	}
	defer rows.Close()
	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		var players, teams, mapName, note sql.NullString
		if err := rows.Scan(&h.DemoID, &players, &teams, &mapName, &note, &h.Rank); err != nil {
			return nil, err
		}
		h.Players, h.Teams, h.MapName, h.Note = players.String, teams.String, mapName.String, note.String
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// queryFTS runs the ranked prefix match.
func (r *SearchRepo) queryFTS(ctx context.Context, match string, limit int) ([]SearchHit, error) {
	if match == "" {
		return nil, nil
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT demo_id, players, teams, map_name, note, bm25(match_search)
		FROM match_search
		WHERE match_search MATCH ?
		ORDER BY bm25(match_search) ASC, demo_id ASC
		LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		var players, teams, mapName, note sql.NullString
		if err := rows.Scan(&h.DemoID, &players, &teams, &mapName, &note, &h.Rank); err != nil {
			return nil, err
		}
		h.Players = players.String
		h.Teams = teams.String
		h.MapName = mapName.String
		h.Note = note.String
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// ftsQuery turns free text into a safe FTS5 prefix query: every token is
// reduced to letters/digits, so operators like NEAR/OR/"" typed by a user
// cannot break the query.
func ftsQuery(text string) string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		// FTS5 reserves these words; a bare "OR*" is a syntax error.
		switch strings.ToLower(f) {
		case "and", "or", "not", "near":
			continue
		}
		// Tokens are pure letters/digits (FieldsFunc stripped the rest), so they
		// cannot carry FTS5 operators; appending * makes them prefix matches.
		out = append(out, f+"*")
	}
	return strings.Join(out, " ")
}
