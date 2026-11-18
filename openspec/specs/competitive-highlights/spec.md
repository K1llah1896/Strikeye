# competitive-highlights Specification

## Purpose

Detects competitive CS2 highlight clips and assigns deduplicated tactical and skill tags aligned with legacy competitive thresholds while excluding entertainment, fail, and meme tags.

## Requirements

### Requirement: Competitive highlight clip detection
The highlight detector SHALL identify competitive highlight clips across multi-kills (2K through 5K ACE), 1vN/2vN clutches, opening kills, knife/backstab kills, extreme or ninja defuses, and legacy-aligned weapon skill events (using `1500.0` units for long-range sniper/Deagle kills) within each official round.

#### Scenario: Multi-kill and ACE clip grouped with anchor ticks
- **WHEN** a player achieves 2 or more competitive kills within a round's rapid-kill window
- **THEN** a `highlight_clips` record is generated with `kill_count`, `anchor_ticks_json`, `start_tick`, `end_tick`, and `importance_score`

### Requirement: Priority-based tag deduplication
The highlight detector SHALL apply legacy coverage rules so broader tags are suppressed when a more specific tag is present on the same clip, preserving deterministic tag ordering without duplicates.

#### Scenario: Specific tag suppresses broader parent tag
- **WHEN** a highlight clip qualifies for both `爆头` and `💥 颗秒` (or `🙈 盲狙` and `✈️ 飞天盲狙`)
- **THEN** only the more specific tag is retained in `tags_json`

### Requirement: Exclusion of fail and meme tags
The highlight detector SHALL NOT emit fail-death clips, `下饭` tags, match-level meme tags (`211`, `o`, `i`, `z`), or AI roast metadata.

#### Scenario: Low-K/D or fail-death round produces no meme or fail clips
- **WHEN** a player finishes a match with 0–2 kills or dies under legacy fail conditions
- **THEN** no fail-type `highlight_clips` rows and no `211`/`o`/`i`/`z` tags are created
