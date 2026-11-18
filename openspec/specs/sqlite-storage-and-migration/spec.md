# sqlite-storage-and-migration Specification

## Purpose

Provides durable SQLite WAL persistence, versioned schema migrations, pre-migration snapshots, idempotent entity repositories, and lossless import of legacy CS2-insight-agent assets into Strikeye.

## Requirements

### Requirement: SQLite WAL mode and foreign key enforcement
The storage layer SHALL configure every database connection with WAL journal mode, normal synchronization, and foreign key enforcement before executing schema migrations or queries.

#### Scenario: Database initialization
- **WHEN** the storage engine opens `strikeye.db`
- **THEN** WAL mode and foreign key constraints are active and all core tables (`demos`, `match_players`, `match_rounds`, `match_events`, `highlight_clips`, `media_videos`, `render_tasks`, `radar_cards`, `montage_projects`, `montage_exports`) exist with their indexes

### Requirement: Automatic pre-migration database snapshot
The storage layer SHALL create a consistent backup snapshot in `<DataRoot>/backup/db_snapshots/` before applying pending schema migrations to an existing non-empty database.

#### Scenario: Schema upgrade on existing database
- **WHEN** an existing `strikeye.db` at an older schema version is opened
- **THEN** a snapshot file is written to `backup/db_snapshots/` before any migration DDL executes

### Requirement: Idempotent demo and render task deduplication
The storage layer SHALL enforce uniqueness on `demos.content_sha256` and `render_tasks.task_uid` so repeated registration of the same demo content or identical render task parameters updates or returns the existing record without creating duplicates.

#### Scenario: Duplicate demo registration by content hash
- **WHEN** a demo file with an already stored `content_sha256` is registered again
- **THEN** the existing demo record is returned or updated in place without inserting a second row

#### Scenario: Duplicate render task enqueue
- **WHEN** a render task with an existing `task_uid` is enqueued
- **THEN** no duplicate `render_tasks` row is created and the existing task identifier is returned

### Requirement: Consistent media replacement on re-render
The storage layer SHALL replace or update existing `media_videos` metadata when a matching scope, demo, round, player, and perspective is re-rendered, preventing orphan index entries.

#### Scenario: Re-rendering an existing clip or POV
- **WHEN** a verified video for an already indexed `file_path` or replacement target is saved
- **THEN** the existing `media_videos` entry is updated cleanly without duplicate rows

### Requirement: Lossless legacy asset migration
The legacy migrator SHALL detect existing `CS2-insight-agent` SQLite data (`cs2-insight.db` and legacy fallbacks), back up the legacy database into `<DataRoot>/backup/` without modifying the source, and import demo metadata, user remarks, and verified existing MP4 media records into Strikeye tables.

#### Scenario: Importing legacy database without mutating source
- **WHEN** legacy migration runs against a directory containing `cs2-insight.db` with `demo_files` and `recorded_clips`
- **THEN** the original legacy database is copied to `<DataRoot>/backup/legacy_import_<timestamp>/`, imported demos preserve `display_name` and `remark`, and existing verified MP4 paths are registered in `media_videos`

#### Scenario: Missing legacy MP4 file during import
- **WHEN** a legacy `recorded_clips` row points to an MP4 file that no longer exists on disk
- **THEN** the missing file is skipped without failing migration of valid records
