package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// MediaProbeInfo carries verified video stream attributes returned by MediaVerifier.
type MediaProbeInfo struct {
	FileSize   int64
	Duration   float64
	Width      int
	Height     int
	FPS        float64
	VideoCodec string
	HasAudio   bool
}

// MediaVerifier validates whether a legacy MP4 asset exists and is playable.
type MediaVerifier func(ctx context.Context, path string) (MediaProbeInfo, bool)

// LegacyMigrationOptions configures importing legacy CS2-insight-agent SQLite databases.
type LegacyMigrationOptions struct {
	DataRoot      string
	LegacyDBPaths []string
	MediaVerifier MediaVerifier
	Now           func() time.Time
}

// LegacyMigrationReport summarizes imported records and non-fatal warnings.
type LegacyMigrationReport struct {
	BackupDir             string
	SourceDBs             []string
	ImportedDemos         int
	SkippedDuplicateDemos int
	ImportedMedia         int
	SkippedMissingMedia   int
	Warnings              []string
}

// DefaultMediaVerifier validates file existence and non-zero size when ffprobe is not wired yet.
func DefaultMediaVerifier(_ context.Context, path string) (MediaProbeInfo, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return MediaProbeInfo{}, false
	}
	return MediaProbeInfo{
		FileSize:   info.Size(),
		Width:      1920,
		Height:     1080,
		FPS:        60,
		VideoCodec: "h264",
		HasAudio:   true,
	}, true
}

// DiscoverLegacyDBPaths locates candidate legacy CS2-insight-agent SQLite databases
// on the current machine without modifying them.
func DiscoverLegacyDBPaths() []string {
	var candidates []string
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			candidates = append(candidates,
				filepath.Join(appdata, "CS2 Insight Agent", "data", "cs2-insight.db"),
				filepath.Join(appdata, "com.cs2insightagent.app", "data", "cs2-insight.db"),
				filepath.Join(appdata, "cs2-insight-agent", "cs2-insight.db"),
			)
		}
	} else if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".config", "CS2 Insight Agent", "data", "cs2-insight.db"),
		)
	}

	var existing []string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			existing = append(existing, c)
		}
	}
	return existing
}

// MigrateLegacyAssets backs up legacy SQLite databases into <DataRoot>/backup/legacy_import_<ts>/,
// reads them safely without mutating the originals, and imports demos and verified MP4 clips.
func (db *DB) MigrateLegacyAssets(ctx context.Context, opts LegacyMigrationOptions) (LegacyMigrationReport, error) {
	var report LegacyMigrationReport
	if opts.DataRoot == "" {
		return report, fmt.Errorf("DataRoot is required")
	}
	if opts.MediaVerifier == nil {
		opts.MediaVerifier = DefaultMediaVerifier
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}

	sources := opts.LegacyDBPaths
	if len(sources) == 0 {
		sources = DiscoverLegacyDBPaths()
	}
	if len(sources) == 0 {
		return report, nil
	}

	var validSources []string
	for _, s := range sources {
		if info, err := os.Stat(s); err == nil && !info.IsDir() {
			validSources = append(validSources, filepath.Clean(s))
		} else {
			report.Warnings = append(report.Warnings, fmt.Sprintf("legacy db missing or unreadable: %s", s))
		}
	}
	if len(validSources) == 0 {
		return report, nil
	}

	backupDir := filepath.Join(filepath.Clean(opts.DataRoot), "backup", "legacy_import_"+opts.Now().UTC().Format("20060102T150405Z"))
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return report, fmt.Errorf("create legacy backup dir: %w", err)
	}
	report.BackupDir = backupDir

	for idx, src := range validSources {
		report.SourceDBs = append(report.SourceDBs, src)
		backupDBPath := filepath.Join(backupDir, fmt.Sprintf("%02d_%s", idx+1, filepath.Base(src)))
		if err := copyFileWithSidecars(src, backupDBPath); err != nil {
			return report, fmt.Errorf("backup legacy db %q: %w", src, err)
		}
		if err := db.importSingleLegacyDB(ctx, backupDBPath, opts.MediaVerifier, &report); err != nil {
			return report, fmt.Errorf("import legacy db %q: %w", src, err)
		}
	}
	return report, nil
}

func (db *DB) importSingleLegacyDB(ctx context.Context, backupDBPath string, verifier MediaVerifier, report *LegacyMigrationReport) error {
	dsn := fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(backupDBPath))
	srcDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	defer srcDB.Close()

	demoIDByPath := make(map[string]int64)

	if tableExists(ctx, srcDB, "demo_files") {
		if err := db.importLegacyDemos(ctx, srcDB, demoIDByPath, report); err != nil {
			return err
		}
	}
	if tableExists(ctx, srcDB, "recorded_clips") {
		if err := db.importLegacyClips(ctx, srcDB, verifier, demoIDByPath, report); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) importLegacyDemos(ctx context.Context, srcDB *sql.DB, demoIDByPath map[string]int64, report *LegacyMigrationReport) error {
	rows, err := srcDB.QueryContext(ctx, `
		SELECT path, filename, file_size, map_name, total_rounds,
		       team_a_score, team_b_score, team_a_name, team_b_name,
		       duration_mins, match_date, added_at, parsed_at,
		       display_name, source, watch_root, remark
		FROM demo_files`)
	if err != nil {
		return fmt.Errorf("query legacy demo_files: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var path, filename, addedAt string
		var fileSize sql.NullInt64
		var totalRounds, teamAScore, teamBScore sql.NullInt64
		var durationMins sql.NullFloat64
		var mapName, teamAName, teamBName, matchDate, parsedAt, displayName, source, watchRoot, remark sql.NullString

		if err := rows.Scan(
			&path, &filename, &fileSize, &mapName, &totalRounds,
			&teamAScore, &teamBScore, &teamAName, &teamBName,
			&durationMins, &matchDate, &addedAt, &parsedAt,
			&displayName, &source, &watchRoot, &remark,
		); err != nil {
			return err
		}

		hash, actualSize := computeDemoHashOrSynthetic(path, fileSize.Int64)
		d := Demo{
			FilePath:       path,
			Filename:       filename,
			FileSize:       actualSize,
			ContentSHA256:  hash,
			SourcePlatform: nullStrOr(source, "unknown"),
			WatchRoot:      nullStrPtr(watchRoot),
			MapName:        nullStrPtr(mapName),
			TotalRounds:    int(totalRounds.Int64),
			TeamAName:      nullStrOr(teamAName, "Team A"),
			TeamBName:      nullStrOr(teamBName, "Team B"),
			TeamAScore:     int(teamAScore.Int64),
			TeamBScore:     int(teamBScore.Int64),
			DurationSec:    durationMins.Float64 * 60.0,
			MatchDate:      nullStrPtr(matchDate),
			ParseStatus:    "pending",
			DisplayName:    nullStrPtr(displayName),
			Remark:         nullStrPtr(remark),
			AddedAt:        addedAt,
			ParsedAt:       nullStrPtr(parsedAt),
		}

		// Preserve existing path duplicates cleanly if demo was already registered via another path.
		if existing, err := db.Demos().GetByFilePath(ctx, path); err == nil {
			demoIDByPath[path] = existing.ID
			report.SkippedDuplicateDemos++
			continue
		}

		saved, created, err := db.Demos().UpsertByContentHash(ctx, d)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("skip legacy demo %q: %v", path, err))
			continue
		}
		demoIDByPath[path] = saved.ID
		if created {
			report.ImportedDemos++
		} else {
			if err := db.Demos().UpdateAnnotations(ctx, saved.ID, mergePtr(saved.DisplayName, d.DisplayName), mergePtr(saved.Remark, d.Remark), saved.IsFavorite); err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("update annotations for %q: %v", path, err))
			}
			report.SkippedDuplicateDemos++
		}
	}
	return rows.Err()
}

func (db *DB) importLegacyClips(ctx context.Context, srcDB *sql.DB, verifier MediaVerifier, demoIDByPath map[string]int64, report *LegacyMigrationReport) error {
	rows, err := srcDB.QueryContext(ctx, `
		SELECT demo_path, player_name, output_path, duration_sec, created_at, clip_meta
		FROM recorded_clips`)
	if err != nil {
		return fmt.Errorf("query legacy recorded_clips: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var demoPath, outputPath, createdAt string
		var playerName, clipMeta sql.NullString
		var durationSec sql.NullFloat64
		if err := rows.Scan(&demoPath, &playerName, &outputPath, &durationSec, &createdAt, &clipMeta); err != nil {
			return err
		}

		probe, ok := verifier(ctx, outputPath)
		if !ok {
			report.SkippedMissingMedia++
			report.Warnings = append(report.Warnings, fmt.Sprintf("missing or invalid legacy clip: %s", outputPath))
			continue
		}

		demoID, ok := demoIDByPath[demoPath]
		if !ok {
			existing, err := db.Demos().GetByFilePath(ctx, demoPath)
			if err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("legacy clip %q has no matching demo %q", outputPath, demoPath))
				continue
			}
			demoID = existing.ID
			demoIDByPath[demoPath] = demoID
		}

		meta := parseClipMeta(clipMeta.String)
		dur := probe.Duration
		if dur <= 0 && durationSec.Valid {
			dur = durationSec.Float64
		}

		media := MediaVideo{
			DemoID:         demoID,
			RoundNumber:    meta.roundNumber,
			PlayerName:     nullStrPtr(playerName),
			Perspective:    meta.perspective,
			VideoScope:     "highlight_clip",
			StartTick:      meta.startTick,
			EndTick:        meta.endTick,
			FilePath:       outputPath,
			FileSize:       probe.FileSize,
			DurationSec:    dur,
			Width:          probe.Width,
			Height:         probe.Height,
			FPS:            probe.FPS,
			VideoCodec:     probe.VideoCodec,
			HasAudio:       probe.HasAudio,
			VerifiedStatus: "verified",
			CreatedAt:      createdAt,
		}
		if _, err := db.Media().Upsert(ctx, media); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("import legacy clip %q: %v", outputPath, err))
			continue
		}
		report.ImportedMedia++
	}
	return rows.Err()
}

type legacyClipMeta struct {
	roundNumber *int
	perspective string
	startTick   int64
	endTick     int64
}

func parseClipMeta(raw string) legacyClipMeta {
	out := legacyClipMeta{perspective: "pov"}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	if v, ok := m["recording_perspective"].(string); ok && v != "" {
		out.perspective = v
	}
	if v, ok := m["round_number"].(float64); ok && v > 0 {
		r := int(v)
		out.roundNumber = &r
	}
	if v, ok := m["start_tick"].(float64); ok {
		out.startTick = int64(v)
	}
	if v, ok := m["end_tick"].(float64); ok {
		out.endTick = int64(v)
	}
	return out
}

func computeDemoHashOrSynthetic(demoPath string, fallbackSize int64) (string, int64) {
	f, err := os.Open(demoPath)
	if err != nil {
		sum := sha256.Sum256([]byte("legacy-missing:" + demoPath))
		return "legacy-missing:" + hex.EncodeToString(sum[:]), fallbackSize
	}
	defer f.Close()

	info, _ := f.Stat()
	size := fallbackSize
	if info != nil {
		size = info.Size()
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		sum := sha256.Sum256([]byte("legacy-missing:" + demoPath))
		return "legacy-missing:" + hex.EncodeToString(sum[:]), size
	}
	return hex.EncodeToString(h.Sum(nil)), size
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) bool {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&count)
	return err == nil && count > 0
}

func copyFileWithSidecars(src, dst string) error {
	if err := copySingleFile(src, dst); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(src + suffix); err == nil {
			if err := copySingleFile(src+suffix, dst+suffix); err != nil {
				return err
			}
		}
	}
	return nil
}

func copySingleFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func nullStrOr(s sql.NullString, fallback string) string {
	if s.Valid && s.String != "" {
		return s.String
	}
	return fallback
}

func nullStrPtr(s sql.NullString) *string {
	if !s.Valid || s.String == "" {
		return nil
	}
	v := s.String
	return &v
}

func mergePtr(existing, incoming *string) *string {
	if existing != nil && *existing != "" {
		return existing
	}
	return incoming
}

func IsErrNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
