package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

// Migration defines one ordered schema upgrade step.
type Migration struct {
	Version int
	Name    string
	Up      func(ctx context.Context, tx *sql.Tx) error
}

// DefaultMigrations returns Strikeye's built-in ordered schema migrations.
func DefaultMigrations() []Migration {
	return []Migration{
		{
			Version: 1,
			Name:    "initial_strikeye_schema",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range initialSchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute initial DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 3,
			Name:    "player_profiles",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v3SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v3 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 4,
			Name:    "duel_timing_metrics",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v4SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v4 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 5,
			Name:    "visibility_time_to_damage",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v5SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v5 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 6,
			Name:    "player_bans",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v6SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v6 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 7,
			Name:    "map_annotations",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v7SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v7 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 8,
			Name:    "match_purchases",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v8SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v8 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 2,
			Name:    "hltv_match_analysis_v2",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v2SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v2 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 10,
			Name:    "library_search_index",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v10SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v10 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 11,
			Name:    "map_view_prefs",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v11SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v11 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 12,
			Name:    "kill_context",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v12SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v12 DDL: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version: 9,
			Name:    "map_strokes",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				for _, stmt := range v9SchemaStatements {
					if _, err := tx.ExecContext(ctx, stmt); err != nil {
						return fmt.Errorf("execute v9 DDL: %w", err)
					}
				}
				return nil
			},
		},
	}
}

func (db *DB) migrate(ctx context.Context, migrations []Migration) error {
	if len(migrations) == 0 {
		return nil
	}

	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	hasTables, err := db.hasUserTables(ctx)
	if err != nil {
		return err
	}

	if _, err := db.sql.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied, currentVersion, err := db.appliedVersions(ctx)
	if err != nil {
		return err
	}

	var pending []Migration
	for _, m := range sorted {
		if !applied[m.Version] {
			pending = append(pending, m)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	// Only snapshot when upgrading an existing non-empty database.
	if hasTables && db.snapshotDir != "" {
		snapName := fmt.Sprintf("strikeye_v%d_%s.db", currentVersion, time.Now().UTC().Format("20060102T150405.000000000Z"))
		snapPath := filepath.Join(db.snapshotDir, snapName)
		if err := db.Snapshot(ctx, snapPath); err != nil {
			return fmt.Errorf("pre-migration snapshot failed: %w", err)
		}
	}

	for _, m := range pending {
		tx, err := db.sql.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration v%d: %w", m.Version, err)
		}
		if err := m.Up(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration v%d (%s): %w", m.Version, m.Name, err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			m.Version, m.Name, utcNowISO(),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration v%d: %w", m.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d: %w", m.Version, err)
		}
	}
	return nil
}

func (db *DB) hasUserTables(ctx context.Context) (bool, error) {
	var count int
	err := db.sql.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("inspect sqlite_master: %w", err)
	}
	return count > 0, nil
}

func (db *DB) appliedVersions(ctx context.Context) (map[int]bool, int, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, 0, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	maxVer := 0
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, 0, err
		}
		applied[v] = true
		if v > maxVer {
			maxVer = v
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return applied, maxVer, nil
}

// CurrentSchemaVersion returns the highest applied migration version.
func (db *DB) CurrentSchemaVersion(ctx context.Context) (int, error) {
	_, maxVer, err := db.appliedVersions(ctx)
	return maxVer, err
}

var initialSchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS demos (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path               TEXT UNIQUE NOT NULL,
		filename                TEXT NOT NULL,
		file_size               INTEGER NOT NULL,
		content_sha256          TEXT UNIQUE NOT NULL,
		source_platform         TEXT NOT NULL DEFAULT 'unknown',
		watch_root              TEXT,
		map_name                TEXT,
		tick_rate               REAL NOT NULL DEFAULT 64.0,
		match_start_tick        INTEGER NOT NULL DEFAULT 0,
		demo_end_tick           INTEGER NOT NULL DEFAULT 0,
		total_rounds            INTEGER NOT NULL DEFAULT 0,
		halftime_round          INTEGER,
		team_a_name             TEXT NOT NULL DEFAULT 'Team A',
		team_b_name             TEXT NOT NULL DEFAULT 'Team B',
		team_a_score            INTEGER NOT NULL DEFAULT 0,
		team_b_score            INTEGER NOT NULL DEFAULT 0,
		duration_sec            REAL NOT NULL DEFAULT 0.0,
		match_date              TEXT,
		parse_status            TEXT NOT NULL DEFAULT 'pending',
		parse_error             TEXT,
		parser_version          TEXT,
		render_status           TEXT NOT NULL DEFAULT 'idle',
		input_data_capability   TEXT NOT NULL DEFAULT 'none',
		replay_cache_path       TEXT,
		is_favorite             INTEGER NOT NULL DEFAULT 0,
		display_name            TEXT,
		remark                  TEXT,
		added_at                TEXT NOT NULL,
		parsed_at               TEXT
	);`,
	`CREATE INDEX IF NOT EXISTS idx_demos_parse_status ON demos(parse_status);`,
	`CREATE INDEX IF NOT EXISTS idx_demos_map_date ON demos(map_name, match_date DESC);`,

	`CREATE TABLE IF NOT EXISTS match_players (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		demo_id                 INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
		steam_id64              TEXT NOT NULL,
		player_name             TEXT NOT NULL,
		display_name            TEXT NOT NULL,
		team_key                TEXT NOT NULL,
		spec_slot               INTEGER,
		kills                   INTEGER NOT NULL DEFAULT 0,
		deaths                  INTEGER NOT NULL DEFAULT 0,
		assists                 INTEGER NOT NULL DEFAULT 0,
		kd_ratio                REAL NOT NULL DEFAULT 0.0,
		kpr                     REAL NOT NULL DEFAULT 0.0,
		dpr                     REAL NOT NULL DEFAULT 0.0,
		apr                     REAL NOT NULL DEFAULT 0.0,
		total_damage            INTEGER NOT NULL DEFAULT 0,
		adr                     REAL NOT NULL DEFAULT 0.0,
		utility_damage          INTEGER NOT NULL DEFAULT 0,
		utility_adr             REAL NOT NULL DEFAULT 0.0,
		kast_percent            REAL NOT NULL DEFAULT 0.0,
		headshots               INTEGER NOT NULL DEFAULT 0,
		hs_percent              REAL NOT NULL DEFAULT 0.0,
		first_kills             INTEGER NOT NULL DEFAULT 0,
		first_deaths            INTEGER NOT NULL DEFAULT 0,
		opening_duel_win_rate   REAL NOT NULL DEFAULT 0.0,
		trade_kills             INTEGER NOT NULL DEFAULT 0,
		trade_deaths            INTEGER NOT NULL DEFAULT 0,
		survival_rate           REAL NOT NULL DEFAULT 0.0,
		one_kill_rounds         INTEGER NOT NULL DEFAULT 0,
		two_kill_rounds         INTEGER NOT NULL DEFAULT 0,
		three_kill_rounds       INTEGER NOT NULL DEFAULT 0,
		four_kill_rounds        INTEGER NOT NULL DEFAULT 0,
		five_kill_rounds        INTEGER NOT NULL DEFAULT 0,
		multi_kill_rounds       INTEGER NOT NULL DEFAULT 0,
		clutch_attempts         INTEGER NOT NULL DEFAULT 0,
		clutch_wins             INTEGER NOT NULL DEFAULT 0,
		awp_kills               INTEGER NOT NULL DEFAULT 0,
		avg_equipment_value     INTEGER NOT NULL DEFAULT 0,
		impact_score            REAL NOT NULL DEFAULT 0.0,
		strikeye_rating         REAL NOT NULL DEFAULT 0.0,
		UNIQUE(demo_id, steam_id64)
	);`,
	`CREATE INDEX IF NOT EXISTS idx_match_players_demo ON match_players(demo_id);`,

	`CREATE TABLE IF NOT EXISTS match_rounds (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		demo_id                 INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
		round_number            INTEGER NOT NULL,
		start_tick              INTEGER NOT NULL,
		freeze_end_tick         INTEGER NOT NULL,
		end_tick                INTEGER NOT NULL,
		duration_sec            INTEGER NOT NULL,
		winner_team_key         TEXT,
		winner_side             TEXT,
		win_reason              TEXT,
		bomb_site               TEXT,
		team_a_side             TEXT NOT NULL,
		team_b_side             TEXT NOT NULL,
		team_a_score_before     INTEGER NOT NULL,
		team_b_score_before     INTEGER NOT NULL,
		team_a_score_after      INTEGER NOT NULL,
		team_b_score_after      INTEGER NOT NULL,
		team_a_equip_value      INTEGER NOT NULL DEFAULT 0,
		team_b_equip_value      INTEGER NOT NULL DEFAULT 0,
		team_a_economy_type     TEXT NOT NULL DEFAULT 'full',
		team_b_economy_type     TEXT NOT NULL DEFAULT 'full',
		headline                TEXT,
		tags_json               TEXT NOT NULL DEFAULT '[]',
		player_economy_json     TEXT NOT NULL DEFAULT '[]',
		UNIQUE(demo_id, round_number)
	);`,

	`CREATE TABLE IF NOT EXISTS match_events (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		demo_id                 INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
		round_number            INTEGER NOT NULL,
		tick                    INTEGER NOT NULL,
		round_time_sec          REAL NOT NULL,
		event_type              TEXT NOT NULL,
		actor_steam_id64        TEXT,
		actor_name              TEXT,
		actor_team_key          TEXT,
		target_steam_id64       TEXT,
		target_name             TEXT,
		target_team_key         TEXT,
		assister_steam_id64     TEXT,
		assister_name           TEXT,
		weapon                  TEXT,
		is_headshot             INTEGER NOT NULL DEFAULT 0,
		is_wallbang             INTEGER NOT NULL DEFAULT 0,
		is_through_smoke        INTEGER NOT NULL DEFAULT 0,
		is_noscope              INTEGER NOT NULL DEFAULT 0,
		is_attacker_blind       INTEGER NOT NULL DEFAULT 0,
		is_assisted_flash       INTEGER NOT NULL DEFAULT 0,
		site                    TEXT,
		extra_json              TEXT
	);`,
	`CREATE INDEX IF NOT EXISTS idx_match_events_demo_round ON match_events(demo_id, round_number, tick);`,

	`CREATE TABLE IF NOT EXISTS media_videos (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		demo_id                 INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
		round_number            INTEGER,
		player_steam_id64       TEXT,
		player_name             TEXT,
		perspective             TEXT NOT NULL DEFAULT 'pov',
		video_scope             TEXT NOT NULL,
		start_tick              INTEGER NOT NULL,
		end_tick                INTEGER NOT NULL,
		segments_json           TEXT NOT NULL DEFAULT '[]',
		file_path               TEXT UNIQUE NOT NULL,
		file_size               INTEGER NOT NULL,
		duration_sec            REAL NOT NULL,
		width                   INTEGER NOT NULL,
		height                  INTEGER NOT NULL,
		fps                     REAL NOT NULL,
		video_codec             TEXT NOT NULL,
		has_audio               INTEGER NOT NULL DEFAULT 1,
		has_key_overlay         INTEGER NOT NULL DEFAULT 0,
		verified_status         TEXT NOT NULL DEFAULT 'verified',
		created_at              TEXT NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_media_videos_lookup ON media_videos(demo_id, round_number, player_steam_id64, perspective);`,

	`CREATE TABLE IF NOT EXISTS highlight_clips (
		id                      TEXT PRIMARY KEY,
		demo_id                 INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
		player_steam_id64       TEXT NOT NULL,
		player_name             TEXT NOT NULL,
		round_number            INTEGER NOT NULL,
		clip_type               TEXT NOT NULL,
		kill_count              INTEGER NOT NULL DEFAULT 0,
		start_tick              INTEGER NOT NULL,
		end_tick                INTEGER NOT NULL,
		anchor_ticks_json       TEXT NOT NULL DEFAULT '[]',
		pre_buffer_sec          REAL NOT NULL DEFAULT 3.0,
		post_buffer_sec         REAL NOT NULL DEFAULT 2.0,
		tags_json               TEXT NOT NULL DEFAULT '[]',
		weapon_label            TEXT,
		description             TEXT,
		importance_score        REAL NOT NULL DEFAULT 0.0,
		is_favorite             INTEGER NOT NULL DEFAULT 0,
		media_video_id          INTEGER REFERENCES media_videos(id) ON DELETE SET NULL,
		created_at              TEXT NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_highlight_clips_demo ON highlight_clips(demo_id, player_steam_id64);`,

	`CREATE TABLE IF NOT EXISTS render_tasks (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		task_uid                TEXT UNIQUE NOT NULL,
		demo_id                 INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
		clip_id                 TEXT REFERENCES highlight_clips(id) ON DELETE SET NULL,
		round_number            INTEGER,
		target_steam_id64       TEXT NOT NULL,
		target_player_name      TEXT NOT NULL,
		target_spec_slot        INTEGER NOT NULL,
		perspective             TEXT NOT NULL DEFAULT 'pov',
		task_scope              TEXT NOT NULL,
		segments_json           TEXT NOT NULL,
		render_options_json     TEXT NOT NULL,
		priority                INTEGER NOT NULL DEFAULT 10,
		status                  TEXT NOT NULL DEFAULT 'queued',
		retry_count             INTEGER NOT NULL DEFAULT 0,
		max_retries             INTEGER NOT NULL DEFAULT 2,
		spawned_hlae_pid        INTEGER,
		spawned_cs2_pid         INTEGER,
		output_media_id         INTEGER REFERENCES media_videos(id) ON DELETE SET NULL,
		error_code              TEXT,
		error_message           TEXT,
		created_at              TEXT NOT NULL,
		started_at              TEXT,
		finished_at             TEXT
	);`,
	`CREATE INDEX IF NOT EXISTS idx_render_tasks_queue ON render_tasks(status, priority DESC, id ASC);`,

	`CREATE TABLE IF NOT EXISTS radar_cards (
		id                      TEXT PRIMARY KEY,
		demo_id                 INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
		steam_id64              TEXT NOT NULL,
		player_name             TEXT NOT NULL,
		team_key                TEXT NOT NULL,
		team_label              TEXT NOT NULL,
		dimensions_json         TEXT NOT NULL,
		match_avg_json          TEXT NOT NULL,
		static_image_path       TEXT NOT NULL,
		anim_video_path         TEXT,
		custom_portrait_path    TEXT,
		custom_team_logo_path   TEXT,
		updated_at              TEXT NOT NULL,
		UNIQUE(demo_id, steam_id64)
	);`,

	`CREATE TABLE IF NOT EXISTS montage_projects (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		name                    TEXT NOT NULL,
		config_json             TEXT NOT NULL,
		created_at              TEXT NOT NULL,
		updated_at              TEXT NOT NULL
	);`,

	`CREATE TABLE IF NOT EXISTS montage_exports (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id              INTEGER REFERENCES montage_projects(id) ON DELETE SET NULL,
		output_path             TEXT NOT NULL,
		status                  TEXT NOT NULL DEFAULT 'pending',
		progress_percent        REAL NOT NULL DEFAULT 0.0,
		error_message           TEXT,
		media_video_id          INTEGER REFERENCES media_videos(id) ON DELETE SET NULL,
		created_at              TEXT NOT NULL,
		finished_at             TEXT
	);`,
}

var v2SchemaStatements = []string{
	`ALTER TABLE match_players ADD COLUMN flash_assists INTEGER NOT NULL DEFAULT 0;`,
	`ALTER TABLE match_players ADD COLUMN enemies_flashed INTEGER NOT NULL DEFAULT 0;`,
	`ALTER TABLE match_players ADD COLUMN total_flash_sec REAL NOT NULL DEFAULT 0.0;`,
	`ALTER TABLE match_players ADD COLUMN round_swing REAL NOT NULL DEFAULT 0.0;`,
	`ALTER TABLE match_players ADD COLUMN eco_adj_rating REAL NOT NULL DEFAULT 0.0;`,
	`ALTER TABLE match_players ADD COLUMN eco_adj_adr REAL NOT NULL DEFAULT 0.0;`,
	`ALTER TABLE match_players ADD COLUMN clutch_breakdown_json TEXT NOT NULL DEFAULT '{}';`,

	`CREATE TABLE IF NOT EXISTS match_player_sides (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		demo_id                 INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
		steam_id64              TEXT NOT NULL,
		side                    TEXT NOT NULL,
		rounds_played           INTEGER NOT NULL DEFAULT 0,
		kills                   INTEGER NOT NULL DEFAULT 0,
		deaths                  INTEGER NOT NULL DEFAULT 0,
		assists                 INTEGER NOT NULL DEFAULT 0,
		kd_ratio                REAL NOT NULL DEFAULT 0.0,
		kpr                     REAL NOT NULL DEFAULT 0.0,
		dpr                     REAL NOT NULL DEFAULT 0.0,
		apr                     REAL NOT NULL DEFAULT 0.0,
		total_damage            INTEGER NOT NULL DEFAULT 0,
		adr                     REAL NOT NULL DEFAULT 0.0,
		utility_damage          INTEGER NOT NULL DEFAULT 0,
		utility_adr             REAL NOT NULL DEFAULT 0.0,
		kast_percent            REAL NOT NULL DEFAULT 0.0,
		headshots               INTEGER NOT NULL DEFAULT 0,
		hs_percent              REAL NOT NULL DEFAULT 0.0,
		first_kills             INTEGER NOT NULL DEFAULT 0,
		first_deaths            INTEGER NOT NULL DEFAULT 0,
		opening_duel_win_rate   REAL NOT NULL DEFAULT 0.0,
		trade_kills             INTEGER NOT NULL DEFAULT 0,
		trade_deaths            INTEGER NOT NULL DEFAULT 0,
		survival_rate           REAL NOT NULL DEFAULT 0.0,
		multi_kill_rounds       INTEGER NOT NULL DEFAULT 0,
		clutch_attempts         INTEGER NOT NULL DEFAULT 0,
		clutch_wins             INTEGER NOT NULL DEFAULT 0,
		flash_assists           INTEGER NOT NULL DEFAULT 0,
		enemies_flashed         INTEGER NOT NULL DEFAULT 0,
		total_flash_sec         REAL NOT NULL DEFAULT 0.0,
		impact_score            REAL NOT NULL DEFAULT 0.0,
		round_swing             REAL NOT NULL DEFAULT 0.0,
		strikeye_rating         REAL NOT NULL DEFAULT 0.0,
		eco_adj_rating          REAL NOT NULL DEFAULT 0.0,
		eco_adj_adr             REAL NOT NULL DEFAULT 0.0,
		UNIQUE(demo_id, steam_id64, side)
	);`,
	`CREATE INDEX IF NOT EXISTS idx_match_player_sides_demo ON match_player_sides(demo_id, side);`,
}

var v4SchemaStatements = []string{
	`ALTER TABLE match_players ADD COLUMN ttk_avg_ms REAL NOT NULL DEFAULT 0.0;`,
	`ALTER TABLE match_players ADD COLUMN ttk_samples INTEGER NOT NULL DEFAULT 0;`,
}

var v5SchemaStatements = []string{
	`ALTER TABLE match_players ADD COLUMN ttd_avg_ms REAL NOT NULL DEFAULT 0.0;`,
	`ALTER TABLE match_players ADD COLUMN ttd_samples INTEGER NOT NULL DEFAULT 0;`,
}

var v3SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS player_profiles (
		steam_id64    TEXT PRIMARY KEY,
		display_name  TEXT NOT NULL DEFAULT '',
		note          TEXT NOT NULL DEFAULT '',
		tags_json     TEXT NOT NULL DEFAULT '[]',
		updated_at    TEXT NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_player_profiles_updated ON player_profiles(updated_at);`,
}

var v6SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS player_bans (
		steam_id64            TEXT PRIMARY KEY,
		vac_banned            INTEGER NOT NULL DEFAULT 0,
		number_of_vac_bans    INTEGER NOT NULL DEFAULT 0,
		number_of_game_bans   INTEGER NOT NULL DEFAULT 0,
		community_banned      INTEGER NOT NULL DEFAULT 0,
		days_since_last_ban   INTEGER NOT NULL DEFAULT 0,
		economy_ban           TEXT NOT NULL DEFAULT '',
		checked_at            TEXT NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_player_bans_checked ON player_bans(checked_at);`,
}

var v7SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS map_annotations (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		demo_id       INTEGER NOT NULL,
		round_number  INTEGER NOT NULL DEFAULT 0,
		callout_id    TEXT NOT NULL,
		note          TEXT NOT NULL DEFAULT '',
		color         TEXT NOT NULL DEFAULT 'amber',
		created_at    TEXT NOT NULL
	);`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_map_annotations_key ON map_annotations(demo_id, round_number, callout_id);`,
	`CREATE INDEX IF NOT EXISTS idx_map_annotations_demo ON map_annotations(demo_id);`,
}

var v8SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS match_purchases (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		demo_id       INTEGER NOT NULL,
		round_number  INTEGER NOT NULL,
		tick          INTEGER NOT NULL,
		time_sec      REAL NOT NULL DEFAULT 0,
		steam_id64    TEXT NOT NULL,
		player_name   TEXT NOT NULL,
		weapon        TEXT NOT NULL,
		cost          INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_match_purchases_demo_round ON match_purchases(demo_id, round_number, tick);`,
}

// v11SchemaStatements stores per-map lower-level alignment so a hand-tuned
// overlay survives restarts and demo switches.
var v11SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS map_view_prefs (
		map_name TEXT PRIMARY KEY,
		lower_offset_x REAL NOT NULL DEFAULT 0,
		lower_offset_y REAL NOT NULL DEFAULT 0,
		lower_opacity REAL NOT NULL DEFAULT 1.0,
		updated_at TEXT NOT NULL
	)`,
}

// v12SchemaStatements persists the per-kill geometry annotations computed by
// internal/killctx so they can be queried from SQL and exported without
// re-reading the replay.
var v12SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS kill_context (
		demo_id INTEGER NOT NULL,
		round_number INTEGER NOT NULL,
		tick INTEGER NOT NULL,
		time_sec REAL NOT NULL DEFAULT 0,
		killer_slot INTEGER NOT NULL DEFAULT 0,
		victim_slot INTEGER NOT NULL DEFAULT 0,
		killer_name TEXT NOT NULL DEFAULT '',
		victim_name TEXT NOT NULL DEFAULT '',
		weapon TEXT NOT NULL DEFAULT '',
		headshot INTEGER NOT NULL DEFAULT 0,
		distance REAL NOT NULL DEFAULT 0,
		visible INTEGER NOT NULL DEFAULT 0,
		geometry_checked INTEGER NOT NULL DEFAULT 0,
		smoke_blocked INTEGER NOT NULL DEFAULT 0,
		optics_checked INTEGER NOT NULL DEFAULT 0,
		aim_off_deg REAL NOT NULL DEFAULT 0,
		in_fov INTEGER NOT NULL DEFAULT 0,
		victim_sees_killer INTEGER NOT NULL DEFAULT 0,
		attacker_blind INTEGER NOT NULL DEFAULT 0,
		victim_blind INTEGER NOT NULL DEFAULT 0,
		attacker_zoom INTEGER NOT NULL DEFAULT 0,
		victim_zoom INTEGER NOT NULL DEFAULT 0,
		killer_x REAL NOT NULL DEFAULT 0,
		killer_y REAL NOT NULL DEFAULT 0,
		killer_z REAL NOT NULL DEFAULT 0,
		victim_x REAL NOT NULL DEFAULT 0,
		victim_y REAL NOT NULL DEFAULT 0,
		victim_z REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (demo_id, round_number, tick, victim_slot)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kill_context_demo ON kill_context(demo_id, round_number)`,
	`CREATE INDEX IF NOT EXISTS idx_kill_context_smoke ON kill_context(demo_id, smoke_blocked)`,
}

var v10SchemaStatements = []string{
	`CREATE VIRTUAL TABLE IF NOT EXISTS match_search USING fts5(
		demo_id UNINDEXED,
		players,
		teams,
		map_name,
		note,
		tokenize = 'unicode61'
	);`,
}

var v9SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS map_strokes (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		demo_id       INTEGER NOT NULL,
		round_number  INTEGER NOT NULL DEFAULT 0,
		color         TEXT NOT NULL DEFAULT 'amber',
		points_json   TEXT NOT NULL,
		created_at    TEXT NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_map_strokes_demo_round ON map_strokes(demo_id, round_number);`,
}
