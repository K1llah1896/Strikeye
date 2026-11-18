# Proposal: ui-design-review-polish

## Why

A visual design review of the `1360x840` offscreen-rendered UI snapshots (`TestVisualAuditOffscreenRender`) revealed three high-impact polish gaps remaining after the initial dark-theme overhaul: raw schema strings/JSON (`TagsJSON`, `full_buy`, `ready`, `CT (elimination)`, `.exe` labels on Linux) leak directly into user-facing views, "bordered-box fatigue" blurs the hierarchy between tabs, table headers, round pills, and action buttons, and empty-state canvases (`560x560` 2D replay, radar preview, and single-card match library) feel visually heavy or unbalanced.

## What Changes

- **Humanized Display Formatters (`internal/ui/theme` & views)**:
  - Add shared display-formatting helpers for parse status (`ready` -> `已解析`), buy types (`full_buy` -> `长枪局`, `half_buy` -> `半起局`, etc.), win reasons (`elimination` -> `全歼`, `bomb_exploded` -> `雷包引爆`, etc.), clip types (`multi_kill` -> `多杀`), and `TagsJSON` parsing into clean visual tag pills instead of raw JSON strings.
  - Use OS-aware tool labels in `Settings & Tasks` so Linux/macOS environments do not display `.exe` suffixes for `ffmpeg`/`ffprobe`, and tighten the watch-directory textarea height so `4. 后台渲染任务队列` remains reachable without excessive scrolling.
- **Borderless Hierarchy & Component Differentiation (`internal/ui/theme`, `analysis`, `spectator`, `library`)**:
  - Differentiate `TabButton` from `ActionButton` using a clean bottom-indicator underline style (no heavy outer border box).
  - Replace the 13 bordered sort-button boxes in `Overview & Scoreboard` with a borderless, aligned table header row featuring active sort indicator arrows/accents.
  - Add explicit `TEAM A (CT)` vs `TEAM B (T)` column headers above `Rounds & Economy`, tighten row padding, and replace heavy round pills in `Spectator` with lighter pill treatments plus an overflow/scroll hint.
- **Balanced Layout & Structured Empty States (`library`, `analysis`)**:
  - Streamline the `Match Library` top toolbar by removing the redundant inline watch-directory input (keeping `配置监听目录` navigation and collapsible/compact manual `.dem` import) and centering/balancing the match card scoreline strip.
  - Upgrade empty states in `Players & CS Data Radar`, `2D Replay` (including an explicit empty fallback when `replayIndex` is empty), and `Heatmaps` with subtle tactical grid/guide styling instead of blank dark rectangles.

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `native-ui-shell-and-guard`: Differentiate secondary `TabButton` styling from bordered action buttons and make `Settings & Tasks` tool labels OS-aware with tighter vertical rhythm.
- `match-library-and-analysis-ui`: Streamline the `Match Library` toolbar and scoreline balance, replace raw enum/status strings with localized tactical labels, use borderless sortable table headers and economy column headers, and provide structured empty states for Radar and 2D Replay.
- `spectator-and-highlights-ui`: Parse `TagsJSON` into clean tag pills, localize `ClipType` labels in `Highlight Library`, and refine `Spectator` round-strip visual weight and scroll-overflow cues.

## Impact

- **Affected Packages**: `internal/ui/theme`, `internal/ui/views/library`, `internal/ui/views/spectator`, `internal/ui/views/analysis`, `internal/ui/views/highlights`, `internal/ui/views/settings`.
- **Non-Breaking**: Pure UI presentation and layout refinement; all SQLite schemas, JSON-RPC IPC contracts, exported filter/sort helper signatures, and rendering pipelines remain unchanged.
