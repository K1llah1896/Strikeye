# Proposal: ui-design-review-polish-r2

## Why

A second-round visual design review of `1360x840` offscreen-rendered UI snapshots identified three remaining polish gaps: the `560x560` `2D Replay` canvas renders as a blank black square when no `CS2RPL01` frames exist despite having bundled map radar PNGs, the `Overview & Scoreboard` header row has a `6px` horizontal offset relative to player rows, and three local polish issues remain (`(unrendered)` English suffix in `Spectator`, permanently expanded manual `.dem` path bar in `Match Library`, and bottom-edge clipping of the `Settings & Tasks` empty queue message at `840px` height).

## What Changes

- **2D Replay Map Backdrop on Empty State (`internal/ui/components/radar_2d_canvas.go`)**:
  - Render the configured map radar background (or procedural tactical grid fallback) immediately when `ConfigureMap` is called so `2D Replay` displays the map radar even when no replay frames are present.
- **Pixel-Aligned Scoreboard Header (`internal/ui/views/analysis/analysis.go`)**:
  - Match the horizontal padding of the `Overview & Scoreboard` header row and `renderScoreboardGroup` player rows so all 13 metric headers align strictly over their numeric columns.
- **Targeted View Polish (`spectator`, `library`, `settings`)**:
  - Remove the `(unrendered)` English suffix from `Spectator`'s unrendered POV badge (`未生成独立视角视频`).
  - Make `Match Library`'s manual `.dem` import path bar toggleable via a compact `+ 手动导入 Demo` toolbar button so match cards gain top-of-fold focus.
  - Tighten `Settings & Tasks` vertical spacing slightly so `4. 后台渲染任务队列` and its empty-state message fit cleanly inside `1360x840` without bottom-edge clipping.

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `native-ui-shell-and-guard`: Tighten `Settings & Tasks` vertical spacing so the render task queue header and empty-state prompt are not clipped at `1360x840`.
- `match-library-and-analysis-ui`: Render map radar backdrop on `2D Replay` empty state, align `Overview & Scoreboard` header and row horizontal padding, and make `Match Library` manual `.dem` path input collapsible.
- `spectator-and-highlights-ui`: Remove raw `(unrendered)` English suffix from `Spectator` unrendered POV status badge.

## Impact

- **Affected Packages**: `internal/ui/components`, `internal/ui/views/analysis`, `internal/ui/views/library`, `internal/ui/views/spectator`, `internal/ui/views/settings`.
- **Non-Breaking**: Pure visual/layout polish; all SQLite, IPC, parsing, and rendering contracts remain unchanged.
