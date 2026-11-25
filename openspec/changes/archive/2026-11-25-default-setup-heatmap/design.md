# Design: Default Setup Heatmap

## Sampling

`targetTick = FreezeEndTick + 20 × tickRate`. The nearest replay frame to that
tick is used (binary search + neighbour comparison), then each alive player
becomes one weighted point. At 32 Hz replay the frame is within ~16 ms of the
target. When `FreezeEndTick` is missing, the round's first frame is the anchor.

One point per player per round keeps round weighting uniform: a 30-round match
contributes at most 150 setup points regardless of round length, so KDE density
reflects "how often this spot is occupied at 0:20", not frame count.

## Reuse

No new parser, cache, or storage code: the heatmap pipeline already lazily loads
all round replays from `.cs2rpl` and renders KDE from `HeatmapPoint` slices. The
new dimension is a new branch in `BuildHeatmapPointsMulti`.

## Filters

Team filter uses the replay pose team id (1 = T, 2 = CT), which is independent of
the demo's team_a/team_b mapping — the same convention as the position mode.

## Limitations

Setup zones are not labeled with callouts (no map callout dataset yet); the
result is a spatial density map. Zone naming can be layered on later without
changing this sampling contract.
