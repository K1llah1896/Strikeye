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

### Requirement: Library full-text index

系统 SHALL 维护 FTS5 全文索引：

- 迁移 v10 SHALL 建立 `match_search(demo_id UNINDEXED, players, teams, map_name, note)` 使用 `unicode61` 分词。
- 重建 SHALL 从 `demos`（队伍、地图、备注）、`match_players`（选手名）与 `map_annotations`（callout 与备注）汇总文本。
- `EnsureSearchIndex` SHALL 在索引行数与 demo 数不一致时重建，并报告是否重建。
- `Query(ctx, text, limit)` SHALL 对每个 token 做前缀匹配、按 `bm25` 升序返回、遵守 limit（默认 50、上限 500）。
- 查询构造 SHALL 只保留字母数字 token 并丢弃 FTS5 保留字，因此用户输入的操作符与引号 SHALL NOT 造成 SQL 语法错误。
- 空查询 SHALL 返回零结果而不访问数据库。

#### Scenario: Player prefix match
- **WHEN** the index contains player `KSCERATO` and the query is `ksce`
- **THEN** that demo is returned

#### Scenario: Operators cannot break the query
- **WHEN** the query is `NiKo" OR NEAR(`
- **THEN** the query succeeds and behaves like a two-token AND prefix search

#### Scenario: Stale index is repaired
- **WHEN** a demo is added after the last rebuild
- **THEN** `EnsureSearchIndex` reports a rebuild and the new demo becomes searchable

### Requirement: Large library pagination

系统 SHALL 支持大资料库分页读取：

- `ListPage(ctx, limit, offset)` SHALL 使用与 `List` 相同的排序，limit 上限 1000、默认 100，负 offset 归零。
- `CountDemos` SHALL 返回 demo 总数。
- 测试 SHALL 覆盖 2000 条 demo 下的分页、索引重建与查询耗时上限（分页 <200ms、FTS 查询 <100ms）。

#### Scenario: Page offset beyond the first page
- **WHEN** requesting `ListPage(50, 950)` on a 2000-row library
- **THEN** 50 rows are returned within the time budget

### Requirement: Per-map view preference storage

数据库 SHALL 持久化每张地图的 2D 下层对齐参数：

- 迁移 SHALL 新增 `map_view_prefs` 表，主键为地图名，字段包含 `lower_offset_x`、`lower_offset_y`、`lower_opacity`、`updated_at`。
- 读取缺失行时 SHALL 返回偏移 0、透明度 1 的默认值而不报错。
- 写入 SHALL 把偏移钳制在 ±512、透明度钳制在 0.1–1.0，并要求地图名非空。
- 列表 SHALL 按地图名排序。

#### Scenario: Defaults for an unseen map
- **WHEN** preferences are read for a map that was never configured
- **THEN** offsets are zero and opacity is 1

#### Scenario: Out-of-range values are clamped
- **WHEN** a caller stores offset 9999 and opacity 7
- **THEN** the stored row reads back as offset 512 and opacity 1

### Requirement: CJK substring search fallback

资料库全文检索 SHALL 支持中文子串查询：

- 查询含非 ASCII 字符时，除 FTS5 前缀匹配外 SHALL 额外执行大小写不敏感的子串匹配（`players`/`teams`/`map_name`/`note`）。
- 两路结果 SHALL 按 `demo_id` 去重合并，FTS 命中优先保留其 bm25 排序。
- 纯 ASCII 查询 SHALL 只走 FTS5，不改变既有排序。
- 空查询 SHALL 返回空结果且不执行 SQL。

#### Scenario: Search a Chinese note

- **WHEN** 备注为 `LongA 默认双架`，用户搜索「双架」
- **THEN** 该比赛出现在结果中，`note` 字段包含完整备注

#### Scenario: Absent Chinese term

- **WHEN** 用户搜索一个中文词库里不存在
- **THEN** 返回 0 条，不报错

#### Scenario: Latin query unchanged

- **WHEN** 用户搜索 `furia`
- **THEN** 走 FTS5 前缀匹配，结果按 bm25 排序
