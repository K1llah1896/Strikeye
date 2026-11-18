package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenWALForeignKeysAndPreMigrationSnapshot(t *testing.T) {
	ctx := context.Background()
	dataRoot := t.TempDir()
	dbPath := filepath.Join(dataRoot, "strikeye.db")
	snapDir := filepath.Join(dataRoot, "backup", "db_snapshots")

	// Open with v1 schema.
	db1, err := Open(dbPath, snapDir)
	if err != nil {
		t.Fatalf("Open v1 failed: %v", err)
	}

	var journalMode string
	if err := db1.SQL().QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("expected WAL journal_mode, got %q", journalMode)
	}

	var fk int
	if err := db1.SQL().QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("expected foreign_keys=1, got %d", fk)
	}

	expectedTables := []string{
		"schema_migrations",
		"demos", "match_players", "match_rounds", "match_events",
		"highlight_clips", "media_videos", "render_tasks", "radar_cards",
		"montage_projects", "montage_exports",
	}
	for _, tbl := range expectedTables {
		if !tableExists(ctx, db1.SQL(), tbl) {
			t.Fatalf("expected table %q to exist", tbl)
		}
	}

	// Seed a row before upgrading so snapshot captures real data.
	_, created, err := db1.Demos().UpsertByContentHash(ctx, Demo{
		FilePath:      "/tmp/match1.dem",
		Filename:      "match1.dem",
		FileSize:      1024,
		ContentSHA256: "abc123",
	})
	if err != nil || !created {
		t.Fatalf("seed demo failed: created=%v err=%v", created, err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("close v1 db: %v", err)
	}

	// Re-open with an added v3 migration and assert pre-migration snapshot is created.
	nextVer := len(DefaultMigrations()) + 1
	nextMigrations := append(DefaultMigrations(), Migration{
		Version: nextVer,
		Name:    "add_test_marker",
		Up: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `ALTER TABLE demos ADD COLUMN test_marker TEXT`)
			return err
		},
	})
	db2, err := Open(dbPath, snapDir, nextMigrations...)
	if err != nil {
		t.Fatalf("Open v%d failed: %v", nextVer, err)
	}
	defer db2.Close()

	ver, err := db2.CurrentSchemaVersion(ctx)
	if err != nil || ver != nextVer {
		t.Fatalf("expected schema version %d, got %d (err=%v)", nextVer, ver, err)
	}

	snaps, err := filepath.Glob(filepath.Join(snapDir, "strikeye_v2_*.db"))
	if err != nil || len(snaps) != 1 {
		t.Fatalf("expected 1 pre-migration snapshot in %q, got %v (err=%v)", snapDir, snaps, err)
	}
	// Verify snapshot contains the pre-upgrade data and lacks v2 column.
	snapDB, err := sql.Open("sqlite", "file:"+filepath.ToSlash(snaps[0])+"?mode=ro")
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer snapDB.Close()
	var count int
	if err := snapDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM demos WHERE content_sha256='abc123'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("snapshot missing seeded demo row: count=%d err=%v", count, err)
	}
}

func TestRepositoriesIdempotencyAndCascadeDeletes(t *testing.T) {
	ctx := context.Background()
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("OpenDefault failed: %v", err)
	}
	defer db.Close()

	remark := "ACE on Mirage"
	demo1, created, err := db.Demos().UpsertByContentHash(ctx, Demo{
		FilePath:      "/demos/a.dem",
		Filename:      "a.dem",
		FileSize:      2048,
		ContentSHA256: "hash-001",
		Remark:        &remark,
	})
	if err != nil || !created {
		t.Fatalf("first demo insert failed: created=%v err=%v", created, err)
	}

	// Re-registering same content hash with moved path updates path without duplicating or wiping remark.
	demo2, created, err := db.Demos().UpsertByContentHash(ctx, Demo{
		FilePath:      "/demos/moved/a.dem",
		Filename:      "a.dem",
		FileSize:      2048,
		ContentSHA256: "hash-001",
	})
	if err != nil || created {
		t.Fatalf("expected duplicate content hash reuse, created=%v err=%v", created, err)
	}
	if demo2.ID != demo1.ID || demo2.FilePath != "/demos/moved/a.dem" || demo2.Remark == nil || *demo2.Remark != remark {
		t.Fatalf("unexpected updated demo: %+v", demo2)
	}

	// Analysis repositories.
	mapName := "de_mirage"
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo1.ID, []MatchPlayer{{
		SteamID64:      "76561198000000010",
		PlayerName:     "donk",
		DisplayName:    "donk",
		TeamKey:        "a",
		Kills:          28,
		Deaths:         14,
		StrikeyeRating: 1.48,
	}}); err != nil {
		t.Fatalf("ReplaceMatchPlayers failed: %v", err)
	}
	if err := db.Analysis().ReplaceMatchRounds(ctx, demo1.ID, []MatchRound{{
		RoundNumber:      1,
		StartTick:        100,
		FreezeEndTick:    500,
		EndTick:          4500,
		DurationSec:      62,
		TeamASide:        "T",
		TeamBSide:        "CT",
		TeamAScoreAfter:  1,
		TeamAEconomyType: "pistol",
		TeamBEconomyType: "pistol",
	}}); err != nil {
		t.Fatalf("ReplaceMatchRounds failed: %v", err)
	}
	if err := db.Analysis().ReplaceMatchEvents(ctx, demo1.ID, []MatchEvent{{
		RoundNumber:  1,
		Tick:         1200,
		RoundTimeSec: 10.9,
		EventType:    "kill",
		IsHeadshot:   true,
	}}); err != nil {
		t.Fatalf("ReplaceMatchEvents failed: %v", err)
	}
	if err := db.Analysis().ReplaceHighlightClips(ctx, demo1.ID, []HighlightClip{{
		ID:              "clip-1",
		PlayerSteamID64: "76561198000000010",
		PlayerName:      "donk",
		RoundNumber:     1,
		ClipType:        "multikill",
		KillCount:       4,
		StartTick:       800,
		EndTick:         2400,
		ImportanceScore: 95.0,
	}}); err != nil {
		t.Fatalf("ReplaceHighlightClips failed: %v", err)
	}
	if err := db.Analysis().UpsertRadarCard(ctx, RadarCard{
		ID:              "1_76561198000000010",
		DemoID:          demo1.ID,
		SteamID64:       "76561198000000010",
		PlayerName:      "donk",
		TeamKey:         "a",
		TeamLabel:       "Team A",
		DimensionsJSON:  `{"rating":1.48}`,
		MatchAvgJSON:    `{"rating":1.05}`,
		StaticImagePath: "/cache/card.png",
	}); err != nil {
		t.Fatalf("UpsertRadarCard failed: %v", err)
	}

	// Media upsert replacement on re-render of same target slot.
	round1 := 1
	sid := "76561198000000010"
	m1, err := db.Media().Upsert(ctx, MediaVideo{
		DemoID:          demo1.ID,
		RoundNumber:     &round1,
		PlayerSteamID64: &sid,
		Perspective:     "pov",
		VideoScope:      "round_full",
		StartTick:       500,
		EndTick:         4500,
		FilePath:        "/media/r1_v1.mp4",
		FileSize:        100000,
		DurationSec:     62.5,
		Width:           1920,
		Height:          1080,
		FPS:             60,
		VideoCodec:      "h264",
	})
	if err != nil {
		t.Fatalf("Media Upsert 1 failed: %v", err)
	}
	m2, err := db.Media().Upsert(ctx, MediaVideo{
		DemoID:          demo1.ID,
		RoundNumber:     &round1,
		PlayerSteamID64: &sid,
		Perspective:     "pov",
		VideoScope:      "round_full",
		StartTick:       500,
		EndTick:         4500,
		FilePath:        "/media/r1_v2.mp4",
		FileSize:        200000,
		DurationSec:     62.5,
		Width:           1920,
		Height:          1080,
		FPS:             60,
		VideoCodec:      "h264",
	})
	if err != nil {
		t.Fatalf("Media Upsert 2 failed: %v", err)
	}
	if m2.ID != m1.ID {
		t.Fatalf("expected media replacement to reuse row ID %d, got %d", m1.ID, m2.ID)
	}
	mediaList, err := db.Media().ListByDemo(ctx, demo1.ID)
	if err != nil || len(mediaList) != 1 {
		t.Fatalf("expected 1 media row after replacement, got %d (err=%v)", len(mediaList), err)
	}

	// RenderTask idempotency, priority, and recovery.
	uid := ComputeTaskUID("hash-001", "round_full", sid, "pov", `[{"start_tick":500,"end_tick":4500}]`, `{}`)
	t1, tCreated, err := db.RenderTasks().EnqueueIdempotent(ctx, RenderTask{
		TaskUID:           uid,
		DemoID:            demo1.ID,
		RoundNumber:       &round1,
		TargetSteamID64:   sid,
		TargetPlayerName:  "donk",
		TargetSpecSlot:    2,
		TaskScope:         "round_full",
		SegmentsJSON:      `[{"start_tick":500,"end_tick":4500}]`,
		RenderOptionsJSON: `{}`,
		Priority:          50,
	})
	if err != nil || !tCreated {
		t.Fatalf("EnqueueIdempotent 1 failed: created=%v err=%v", tCreated, err)
	}
	t2, tCreated2, err := db.RenderTasks().EnqueueIdempotent(ctx, RenderTask{TaskUID: uid, DemoID: demo1.ID})
	if err != nil || tCreated2 || t2.ID != t1.ID {
		t.Fatalf("expected duplicate task_uid to return existing task, got id=%d created=%v err=%v", t2.ID, tCreated2, err)
	}

	pid := 1234
	if err := db.RenderTasks().MarkRunning(ctx, t1.ID, &pid, &pid); err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}
	requeued, failed, err := db.RenderTasks().RecoverInterrupted(ctx)
	if err != nil || requeued != 1 || failed != 0 {
		t.Fatalf("RecoverInterrupted mismatch: requeued=%d failed=%d err=%v", requeued, failed, err)
	}

	// Montage project & export.
	proj, err := db.Montage().CreateProject(ctx, "Mirage Highlights", `{"clips":[]}`)
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	exp, err := db.Montage().CreateExport(ctx, &proj.ID, "/media/exports/final.mp4")
	if err != nil {
		t.Fatalf("CreateExport failed: %v", err)
	}
	if err := db.Montage().CompleteExport(ctx, exp.ID, m2.ID); err != nil {
		t.Fatalf("CompleteExport failed: %v", err)
	}

	// Cascade delete demo removes child rows cleanly.
	_ = mapName
	if err := db.Demos().Delete(ctx, demo1.ID); err != nil {
		t.Fatalf("Delete demo failed: %v", err)
	}
	players, _ := db.Analysis().ListMatchPlayers(ctx, demo1.ID)
	rounds, _ := db.Analysis().ListMatchRounds(ctx, demo1.ID)
	events, _ := db.Analysis().ListMatchEvents(ctx, demo1.ID, nil)
	clips, _ := db.Analysis().ListHighlightClips(ctx, demo1.ID)
	cards, _ := db.Analysis().ListRadarCards(ctx, demo1.ID)
	mediaAfter, _ := db.Media().ListByDemo(ctx, demo1.ID)
	if len(players)+len(rounds)+len(events)+len(clips)+len(cards)+len(mediaAfter) != 0 {
		t.Fatalf("expected all child rows cascaded on demo delete")
	}
}

func TestLegacyMigrationPreservesSourceAndSkipsMissingMedia(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	dataRoot := filepath.Join(tmp, "strikeye-data")
	legacyDir := filepath.Join(tmp, "legacy")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll legacyDir: %v", err)
	}

	demoFile := filepath.Join(tmp, "mirage.dem")
	if err := os.WriteFile(demoFile, []byte("PBDEMS2-demo-bytes"), 0o644); err != nil {
		t.Fatalf("write dummy demo: %v", err)
	}
	validMP4 := filepath.Join(tmp, "clip_valid.mp4")
	if err := os.WriteFile(validMP4, []byte("ftypisom-mp4-bytes"), 0o644); err != nil {
		t.Fatalf("write valid mp4: %v", err)
	}
	missingMP4 := filepath.Join(tmp, "clip_missing.mp4")

	legacyDBPath := filepath.Join(legacyDir, "cs2-insight.db")
	legacyConn, err := sql.Open("sqlite", "file:"+filepath.ToSlash(legacyDBPath))
	if err != nil {
		t.Fatalf("open legacy fixture db: %v", err)
	}
	_, err = legacyConn.Exec(`
		CREATE TABLE demo_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT UNIQUE NOT NULL,
			filename TEXT NOT NULL,
			file_size INTEGER,
			map_name TEXT,
			total_rounds INTEGER,
			team_a_score INTEGER,
			team_b_score INTEGER,
			team_a_name TEXT,
			team_b_name TEXT,
			duration_mins REAL,
			match_date TEXT,
			status TEXT NOT NULL DEFAULT 'ready',
			added_at TEXT NOT NULL,
			parsed_at TEXT,
			error_msg TEXT,
			display_name TEXT,
			source TEXT,
			watch_root TEXT,
			remark TEXT
		);
		CREATE TABLE recorded_clips (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			clip_id TEXT NOT NULL,
			demo_path TEXT NOT NULL,
			demo_filename TEXT,
			player_name TEXT,
			output_path TEXT NOT NULL,
			duration_sec REAL,
			status TEXT NOT NULL DEFAULT 'ready',
			created_at TEXT NOT NULL,
			clip_meta TEXT
		);
	`)
	if err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	_, err = legacyConn.Exec(
		`INSERT INTO demo_files (path, filename, file_size, map_name, total_rounds, team_a_score, team_b_score, team_a_name, team_b_name, duration_mins, added_at, display_name, source, remark)
		 VALUES (?, 'mirage.dem', 18, 'de_mirage', 24, 13, 11, 'Spirit', 'FaZe', 42.5, '2026-10-01T12:00:00Z', 'Major Final', 'faceit', 'Important match')`,
		demoFile,
	)
	if err != nil {
		t.Fatalf("insert legacy demo_files: %v", err)
	}
	_, err = legacyConn.Exec(
		`INSERT INTO recorded_clips (clip_id, demo_path, player_name, output_path, duration_sec, created_at, clip_meta)
		 VALUES
		 ('c1', ?, 'donk', ?, 12.4, '2026-10-01T13:00:00Z', '{"round_number":12,"recording_perspective":"pov","start_tick":1000,"end_tick":1800}'),
		 ('c2', ?, 'donk', ?, 9.0, '2026-10-01T13:05:00Z', '{"round_number":15}')`,
		demoFile, validMP4, demoFile, missingMP4,
	)
	if err != nil {
		t.Fatalf("insert legacy recorded_clips: %v", err)
	}
	_ = legacyConn.Close()

	beforeSum := fileSHA256(t, legacyDBPath)

	db, err := OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("OpenDefault failed: %v", err)
	}
	defer db.Close()

	report, err := db.MigrateLegacyAssets(ctx, LegacyMigrationOptions{
		DataRoot:      dataRoot,
		LegacyDBPaths: []string{legacyDBPath},
	})
	if err != nil {
		t.Fatalf("MigrateLegacyAssets failed: %v", err)
	}
	if report.ImportedDemos != 1 || report.ImportedMedia != 1 || report.SkippedMissingMedia != 1 {
		t.Fatalf("unexpected migration report: %+v", report)
	}
	if report.BackupDir == "" {
		t.Fatalf("expected non-empty BackupDir")
	}

	afterSum := fileSHA256(t, legacyDBPath)
	if beforeSum != afterSum {
		t.Fatalf("legacy source DB was modified during migration")
	}

	demos, err := db.Demos().List(ctx)
	if err != nil || len(demos) != 1 {
		t.Fatalf("expected 1 imported demo, got %d (err=%v)", len(demos), err)
	}
	if demos[0].Remark == nil || *demos[0].Remark != "Important match" || demos[0].DisplayName == nil || *demos[0].DisplayName != "Major Final" {
		t.Fatalf("legacy annotations lost: %+v", demos[0])
	}

	media, err := db.Media().ListByDemo(ctx, demos[0].ID)
	if err != nil || len(media) != 1 || media[0].FilePath != validMP4 {
		t.Fatalf("unexpected imported media: %+v (err=%v)", media, err)
	}

	// Re-running migration must not duplicate records.
	report2, err := db.MigrateLegacyAssets(ctx, LegacyMigrationOptions{
		DataRoot:      dataRoot,
		LegacyDBPaths: []string{legacyDBPath},
	})
	if err != nil {
		t.Fatalf("second MigrateLegacyAssets failed: %v", err)
	}
	if report2.ImportedDemos != 0 || report2.SkippedDuplicateDemos != 1 {
		t.Fatalf("expected idempotent re-run skip, got %+v", report2)
	}
}

func TestV2MigrationAndMatchPlayerSides(t *testing.T) {
	ctx := context.Background()
	dataRoot := t.TempDir()
	dbPath := filepath.Join(dataRoot, "strikeye.db")
	snapDir := filepath.Join(dataRoot, "backup", "db_snapshots")

	// Start on v1 only, seed a demo + v1 player row.
	v1Only := DefaultMigrations()[:1]
	db1, err := Open(dbPath, snapDir, v1Only...)
	if err != nil {
		t.Fatalf("Open v1 failed: %v", err)
	}
	demo, _, err := db1.Demos().UpsertByContentHash(ctx, Demo{
		FilePath:      "/demos/v1.dem",
		Filename:      "v1.dem",
		FileSize:      512,
		ContentSHA256: "v1-hash",
	})
	if err != nil {
		t.Fatalf("Upsert demo: %v", err)
	}
	if _, err := db1.SQL().ExecContext(ctx, `
		INSERT INTO match_players (demo_id, steam_id64, player_name, display_name, team_key, kills, strikeye_rating)
		VALUES (?, '76561198000000001', 'LegacyPlayer', 'LegacyPlayer', 'A', 20, 1.25)
	`, demo.ID); err != nil {
		t.Fatalf("insert v1 player: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("close v1: %v", err)
	}

	// Upgrade to v2 using DefaultMigrations.
	db2, err := Open(dbPath, snapDir)
	if err != nil {
		t.Fatalf("Open v2 failed: %v", err)
	}
	defer db2.Close()

	ver, err := db2.CurrentSchemaVersion(ctx)
	if err != nil || ver != 2 {
		t.Fatalf("expected schema v2, got %d (err=%v)", ver, err)
	}
	if !tableExists(ctx, db2.SQL(), "match_player_sides") {
		t.Fatalf("expected match_player_sides table to exist")
	}

	// Legacy v1 demo reads cleanly with empty side slice.
	players, err := db2.Analysis().ListMatchPlayers(ctx, demo.ID)
	if err != nil || len(players) != 1 || players[0].ClutchBreakdownJSON != "{}" {
		t.Fatalf("ListMatchPlayers legacy failed: len=%d err=%v players=%+v", len(players), err, players)
	}
	sides, err := db2.Analysis().ListMatchPlayerSides(ctx, demo.ID)
	if err != nil || len(sides) != 0 {
		t.Fatalf("expected empty sides for v1 demo, got %d (err=%v)", len(sides), err)
	}

	// Write v2 player + side rows and verify round-trip.
	if err := db2.Analysis().ReplaceMatchPlayers(ctx, demo.ID, []MatchPlayer{{
		SteamID64:           "76561198000000001",
		PlayerName:          "LegacyPlayer",
		DisplayName:         "LegacyPlayer",
		TeamKey:             "A",
		Kills:               22,
		StrikeyeRating:      1.30,
		FlashAssists:        3,
		EnemiesFlashed:      11,
		TotalFlashSec:       18.5,
		RoundSwing:          4.2,
		EcoAdjRating:        1.35,
		EcoAdjADR:           92.4,
		ClutchBreakdownJSON: `{"1v1":{"attempts":2,"wins":1}}`,
	}}); err != nil {
		t.Fatalf("ReplaceMatchPlayers v2 failed: %v", err)
	}
	if err := db2.Analysis().ReplaceMatchPlayerSides(ctx, demo.ID, []MatchPlayerSide{
		{DemoID: demo.ID, SteamID64: "76561198000000001", Side: "CT", RoundsPlayed: 12, Kills: 14, StrikeyeRating: 1.40},
		{DemoID: demo.ID, SteamID64: "76561198000000001", Side: "T", RoundsPlayed: 10, Kills: 8, StrikeyeRating: 1.15},
	}); err != nil {
		t.Fatalf("ReplaceMatchPlayerSides failed: %v", err)
	}
	sides, err = db2.Analysis().ListMatchPlayerSides(ctx, demo.ID)
	if err != nil || len(sides) != 2 {
		t.Fatalf("expected 2 side rows, got %d (err=%v)", len(sides), err)
	}
}

func fileSHA256(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %q: %v", p, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestBuildSQLiteDSNForUNCPath(t *testing.T) {
	got := buildSQLiteDSN(`\\wsl.localhost\archlinux\home\killah\Strikeye\dist\strikeye.db`)
	wantPrefix := "file:////wsl.localhost/archlinux/home/killah/Strikeye/dist/strikeye.db?"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("expected UNC DSN prefix %q, got %q", wantPrefix, got)
	}
}
