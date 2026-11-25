# metric-snapshots Specification

## Purpose

用黄金指标快照锁定解析器与指标口径，使解析、指标或存储改动无法在不被发现的情况下改变用户看到的数字。

## Requirements

### Requirement: Golden metric snapshots

系统 SHALL 为测试语料维护黄金指标快照，`corpus.BuildWithGeometry(ctx, demoPath, workDir, geometryDir)` SHALL 在临时 DataRoot 内以 `Force` 解析一次 demo 并取指标。

- 快照 SHALL 包含身份（source/sha256/size_bytes）、比赛（map/队伍/比分/tick_rate/parse_status）、结构计数（round_count/event_count/highlights/replay_* /purchases）与 `input_capability`。
- `players[]` SHALL 按选手名排序，含 kills/deaths/assists/headshots/adr/kast/rating/ttk_avg_ms/ttk_samples/ttd_avg_ms/ttd_samples/opening_kills。
- `Diff(want, got)` SHALL 报告字符串与整数差异，浮点指标 SHALL 使用 0.11 的容差。

#### Scenario: Metrics unchanged
- **WHEN** the corpus is verified against committed golden snapshots
- **THEN** every field matches and the command exits zero

#### Scenario: Divergence is reported
- **WHEN** a score, count or player metric changes beyond tolerance
- **THEN** the command lists each differing field and exits non-zero

#### Scenario: Geometry enables TTD coverage
- **WHEN** `geometryDir` points at the map geometry cache
- **THEN** the `.tri` files are copied into `<workDir>/cache/map_geometry` and the snapshot records non-zero `ttd_samples`

### Requirement: Corpus command and opt-in test

CLI `corpus` SHALL 支持 `--dir`、`--golden`、`--demo`、`--update`、`--geometry-dir`、`--work-dir`，比对失败时 SHALL 打印差异并返回非零退出码。解析 fixture 的 Go 测试 SHALL 仅在 `STRIKEYE_CORPUS=1` 时执行，常规 `go test` SHALL 跳过。

#### Scenario: Update rewrites snapshots
- **WHEN** `corpus --update` runs
- **THEN** each fixture's JSON snapshot is rewritten and the command exits zero

#### Scenario: Default test run skips fixtures
- **WHEN** `go test ./internal/corpus/` runs without `STRIKEYE_CORPUS`
- **THEN** the fixture test is skipped while unit tests still run

### Requirement: Multi-source regression corpus

系统 SHALL 支持对多种来源的 demo 建立回归语料：

- 语料目录 SHALL 可配置（`--dir`，默认 `tests`），并只处理 `.dem` 文件。
- 每只 demo SHALL 对应一个 `<name>.json` 快照，路径可配置（`--golden`，默认 `tests/fixtures/golden`）。
- `--demo` SHALL 把运行范围限制到单只 fixture，便于定位。
- 快照 SHALL 记录 demo 的 `sha256`，fixture 被替换时比对 SHALL 失败而不是静默通过。

#### Scenario: Single fixture run
- **WHEN** `--demo furia-vs-falcons-m2-dust2.dem` is passed
- **THEN** only that fixture is parsed and reported
