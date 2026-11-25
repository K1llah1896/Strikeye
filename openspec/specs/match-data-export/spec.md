# match-data-export Specification

## Purpose
把已解析的 CS2 比赛数据导出为可移植的 CSV/JSON 文档，覆盖选手、回合、事件与半场拆分，并支持按 SteamID64 跨比赛聚合选手表现，便于 Excel、SQL 与第三方分析工具消费。

## Requirements

### Requirement: Per-match export
The system SHALL export a parsed match's players, rounds, events, side splits, and stored kill-context rows to CSV files and a combined JSON document, and SHALL accept `csv`, `json`, `xlsx`, `html`, `both` (csv+json) and `all` (every format) as `--format` values, including comma-separated combinations.

#### Scenario: Export both formats
- **WHEN** the operator runs `strikeye-cli export --demo-id <id> --format both --out <dir>`
- **THEN** the directory receives `match_<id>_players.csv`, `match_<id>_rounds.csv`, `match_<id>_events.csv`, `match_<id>_sides.csv`, `match_<id>_kills.csv`, and `match_<id>.json`, and their paths are printed

#### Scenario: Export every format
- **WHEN** the operator passes `--format all`
- **THEN** CSV files, `match_<id>.xlsx`, `match_<id>.html` and `match_<id>.json` are all written

#### Scenario: Export rejects unknown demo
- **WHEN** the requested demo ID does not exist
- **THEN** the command exits non-zero with an error and writes no files

#### Scenario: Export rejects unknown format
- **WHEN** the operator passes `--format pdf`
- **THEN** the command exits non-zero and names the supported values

#### Scenario: CSV headers are stable
- **WHEN** two exports of the same match are produced
- **THEN** each CSV file starts with the same header row in the same column order

### Requirement: Cross-match player aggregation
The system SHALL aggregate a player's matches by SteamID64 into per-match rows plus totals.

#### Scenario: Aggregate totals use summed rounds
- **WHEN** the operator exports `--steamid <id>`
- **THEN** `player_<id>_matches.csv` and `.json` contain one row per parsed match and totals whose ADR/KAST/HS%/Rating are computed from summed damage, rounds, and weighted rating

#### Scenario: Player without matches
- **WHEN** the SteamID64 appears in no parsed match
- **THEN** the command exits non-zero with a "no matches" error

### Requirement: HTML match report
The system SHALL export a match as one self-contained HTML document.

#### Scenario: Report content
- **WHEN** a match is exported with the `html` format
- **THEN** `match_<id>.html` contains the team names, score, map, date, per-side scoreboards, and the round list

#### Scenario: No external assets
- **WHEN** the report is generated
- **THEN** it references no scripts, images, stylesheets, or remote URLs

#### Scenario: Escaped dynamic text
- **WHEN** a player name or team name contains HTML metacharacters
- **THEN** the characters are escaped in the output

#### Scenario: Combined formats
- **WHEN** the operator passes `csv,html` or `json,html`
- **THEN** each requested format is written and unsupported names still fail with an error

### Requirement: XLSX workbook export

系统 SHALL 支持 `--format xlsx` 导出：

- 比赛包 SHALL 生成 `match_<demo_id>.xlsx`，包含 `players`、`rounds`、`events`、`sides` 四张工作表。
- 选手聚合 SHALL 生成 `player_<steam_id64>_matches.xlsx`。
- 每张工作表 SHALL 带一行加粗表头，列定义与同内容 CSV 完全一致。
- 数值字段 SHALL 写成数字单元格而不是文本，以便表格软件直接求和、求平均。
- 工作簿 SHALL 是合法的 OOXML 包，至少包含 `[Content_Types].xml`、`_rels/.rels`、`xl/workbook.xml`、`xl/_rels/workbook.xml.rels`、`xl/styles.xml` 与每张表的 `xl/worksheets/sheetN.xml`。

#### Scenario: Export a match to XLSX

- **WHEN** 用户执行 `strikeye-cli export --demo-id 2 --format xlsx`
- **THEN** 输出目录中出现 `match_2.xlsx`，四张工作表可按名字访问，`kills` 等列是数字

#### Scenario: Numeric columns stay numeric

- **WHEN** 某选手 ADR 为 88.5
- **THEN** 单元格值是数字 88.5，而不是字符串 "88.5"

#### Scenario: CSV and XLSX agree on columns

- **WHEN** 同一比赛分别以 CSV 与 XLSX 导出
- **THEN** `players` 表的列名与顺序和 `match_<id>_players.csv` 表头一致

### Requirement: Batch export across matches

`strikeye-cli export --all` SHALL export every parsed match in one run:

- 每场比赛 SHALL 写入独立目录 `<out>/demo_<id>/`，文件名沿用单场导出的 `match_<id>_*` 规则。
- SHALL 支持 `--team`（大小写不敏感，命中 A 或 B 任一侧）、`--map`（大小写不敏感）与 `--limit`（按 id 升序最多 N 场）。
- 只导出 `parse_status = parsed` 的比赛。
- 批末 SHALL 写 `<out>/export_index.json`，包含格式化后的过滤条件、`exported`/`failed` 计数与每场的 `demo_id`、队伍、地图、目录、文件列表、错误。
- 单场失败 SHALL 记入索引并继续；全部失败时命令 SHALL 返回错误。
- `--all` SHALL 与 `--demo-id`、`--steamid` 互斥，同时给出时 SHALL 报错。

#### Scenario: Export every parsed match
- **WHEN** 库中有 2 场 parsed 与 1 场 pending，运行 `export --all --format all`
- **THEN** 生成 `demo_<id1>/` 与 `demo_<id2>/` 各 4 种格式文件，pending 场次被跳过，`export_index.json` 中 `exported=2`

#### Scenario: Filter by team
- **WHEN** 运行 `export --all --team navi` 且只有一场包含 NAVI
- **THEN** 只导出那一场，索引中 `filter` 含 `team=navi`

#### Scenario: Empty selection
- **WHEN** 过滤条件没有任何比赛命中
- **THEN** 命令返回 `no matches matched filter …` 错误且不写索引
