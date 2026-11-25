# Spec Delta

## Purpose

跨已解析 Demo 汇总同一队伍的地图战绩、回合得失与轮次加权阵容数据，为赛前侦察提供队伍级视图。

## ADDED Requirements

### Requirement: Team map aggregation
The system SHALL aggregate every parsed demo where the requested team name appears as either side, reporting each map's score, result, opponent, and total rounds.

#### Scenario: Team appears on side B
- **WHEN** a parsed demo lists the requested team as team B with score 16
- **THEN** the map result reports 16 as the team score and the opposing score as the opponent score

#### Scenario: Case and whitespace insensitive
- **WHEN** the operator requests ` furia ` and demos record `FURIA`
- **THEN** those demos are included

#### Scenario: Unparsed demos excluded
- **WHEN** a demo matching the team name is not parsed
- **THEN** it contributes no map or roster data

### Requirement: Round-weighted roster
The system SHALL report one roster row per SteamID64 that played for the team, with maps, rounds, kills, deaths, assists, and round-weighted ADR, KAST, and rating.

#### Scenario: Two maps of different length
- **WHEN** a player records a 16-round and a 30-round map
- **THEN** ADR is total damage divided by 46 rounds

#### Scenario: Opponent players excluded
- **WHEN** the requested team is team A in a demo
- **THEN** only team A player rows contribute to the roster

### Requirement: Team dossier CLI
The system SHALL expose the team dossier as JSON through `strikeye-cli team --name`.

#### Scenario: Missing name
- **WHEN** `--name` is empty
- **THEN** the command fails with a "team name is required" error

#### Scenario: No matching demos
- **WHEN** no parsed demo contains the name
- **THEN** the command succeeds with an empty map list and empty roster
