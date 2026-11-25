# Proposal: Default Setup Heatmap

## Why

The 2D heatmap tab can map kills, deaths, firefights, and all-frame positions, but
it cannot answer the most common coaching question: "how does this team set up in
the first 20 seconds?" cs2-meta-engine ships CT setups and T default spread at
0:20 as its headline report; CS:DM offers per-phase heatmaps. The research report
lists map control and default setups as a P1 gap.

## What Changes

- New `setup` heatmap dimension that samples exactly one frame per round — the
  frame nearest to freeze-end + 20 s — and only alive players.
- Player, side, and round-type filters apply unchanged.
- Tick rate flows into the heatmap filter so the 20-second offset is accurate for
  both 64- and 128-tick demos.
- UI dimension selector and helper copy updated.

## Impact

- Affected specs: `match-library-and-analysis-ui` (MODIFIED/ADDED requirement)
- Affected code: `internal/ui/views/analysis`
- Read-only; no schema or parser changes.
