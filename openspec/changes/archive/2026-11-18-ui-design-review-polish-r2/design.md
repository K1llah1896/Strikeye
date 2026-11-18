# Design: ui-design-review-polish-r2

## Context

See `proposal.md` for the 3 visual polish findings from Round 2 of the `1360x840` offscreen review.

## Goals / Non-Goals

**Goals:**
- Ensure `Radar2DCanvas.ConfigureMap` renders the map radar backdrop (or tactical grid) into `currentImg` immediately so `2D Replay` never shows a dead black rectangle when no `CS2RPL01` frames exist.
- Align `Overview & Scoreboard` header and data rows to the exact pixel by matching `PaddingX(6)` on both.
- Refine `Spectator` unrendered badge text, make `Match Library` manual `.dem` import collapsible, and tighten `Settings & Tasks` vertical rhythm so the queue empty-state line is not clipped at `840px` height.

**Non-Goals:**
- Changing any SQLite, IPC, parser, or video/radar rendering backend logic.

## Decisions

### 1. Render Backdrop Immediately in `Radar2DCanvas.ConfigureMap`
- **Decision**: At the end of `Radar2DCanvas.ConfigureMap` (after releasing/using `r.bgImage` and `r.meta`), render an empty-frame background image (`r.RenderReplayFrame(parser.ReplayRoundData{}, -1)` or internal `drawBackground` helper) so `r.currentImg` is immediately populated with the map radar PNG or procedural grid.
- **Rationale**: `RenderUI` draws `r.currentImg` when non-nil and falls back to `"等待加载 2D 雷达数据..."` only when `r.currentImg == nil`. Populating `r.currentImg` on `ConfigureMap` gives instant visual context for the selected map even before replay frames are loaded.

### 2. Scoreboard Padding Parity
- **Decision**: Add `PaddingX(6)` to `renderScoreboardGroup` rows in `internal/ui/views/analysis/analysis.go` (matching the header row's `PaddingX(6)`), and keep `Width(150)` + `Gap(4)` + `Width(56)` identical across header and data rows.
- **Rationale**: Eliminates the 6px horizontal shift between `K..Rating` headers and their numeric columns.

### 3. Collapsible Manual Import & Compact Settings Rhythm
- **Decision**:
  - In `internal/ui/views/library/library.go`, add `showManualImport bool` toggled by a `+ 手动导入 Demo` `ActionButton` in the main toolbar row, rendering the `.dem` path input row only when expanded.
  - In `internal/ui/views/spectator/spectator.go`, change `theme.Capsule(c, "未生成独立视角视频 (unrendered)", ...)` to `theme.Capsule(c, "未生成独立视角视频", ...)`.
  - In `internal/ui/views/settings/settings.go`, tighten outer `Padding(14).Gap(8)` and `watchDirsTx` height (`Height(40)`) so `4. 后台渲染任务队列` and its empty-state line fit cleanly above `y = 840`.

## Risks / Trade-offs

- **Canvas `ConfigureMap` Lock Ordering**: `ConfigureMap` acquires `r.mu.Lock()` and `RenderReplayFrame` also acquires `r.mu.Lock()`. → **Mitigation**: Release `r.mu.Unlock()` before calling `r.RenderReplayFrame(parser.ReplayRoundData{}, -1)` (or extract a lock-free `renderFrameLocked` helper) to avoid deadlock.
