# Spec Delta

## Purpose

在 2D 热力分析中增加“开局 20 秒默认站位”维度，用于观察双方默认布防与分推习惯。

## ADDED Requirements

### Requirement: Default setup heatmap dimension
The system SHALL offer a `setup` heatmap dimension that samples one replay frame per round near freeze-end + 20 seconds and includes only players alive at that frame.

#### Scenario: Twenty-second sampling
- **WHEN** a round has freeze-end tick 1000 and the demo runs at 32 tick
- **THEN** the sampled frame is the one nearest tick 1640

#### Scenario: Dead players excluded
- **WHEN** a player is dead at the sampled frame
- **THEN** that player contributes no setup point

#### Scenario: Team filter uses replay side
- **WHEN** the operator filters the setup heatmap to CT
- **THEN** only players whose replay pose team is CT are sampled

#### Scenario: Filters compose
- **WHEN** a player and a round type are selected together with `setup`
- **THEN** only that player's setup positions in matching rounds are sampled
