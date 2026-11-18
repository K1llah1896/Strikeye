# radar-engine Specification

## Purpose

Provides pure-Go 6-axis CS Data Radar metric derivation, neon-styled 2560x1440 static card generation, and 4.0-second intro animation encoding with portrait and team-logo regeneration lifecycle.

## Requirements

### Requirement: Six-Axis Radar Metric Derivation and Normalization
The system SHALL derive six clockwise radar dimensions for each player (`KPR` full-scale `0.85`, `Surviving` full-scale `0.44`, `ADR` full-scale `85.0`, `KAST` full-scale `0.78`, `Multi-kill` full-scale `8` rounds, and `Rating` full-scale `1.30`), normalize each value as `raw / full_scale` clamped to `[0.0, 1.6]`, and compute the match-wide average across all players for each dimension.

#### Scenario: High-performing player exceeds full-scale ring up to ceiling
- **WHEN** a player achieves `ADR = 170.0` (2.0x full-scale) and `MultiKillRounds = 10` (1.25x full-scale)
- **THEN** the normalized ADR is clamped to `1.6` and the normalized Multi-kill is `1.25`, while match average uses the un-clamped or normalized player averages consistently across all six axes

#### Scenario: Zero-round or missing player stats handled safely
- **WHEN** a player has `0` rounds played
- **THEN** derived ratios (`KPR`, `Surviving`, `KAST`) evaluate to `0.0` without division-by-zero errors

### Requirement: Static 2560x1440 Radar Card Rendering
The system SHALL render a 2560x1440 PNG radar card containing the neon grid, `STRIKEYE` / `CNCS` watermark, blue full-scale outer ring, always-visible red match-average hexagon, team-colored player polygon (CT cyan-blue or T orange-red), Chinese/English dimension labels, and right-side circular 640px portrait with optional 1.8x radial-feathered team logo backdrop.

#### Scenario: Card rendered without custom portrait
- **WHEN** a player has no custom portrait image configured
- **THEN** the card renders a 2560x1440 PNG with an initial-letter placeholder inside the gold portrait ring and persists the card metadata in `radar_cards`

#### Scenario: Card rendered with custom portrait and team logo
- **WHEN** both a custom portrait and a team logo are provided for a card
- **THEN** the team logo is rendered as a 1.8x circular radial-feathered translucent backdrop behind the circular portrait

### Requirement: Four-Second Monotonic Intro Animation
The system SHALL render a 4.0-second 24fps (96-frame) intro animation scaled to 1920x1080 H.264 MP4 using a monotonic scale curve: ease-in (`0.30` to `0.55`) over `0%-30%`, ease-out-cubic (`0.55` to `1.00`) over `30%-60%`, and locked at `1.00` over `60%-100%` with zero overshoot or bounce, while keeping the red match-average hexagon visible from frame 1.

#### Scenario: Animation scale curve monotonicity and freeze
- **WHEN** evaluating the polygon scale factor across normalized time `t` from `0.0` to `1.0`
- **THEN** the scale is monotonically non-decreasing, never exceeds `1.0`, equals `0.30` at `t=0`, `0.55` at `t=0.30`, and `1.00` for all `t >= 0.60`

#### Scenario: Appearance update triggers animation regeneration or cleanup
- **WHEN** a card's portrait or team logo is updated or cleared after an animation already exists
- **THEN** the system regenerates the animation MP4 using the updated appearance when FFmpeg is available, or removes the stale animation file and clears `anim_video_path` when FFmpeg is unavailable
