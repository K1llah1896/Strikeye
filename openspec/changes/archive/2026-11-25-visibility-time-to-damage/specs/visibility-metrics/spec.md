# Spec Delta

## Purpose

利用本地缓存的地图几何数据计算“首次伤害前持续可见时长”（TTD），为对枪效率分析提供视线依据，并在几何缺失时优雅降级。

## ADDED Requirements

### Requirement: Visibility sampling
The parser SHALL sample alive player eye positions at the replay sampling rate while a round is active.

#### Scenario: Ring buffer window
- **WHEN** a round runs longer than five seconds
- **THEN** samples older than five seconds are discarded before the next damage event is evaluated

#### Scenario: Dead players excluded
- **WHEN** a player is dead
- **THEN** no visibility sample is recorded for them

### Requirement: Time-to-damage measurement
The parser SHALL record at most one TTD sample per round, attacker, and victim pair, measured as the continuous visible run before the first damage.

#### Scenario: Victim exposed briefly
- **WHEN** the victim was visible for 200 ms before the first hit
- **THEN** the sample is approximately 200 ms

#### Scenario: No line of sight
- **WHEN** the sight line was blocked for the whole window (for example a wallbang)
- **THEN** no sample is recorded for that pair

#### Scenario: Same team or self damage
- **WHEN** attacker and victim are the same player or share a team
- **THEN** no sample is recorded

### Requirement: Geometry-optional parsing
Parsing SHALL succeed when map geometry is absent.

#### Scenario: Missing geometry file
- **WHEN** `<DataRoot>/cache/map_geometry/<map>.tri` does not exist
- **THEN** the demo parses normally and TTD samples remain zero

### Requirement: TTD persistence and surfacing
The system SHALL persist average TTD and sample count per player and expose them in analysis and CSV export.

#### Scenario: Storage
- **WHEN** metrics include TTD samples
- **THEN** `match_players.ttd_avg_ms` and `ttd_samples` hold the average and count

#### Scenario: CSV export
- **WHEN** players are exported to CSV
- **THEN** the header and rows include `ttd_avg_ms` and `ttd_samples`

#### Scenario: Analysis drawer
- **WHEN** a player with TTD samples is expanded
- **THEN** the drawer shows the average TTD and sample count
