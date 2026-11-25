# Spec Delta

## Purpose

把同一对阵、同一比赛日的多张地图 Demo 聚合为系列赛，并在比赛库中展示系列赛比分与地图顺序。

## ADDED Requirements

### Requirement: Multi-map series grouping
The system SHALL derive series from parsed demos sharing the same unordered team pair and match day, orienting scores to a stable team A/B, and SHALL expose them through the CLI and the Match Library.

#### Scenario: Swapped team columns still aggregate
- **WHEN** two parsed demos share the same teams and date but one lists the teams as A/B and the other as B/A
- **THEN** the series reports two maps with scores oriented to the same team and the correct map-win totals

#### Scenario: Single-map groups are not series
- **WHEN** only one parsed demo exists for a team pair and date
- **THEN** it does not appear in the series list and stays a standalone library card

#### Scenario: CLI lists series
- **WHEN** the operator runs `strikeye-cli series --data-root <dir>`
- **THEN** the command prints a JSON document whose `series` entries include teams, date, maps, and map-win totals

#### Scenario: Library groups cards by series
- **WHEN** the "按系列赛分组" toggle is enabled and a filtered result contains maps from one series
- **THEN** a header showing `TeamA <wins>-<wins> TeamB` and the map count is rendered above that series' cards
