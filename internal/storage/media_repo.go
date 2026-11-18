package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// MediaVideo represents a row in media_videos.
type MediaVideo struct {
	ID              int64
	DemoID          int64
	RoundNumber     *int
	PlayerSteamID64 *string
	PlayerName      *string
	Perspective     string
	VideoScope      string
	StartTick       int64
	EndTick         int64
	SegmentsJSON    string
	FilePath        string
	FileSize        int64
	DurationSec     float64
	Width           int
	Height          int
	FPS             float64
	VideoCodec      string
	HasAudio        bool
	HasKeyOverlay   bool
	VerifiedStatus  string
	CreatedAt       string
}

// MediaRepository manages persistence for media_videos.
type MediaRepository struct {
	db *DB
}

// Media returns a MediaRepository bound to db.
func (db *DB) Media() *MediaRepository {
	return &MediaRepository{db: db}
}

// Upsert inserts or replaces an existing media_videos row matching either file_path
// or the logical target slot (demo_id, round_number, player_steam_id64, perspective, video_scope)
// for round_full and highlight_clip assets so re-renders never create duplicate index rows.
func (r *MediaRepository) Upsert(ctx context.Context, m MediaVideo) (MediaVideo, error) {
	if m.FilePath == "" {
		return MediaVideo{}, fmt.Errorf("file_path is required")
	}
	if m.Perspective == "" {
		m.Perspective = "pov"
	}
	if m.SegmentsJSON == "" {
		m.SegmentsJSON = "[]"
	}
	if m.VerifiedStatus == "" {
		m.VerifiedStatus = "verified"
	}
	if m.CreatedAt == "" {
		m.CreatedAt = utcNowISO()
	}

	existingID, err := r.findExistingID(ctx, m)
	if err != nil {
		return MediaVideo{}, err
	}
	if existingID > 0 {
		if _, err := r.db.sql.ExecContext(ctx, `
			UPDATE media_videos SET
				demo_id = ?, round_number = ?, player_steam_id64 = ?, player_name = ?,
				perspective = ?, video_scope = ?, start_tick = ?, end_tick = ?,
				segments_json = ?, file_path = ?, file_size = ?, duration_sec = ?,
				width = ?, height = ?, fps = ?, video_codec = ?,
				has_audio = ?, has_key_overlay = ?, verified_status = ?, created_at = ?
			WHERE id = ?`,
			m.DemoID, m.RoundNumber, m.PlayerSteamID64, m.PlayerName,
			m.Perspective, m.VideoScope, m.StartTick, m.EndTick,
			m.SegmentsJSON, m.FilePath, m.FileSize, m.DurationSec,
			m.Width, m.Height, m.FPS, m.VideoCodec,
			boolToInt(m.HasAudio), boolToInt(m.HasKeyOverlay), m.VerifiedStatus, m.CreatedAt,
			existingID,
		); err != nil {
			return MediaVideo{}, fmt.Errorf("update media_video %d: %w", existingID, err)
		}
		return r.GetByID(ctx, existingID)
	}

	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO media_videos (
			demo_id, round_number, player_steam_id64, player_name,
			perspective, video_scope, start_tick, end_tick,
			segments_json, file_path, file_size, duration_sec,
			width, height, fps, video_codec,
			has_audio, has_key_overlay, verified_status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.DemoID, m.RoundNumber, m.PlayerSteamID64, m.PlayerName,
		m.Perspective, m.VideoScope, m.StartTick, m.EndTick,
		m.SegmentsJSON, m.FilePath, m.FileSize, m.DurationSec,
		m.Width, m.Height, m.FPS, m.VideoCodec,
		boolToInt(m.HasAudio), boolToInt(m.HasKeyOverlay), m.VerifiedStatus, m.CreatedAt,
	)
	if err != nil {
		return MediaVideo{}, fmt.Errorf("insert media_video: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return MediaVideo{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *MediaRepository) findExistingID(ctx context.Context, m MediaVideo) (int64, error) {
	var id int64
	err := r.db.sql.QueryRowContext(ctx, `SELECT id FROM media_videos WHERE file_path = ?`, m.FilePath).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	if m.VideoScope == "round_full" || m.VideoScope == "highlight_clip" {
		err = r.db.sql.QueryRowContext(ctx, `
			SELECT id FROM media_videos
			WHERE demo_id = ?
			  AND round_number IS ?
			  AND player_steam_id64 IS ?
			  AND perspective = ?
			  AND video_scope = ?
			  AND start_tick = ?
			  AND end_tick = ?`,
			m.DemoID, m.RoundNumber, m.PlayerSteamID64, m.Perspective, m.VideoScope, m.StartTick, m.EndTick,
		).Scan(&id)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
	}
	return 0, nil
}

func (r *MediaRepository) GetByID(ctx context.Context, id int64) (MediaVideo, error) {
	return scanMedia(r.db.sql.QueryRowContext(ctx, mediaSelectBase+` WHERE id = ?`, id))
}

func (r *MediaRepository) GetByFilePath(ctx context.Context, filePath string) (MediaVideo, error) {
	return scanMedia(r.db.sql.QueryRowContext(ctx, mediaSelectBase+` WHERE file_path = ?`, filePath))
}

func (r *MediaRepository) ListByDemo(ctx context.Context, demoID int64) ([]MediaVideo, error) {
	rows, err := r.db.sql.QueryContext(ctx, mediaSelectBase+` WHERE demo_id = ? ORDER BY round_number ASC, id ASC`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaVideo
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MediaRepository) UpdateVerifiedStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.sql.ExecContext(ctx, `UPDATE media_videos SET verified_status = ? WHERE id = ?`, status, id)
	return err
}

func (r *MediaRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.sql.ExecContext(ctx, `DELETE FROM media_videos WHERE id = ?`, id)
	return err
}

const mediaSelectBase = `SELECT
	id, demo_id, round_number, player_steam_id64, player_name,
	perspective, video_scope, start_tick, end_tick,
	segments_json, file_path, file_size, duration_sec,
	width, height, fps, video_codec,
	has_audio, has_key_overlay, verified_status, created_at
FROM media_videos`

func scanMedia(s rowScanner) (MediaVideo, error) {
	var m MediaVideo
	var hasAudio, hasOverlay int
	err := s.Scan(
		&m.ID, &m.DemoID, &m.RoundNumber, &m.PlayerSteamID64, &m.PlayerName,
		&m.Perspective, &m.VideoScope, &m.StartTick, &m.EndTick,
		&m.SegmentsJSON, &m.FilePath, &m.FileSize, &m.DurationSec,
		&m.Width, &m.Height, &m.FPS, &m.VideoCodec,
		&hasAudio, &hasOverlay, &m.VerifiedStatus, &m.CreatedAt,
	)
	if err != nil {
		return MediaVideo{}, err
	}
	m.HasAudio = hasAudio != 0
	m.HasKeyOverlay = hasOverlay != 0
	return m, nil
}
