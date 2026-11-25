# Proposal: Visibility-Aware Time to Damage

## Why

TTK alone says how fast a duel ended, not how fast the attacker converted
vision into damage. With the map geometry cache in place, Strikeye can measure
TTD: how long the attacker had line of sight before the first damage of a duel.
This is the visibility-backed half of the report's TTD/TTK gap and the first
consumer of `internal/mapgeom`.

## What Changes

- Parser keeps a 5 s / 32 Hz ring buffer of alive eye positions and, on the
  first damage of each attacker→victim duel, traces line of sight backwards to
  find the continuous visible run.
- New `TTDSample` flows into `ComputeMetrics`, adding `ttd_avg_ms` and
  `ttd_samples` per player (migration v5), side-weighted like other metrics.
- Geometry is optional: without `<DataRoot>/cache/map_geometry/<map>.tri` the
  parse still succeeds and TTD samples stay zero.
- Analysis drawer and CSV export surface the new metric.

## Impact

- Affected specs: `visibility-metrics` (new)
- Affected code: `internal/parser` (visibility.go, single_pass.go, metrics.go),
  `internal/storage` (migration v5, analysis repo), `internal/export`,
  `internal/ui/views/analysis`
- Requires `maps-fetch` for real values; no new dependencies.
