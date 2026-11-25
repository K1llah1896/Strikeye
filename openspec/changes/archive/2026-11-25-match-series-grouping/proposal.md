# Proposal: Match Series Grouping

## Why

Match Library treats every demo as an isolated card. A best-of-3 arrives as three
separate demos, so users cannot see the series score, the map order, or which map
decided the series. Competitors (CS:DM, cs2-meta-engine) group maps into a series;
our research report lists "系列赛/BO3" as a P1 gap.

## What Changes

- New pure package `internal/series` that groups parsed demos by canonical team
  pair + match day, orients scores to a stable team A/B even when the demo's
  `team_a`/`team_b` columns swap, and computes map wins.
- `strikeye-cli series` prints the grouped series as JSON for scripting and
  verification.
- Match Library gains a "按系列赛分组" toggle: multi-map groups render a series
  header (`FURIA 2-1 Falcons · 3 图`) above their cards; single-map demos stay
  as standalone cards.

## Impact

- Affected specs: `match-library-and-analysis-ui`
- Affected code: `internal/series` (new), `internal/ui/views/library`,
  `cmd/strikeye-cli`
- No storage migration: series are derived from existing demo rows.
