# Spec Delta

## Purpose

从已有的伤害与击杀事件推导每位选手的对枪用时（首次命中到击杀），持久化并在分析界面与导出中呈现，作为 TTD/TTK 能力的第一阶段。

## ADDED Requirements

### Requirement: Parser computes duel timing
The parser SHALL compute, per player, the average time from that player's first damage on a victim to the killing blow, using the demo tick rate.

#### Scenario: Damage then kill
- **WHEN** a player damages a victim at tick 100 and kills them at tick 164 in a 64-tick demo
- **THEN** that kill contributes 1000 ms to the player's average

#### Scenario: Engagement window
- **WHEN** more than 3 seconds pass between hits on the same victim
- **THEN** the timing restarts from the latest hit instead of spanning the gap

#### Scenario: Death resets the engagement
- **WHEN** a victim dies
- **THEN** any recorded first-hit for that victim is discarded

#### Scenario: Team and self damage are ignored
- **WHEN** the only damage on a victim comes from a teammate or the victim
- **THEN** no timing sample is recorded

### Requirement: Duel timing persists with match players
The system SHALL store each player's average TTK in milliseconds and the number of contributing kills, and SHALL read back zero for legacy rows.

#### Scenario: Legacy database upgrade
- **WHEN** a database created before this change is opened
- **THEN** it migrates to schema v4 and existing players report `ttk_samples` 0

#### Scenario: Round trip
- **WHEN** match players are written and listed
- **THEN** `ttk_avg_ms` and `ttk_samples` survive unchanged

### Requirement: Duel timing is visible
The system SHALL show average TTK and sample count in the player detail drawer and include both fields in CSV and JSON exports.

#### Scenario: Drawer shows timing
- **WHEN** the operator expands a player row
- **THEN** the drawer shows `平均 TTK: <ms>ms (<n> 次对枪)`

#### Scenario: Export includes timing
- **WHEN** a match is exported to CSV
- **THEN** the players file contains `ttk_avg_ms` and `ttk_samples` columns
