# Spec Delta

## Purpose

为每位选手建立按 SteamID64 持久化的档案（显示名覆盖、侦察备注、标签），并在 CLI 与比赛库中读取、编辑与检索这些档案。

## ADDED Requirements

### Requirement: Persistent player profiles
The system SHALL store one profile per SteamID64 with an optional display-name override, a free-form note, and a normalized tag list, and SHALL preserve parsed match_players rows unchanged.

#### Scenario: Tags are normalized
- **WHEN** a profile is saved with tags `["Star", " star ", "", "awp"]`
- **THEN** the stored list contains exactly `Star` and `awp`

#### Scenario: Missing profile is not an error
- **WHEN** the UI or CLI looks up a SteamID64 with no stored profile
- **THEN** it proceeds with an empty profile and `has_profile` false

#### Scenario: Migration upgrades existing databases
- **WHEN** an existing v2 database is opened with the new migrations
- **THEN** `player_profiles` is created, existing demo/player rows are untouched, and a pre-migration snapshot is written

### Requirement: Player dossier CLI
The system SHALL expose a player dossier through `strikeye-cli profile` that combines cross-match aggregates with the stored profile.

#### Scenario: Inspect a player
- **WHEN** the operator runs `strikeye-cli profile --steamid <id64>`
- **THEN** the command prints aggregate totals plus the profile and whether it exists

#### Scenario: Save profile fields
- **WHEN** the operator passes `--name`, `--note`, or `--tags`
- **THEN** the command persists only the supplied fields and prints the saved profile

#### Scenario: Player without matches
- **WHEN** the SteamID64 appears in no parsed match
- **THEN** the inspect command exits non-zero with a "no matches" error

### Requirement: Player tags in Match Library
The system SHALL surface stored player tags on match cards and include them in keyword filtering.

#### Scenario: Cards badge tags
- **WHEN** a demo's players have stored tags and the library refreshes
- **THEN** the card shows up to three tag pills

#### Scenario: Keyword matches a tag
- **WHEN** the operator searches a keyword equal to a stored tag
- **THEN** the library lists demos whose players carry that tag
