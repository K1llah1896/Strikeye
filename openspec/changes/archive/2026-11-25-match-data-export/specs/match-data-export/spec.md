# Spec Delta

## Purpose

把已解析的 CS2 比赛数据导出为可移植的 CSV/JSON 文档，覆盖选手、回合、事件与半场拆分，并支持按 SteamID64 跨比赛聚合选手表现，便于 Excel、SQL 与第三方分析工具消费。

## ADDED Requirements

### Requirement: Per-match export
The system SHALL export a parsed match's players, rounds, events, and side splits to CSV files and a combined JSON document.

#### Scenario: Export both formats
- **WHEN** the operator runs `strikeye-cli export --demo-id <id> --format both --out <dir>`
- **THEN** the directory receives `match_<id>_players.csv`, `match_<id>_rounds.csv`, `match_<id>_events.csv`, `match_<id>_sides.csv`, and `match_<id>.json`, and their paths are printed

#### Scenario: Export rejects unknown demo
- **WHEN** the requested demo ID does not exist
- **THEN** the command exits non-zero with an error and writes no files

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
