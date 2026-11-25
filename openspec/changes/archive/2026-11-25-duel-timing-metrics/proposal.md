# Proposal: Duel Timing Metrics

## Why

The analysis UI reports damage, kills, and trade rates but not how fast duels are
won. Coaches use time-to-kill to separate players who trade shots slowly from
players who finish instantly. The research report lists TTD/TTK as a P1 gap
(CS:DM and cs-demo-analyzer both expose duel timing).

Full TTD needs line-of-sight ray tracing against map geometry, which this project
does not ship yet. TTK — first damage on a victim to the killing blow — is
derivable from data the parser already collects, so it is delivered now as the
first half of that gap.

## What Changes

- Parser computes per-player average TTK (ms) and sample count from
  `PlayerHurt` + `Kill` events, within a 3-second engagement window and reset on
  death.
- Schema v4 adds `ttk_avg_ms` and `ttk_samples` to `match_players`.
- Match Analysis player drawer shows average TTK with sample count.
- CSV/JSON export gains the two fields.

## Impact

- Affected specs: `duel-timing-metrics` (new)
- Affected code: `internal/parser/metrics.go`, `internal/storage` (migration +
  repo), `internal/ui/views/analysis`, `internal/export`
- Additive migration; old rows read as 0 samples.
