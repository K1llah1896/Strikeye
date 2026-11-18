# Spec Delta

## ADDED Requirements

### Requirement: Flashbang utility tracking
During single-pass parsing, the parser SHALL handle `PlayerFlashed` events to record each player's `EnemiesFlashed`, `FlashAssists`, and `TotalFlashSec` across official rounds, excluding self-flashes and friendly-fire flashes from enemy blind totals.

#### Scenario: Enemy flashed and flash-assisted kill
- **WHEN** a player throws a flashbang that blinds an enemy for 2.4 seconds and a teammate kills that blinded enemy
- **THEN** the thrower's `EnemiesFlashed` increments by 1, `TotalFlashSec` increases by 2.4, and `FlashAssists` increments by 1

### Requirement: CT and T side-split statistics
The parser SHALL compute per-player metric aggregates separately for `CT` rounds and `T` rounds in addition to match totals, using each round's resolved `team_a_side` and `team_b_side`.

#### Scenario: Side-split aggregation across halftime
- **WHEN** a player plays 12 rounds on CT and 10 rounds on T before the match ends
- **THEN** the parser outputs distinct `CT` and `T` stat rows whose round counts and combat totals sum to the match total

### Requirement: Granular 1v1 to 1v5 clutch breakdown
The parser SHALL persist per-player clutch attempts and wins broken down by opponent count (`1v1`, `1v2`, `1v3`, `1v4`, `1v5`) alongside total clutch attempts and wins.

#### Scenario: Winning a 1v2 clutch
- **WHEN** a player is the last survivor against two living opponents and wins the round
- **THEN** both total `ClutchWins` and the `1v2` clutch win counter increment by 1

### Requirement: Round Swing and Eco-Adjusted rating
The parser SHALL compute each player's `Round Swing` (win-probability delta per kill and clutch based on alive-player differential and bomb plant state) and `Eco-Adjusted` Rating/ADR weighted by round economy disparity, without replacing the baseline `Strikeye Rating`.

#### Scenario: High-leverage opening kill vs low-leverage anti-eco kill
- **WHEN** a player secures a 5v5 opening kill in a full-buy round versus a 5v1 kill against an eco round
- **THEN** the 5v5 full-buy kill contributes a substantially larger `Round Swing` and `Eco-Adjusted` impact than the 5v1 anti-eco kill
