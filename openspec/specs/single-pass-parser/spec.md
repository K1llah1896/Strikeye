# single-pass-parser Specification

## Purpose

Executes a single-pass scan of CS2 demos to extract official rounds, team economy classifications, combat events, player statistics, and Strikeye Rating into SQLite.

## Requirements

### Requirement: Single-pass parsing and result reuse
The parser SHALL extract header metadata, player rosters, round windows, economy votes, combat events, player metrics, highlights, and replay/input caches in one `ParseToEnd()` pass, persisting results so subsequent reads reuse stored database records and cached artifacts without re-parsing the `.dem` file.

#### Scenario: Demo parsed once and reused
- **WHEN** a demo is parsed to completion and marked `parsed`
- **THEN** all match tables and cache references are persisted and subsequent queries read stored outputs without reopening the `.dem` file

#### Scenario: Failed parse records diagnostic state
- **WHEN** parsing fails due to a corrupt or unreadable demo stream
- **THEN** the demo record is updated with `parse_status = 'failed'` and a non-empty `parse_error` without leaving partial round or player rows

### Requirement: Official round window and halftime resolution
The parser SHALL filter out warmup and knife rounds, track `start_tick`, `freeze_end_tick`, and `end_tick` (including final EOF round settlement), detect halftime side swaps (`halftime_round`), and support MR12 regulation and overtime rounds.

#### Scenario: Warmup and knife rounds excluded
- **WHEN** a demo contains warmup or knife-round skirmishes before match start
- **THEN** only official competitive rounds are numbered and persisted in `match_rounds`

#### Scenario: Final round closed at demo EOF
- **WHEN** the demo stream ends after `RoundEnd` without a subsequent `RoundStart`
- **THEN** the final round's `end_tick` and scores are flushed accurately before finalizing match totals

### Requirement: Five-tier team economy classification
At each round's `freeze_end_tick`, the parser SHALL collect each player's equipment value, starting account, and round spend, classifying individual and team-voted economy into `pistol`, `full`, `force`, `semi`, or `eco` using the legacy voting rules.

#### Scenario: Pistol and eco round classification
- **WHEN** round 1 or the first post-halftime round starts, or a team's voted spend and equipment fall below `1000`
- **THEN** `team_a_economy_type` / `team_b_economy_type` are recorded as `pistol` or `eco` respectively

### Requirement: Combat metrics and Strikeye Rating parity
The parser SHALL compute capped damage (100 HP per victim per round, excluding team/self-damage), ADR, utility ADR, 5-second trade kills/deaths, KAST%, first kills/deaths, 1vN clutch attempts/wins, and `Strikeye Rating` matching `CS2-insight-agent-main` definitions without claiming HLTV official rating.

#### Scenario: Capped damage and 5-second trade attribution
- **WHEN** a victim with 35 HP remaining takes 108 raw damage and a teammate kills the attacker within 5 seconds
- **THEN** effective damage is capped at 35 and both the trade kill and traded death contribute to KAST%

### Requirement: Incremental parse progress reporting
The parser and agent parse worker SHALL report throttled stage and percentage progress (`0.0–1.0`) during demo scanning, highlight/metric computation, and SQLite persistence, publishing `parse.progress` IPC events with `demo_id`, `stage`, and `progress` to subscribed UI clients.

#### Scenario: Demo parsing emits progress updates
- **WHEN** `strikeye-agent` parses a pending `.dem` file
- **THEN** `parse.progress` events are emitted periodically with monotonically increasing progress values and descriptive stage labels before `parse.completed`
