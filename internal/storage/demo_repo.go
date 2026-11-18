package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Demo represents a row in the demos table.
type Demo struct {
	ID                  int64
	FilePath            string
	Filename            string
	FileSize            int64
	ContentSHA256       string
	SourcePlatform      string
	WatchRoot           *string
	MapName             *string
	TickRate            float64
	MatchStartTick      int64
	DemoEndTick         int64
	TotalRounds         int
	HalftimeRound       *int
	TeamAName           string
	TeamBName           string
	TeamAScore          int
	TeamBScore          int
	DurationSec         float64
	MatchDate           *string
	ParseStatus         string
	ParseError          *string
	ParserVersion       *string
	RenderStatus        string
	InputDataCapability string
	ReplayCachePath     *string
	IsFavorite          bool
	DisplayName         *string
	Remark              *string
	AddedAt             string
	ParsedAt            *string
}

// DemoRepository manages persistence for the demos table.
type DemoRepository struct {
	db *DB
}

// Demos returns a DemoRepository bound to db.
func (db *DB) Demos() *DemoRepository {
	return &DemoRepository{db: db}
}

// UpsertByContentHash registers a demo idempotently by content_sha256.
// If a record with the same content_sha256 exists, its file_path/filename/file_size
// are updated while preserving user annotations (display_name, remark, is_favorite).
func (r *DemoRepository) UpsertByContentHash(ctx context.Context, d Demo) (Demo, bool, error) {
	if d.ContentSHA256 == "" {
		return Demo{}, false, fmt.Errorf("content_sha256 is required")
	}
	existing, err := r.GetByContentHash(ctx, d.ContentSHA256)
	if err == nil {
		if d.FilePath != "" && d.FilePath != existing.FilePath {
			if _, updErr := r.db.sql.ExecContext(
				ctx,
				`UPDATE demos SET file_path = ?, filename = ?, file_size = ?, watch_root = COALESCE(?, watch_root) WHERE id = ?`,
				d.FilePath, d.Filename, d.FileSize, d.WatchRoot, existing.ID,
			); updErr != nil {
				return Demo{}, false, fmt.Errorf("update existing demo path: %w", updErr)
			}
			existing, err = r.GetByID(ctx, existing.ID)
			if err != nil {
				return Demo{}, false, err
			}
		}
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Demo{}, false, err
	}

	if d.SourcePlatform == "" {
		d.SourcePlatform = "unknown"
	}
	if d.TickRate <= 0 {
		d.TickRate = 64.0
	}
	if d.TeamAName == "" {
		d.TeamAName = "Team A"
	}
	if d.TeamBName == "" {
		d.TeamBName = "Team B"
	}
	if d.ParseStatus == "" {
		d.ParseStatus = "pending"
	}
	if d.RenderStatus == "" {
		d.RenderStatus = "idle"
	}
	if d.InputDataCapability == "" {
		d.InputDataCapability = "none"
	}
	if d.AddedAt == "" {
		d.AddedAt = utcNowISO()
	}

	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO demos (
			file_path, filename, file_size, content_sha256, source_platform, watch_root,
			map_name, tick_rate, match_start_tick, demo_end_tick, total_rounds, halftime_round,
			team_a_name, team_b_name, team_a_score, team_b_score, duration_sec, match_date,
			parse_status, parse_error, parser_version, render_status, input_data_capability,
			replay_cache_path, is_favorite, display_name, remark, added_at, parsed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.FilePath, d.Filename, d.FileSize, d.ContentSHA256, d.SourcePlatform, d.WatchRoot,
		d.MapName, d.TickRate, d.MatchStartTick, d.DemoEndTick, d.TotalRounds, d.HalftimeRound,
		d.TeamAName, d.TeamBName, d.TeamAScore, d.TeamBScore, d.DurationSec, d.MatchDate,
		d.ParseStatus, d.ParseError, d.ParserVersion, d.RenderStatus, d.InputDataCapability,
		d.ReplayCachePath, boolToInt(d.IsFavorite), d.DisplayName, d.Remark, d.AddedAt, d.ParsedAt,
	)
	if err != nil {
		return Demo{}, false, fmt.Errorf("insert demo: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Demo{}, false, err
	}
	created, err := r.GetByID(ctx, id)
	return created, true, err
}

func (r *DemoRepository) GetByID(ctx context.Context, id int64) (Demo, error) {
	return scanDemo(r.db.sql.QueryRowContext(ctx, demoSelectBase+` WHERE id = ?`, id))
}

func (r *DemoRepository) GetByContentHash(ctx context.Context, sha256Hex string) (Demo, error) {
	return scanDemo(r.db.sql.QueryRowContext(ctx, demoSelectBase+` WHERE content_sha256 = ?`, sha256Hex))
}

func (r *DemoRepository) GetByFilePath(ctx context.Context, filePath string) (Demo, error) {
	return scanDemo(r.db.sql.QueryRowContext(ctx, demoSelectBase+` WHERE file_path = ?`, filePath))
}

func (r *DemoRepository) List(ctx context.Context) ([]Demo, error) {
	rows, err := r.db.sql.QueryContext(ctx, demoSelectBase+` ORDER BY COALESCE(match_date, added_at) DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Demo
	for rows.Next() {
		d, err := scanDemo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DemoRepository) UpdateAnnotations(ctx context.Context, id int64, displayName, remark *string, isFavorite bool) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE demos SET display_name = ?, remark = ?, is_favorite = ? WHERE id = ?`,
		displayName, remark, boolToInt(isFavorite), id,
	)
	return err
}

func (r *DemoRepository) UpdateParseResult(ctx context.Context, d Demo) error {
	_, err := r.db.sql.ExecContext(ctx, `
		UPDATE demos SET
			map_name = ?, tick_rate = ?, match_start_tick = ?, demo_end_tick = ?,
			total_rounds = ?, halftime_round = ?, team_a_name = ?, team_b_name = ?,
			team_a_score = ?, team_b_score = ?, duration_sec = ?, match_date = ?,
			parse_status = ?, parse_error = ?, parser_version = ?,
			input_data_capability = ?, replay_cache_path = ?, parsed_at = ?
		WHERE id = ?`,
		d.MapName, d.TickRate, d.MatchStartTick, d.DemoEndTick,
		d.TotalRounds, d.HalftimeRound, d.TeamAName, d.TeamBName,
		d.TeamAScore, d.TeamBScore, d.DurationSec, d.MatchDate,
		d.ParseStatus, d.ParseError, d.ParserVersion,
		d.InputDataCapability, d.ReplayCachePath, d.ParsedAt, d.ID,
	)
	return err
}

func (r *DemoRepository) UpdateRenderStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.sql.ExecContext(ctx, `UPDATE demos SET render_status = ? WHERE id = ?`, status, id)
	return err
}

func (r *DemoRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.sql.ExecContext(ctx, `DELETE FROM demos WHERE id = ?`, id)
	return err
}

const demoSelectBase = `SELECT
	id, file_path, filename, file_size, content_sha256, source_platform, watch_root,
	map_name, tick_rate, match_start_tick, demo_end_tick, total_rounds, halftime_round,
	team_a_name, team_b_name, team_a_score, team_b_score, duration_sec, match_date,
	parse_status, parse_error, parser_version, render_status, input_data_capability,
	replay_cache_path, is_favorite, display_name, remark, added_at, parsed_at
FROM demos`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDemo(s rowScanner) (Demo, error) {
	var d Demo
	var fav int
	err := s.Scan(
		&d.ID, &d.FilePath, &d.Filename, &d.FileSize, &d.ContentSHA256, &d.SourcePlatform, &d.WatchRoot,
		&d.MapName, &d.TickRate, &d.MatchStartTick, &d.DemoEndTick, &d.TotalRounds, &d.HalftimeRound,
		&d.TeamAName, &d.TeamBName, &d.TeamAScore, &d.TeamBScore, &d.DurationSec, &d.MatchDate,
		&d.ParseStatus, &d.ParseError, &d.ParserVersion, &d.RenderStatus, &d.InputDataCapability,
		&d.ReplayCachePath, &fav, &d.DisplayName, &d.Remark, &d.AddedAt, &d.ParsedAt,
	)
	if err != nil {
		return Demo{}, err
	}
	d.IsFavorite = fav != 0
	return d, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
