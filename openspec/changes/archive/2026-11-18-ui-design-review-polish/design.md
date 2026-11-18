# Design: ui-design-review-polish

## Context

See `proposal.md` for motivation. Visual inspection of the `1360x840` offscreen-rendered PNGs (`TestVisualAuditOffscreenRender`) shows that while the dark palette (`#0F141C` / `#171E29`), `Rajdhani` tabular numbers, and `Noto Sans SC` typography are working, three systemic visual issues remain: raw backend strings leaking into UI, "bordered-box fatigue" across tabs/headers/pills, and heavy blank-rectangle empty states.

## Goals / Non-Goals

**Goals:**
- Centralize display formatting (`ParseStatusLabel`, `BuyTypeLabel`, `WinReasonLabel`, `ClipTypeLabel`, `ParseTagsJSON`) in `internal/ui/theme` so no raw JSON or snake_case enum leaks to the UI.
- Break visual homogenization by reserving `1px BorderSubtle` boxes for primary cards/buttons while converting `TabButton`, scoreboard headers, and round pills to lighter underline/subtle-surface treatments.
- Improve spatial rhythm in `Match Library`, `Rounds & Economy`, `Settings & Tasks`, and empty states in `Radar` / `2D Replay`, verified via `TestVisualAuditOffscreenRender` PNG inspection.

**Non-Goals:**
- Modifying SQLite schemas, JSON-RPC IPC contracts, exported filter/sort function signatures, or underlying FFmpeg/HLAE/radar generation logic.
- Adding decorative animations or extra card chrome.

## Decisions

### 1. Centralized Tactical Localization Helpers in `internal/ui/theme`
- **Decision**: Add pure formatting functions in `internal/ui/theme/format.go` (or `theme.go`):
  - `ParseStatusLabel(status string) string` (`ready` -> `已解析`, `parsing` -> `解析中`, `pending` -> `待解析`, `failed` -> `解析失败`)
  - `BuyTypeLabel(buy string) string` (`full_buy` -> `长枪局`, `half_buy` -> `半起局`, `force_buy` -> `强起局`, `eco` -> `Eco`, `pistol` -> `手枪局`)
  - `WinReasonLabel(reason string) string` (`elimination` -> `全歼`, `bomb_exploded` -> `雷包引爆`, `bomb_defused` -> `拆除雷包`, `time_ran_out` -> `时间耗尽`)
  - `ClipTypeLabel(t string) string` (`multi_kill` -> `多杀`, `clutch` -> `残局`, `ace` -> `五杀`, `awp` -> `狙击`)
  - `ParseTags(tagsJSON string) []string` (unmarshals JSON array, trims whitespace, falls back to comma-split if non-JSON)
- **Rationale**: Keeps all views consistent and unit-testable without touching storage/parser enums.
- **Alternative considered**: Translating inside each view—rejected because `library`, `analysis`, `highlights`, and `spectator` share several of these states.

### 2. Visual Hierarchy: Underline Tabs & Borderless Table Headers
- **Decision**:
  - Update `theme.TabButton(c, label, active)` to render a flat button with a 2px bottom accent bar (`AccentAmber` when active, `ui.Transparent` when inactive) and no outer `1px BorderSubtle` box.
  - In `analysis.go` (`renderOverview`), replace the 13 bordered `ui.ButtonBase` sort boxes with a flat `BgInput` header bar where each column is a borderless clickable label with `▼`/`▲` indicator on the active `sortKey`.
  - In `analysis.go` (`renderEconomy`), add a compact header row (`回合 | 胜方与方式 | TEAM A (CT) 装备 | TEAM B (T) 装备`) and tighten row vertical padding (`PaddingY(4)`).
  - In `spectator.go`, use lighter `BgInput` round pills without heavy outer borders for inactive rounds and append a compact `滑动查看更多 →` / round-range hint when `len(rounds) > 16`.
- **Rationale**: Eliminates "bordered-box fatigue" so primary action buttons and match cards immediately stand out.

### 3. Layout Rhythm, OS-Aware Settings, & Structured Empty States
- **Decision**:
  - **Match Library (`library.go`)**: Remove the redundant `watchDirInput` + `添加目录` controls from the second toolbar row (users already have `配置监听目录` -> Settings, and can toggle/expand manual `.dem` import cleanly); center/balance the `Team A  Score : Score  Team B` strip with a subtle `VS` / divider and right-aligned remark badge.
  - **Settings (`settings.go`)**: Use `runtime.GOOS == "windows"` to decide `.exe` suffix on `ffmpeg`/`ffprobe` (and mark `cs2.exe`/`HLAE.exe` clearly as Windows recording dependencies), and reduce the watch-directory `TextArea` height so `4. 后台渲染任务队列` is visible earlier.
  - **Empty States (`analysis.go`)**: In `renderRadarWorkbench` and `renderReplay2D`, replace blank dark boxes with structured empty-state panels (title + 1-line actionable explanation + subtle `BgElevated` inner frame), and render `"暂无 32Hz 回放索引（需先完成 Demo 解析）"` when `len(v.replayIndex) == 0`.

## Risks / Trade-offs

- **Existing Unit Tests Checking UI Strings**: Some tests in `internal/ui/views/*_test.go` might assert on helper outputs or view state. → **Mitigation**: Keep exported data-model helpers (`FilterMatches`, `SortPlayers`, `BuildHeatmapPoints`, `ResolvePOVStates`, `FilterClips`, `BuildMontageSpec`, `ApplyChoicesToConfig`) untouched and run `go test ./internal/ui/...` after every edit.
- **Visual Regression Check**: Offscreen `ui.Render` must be re-run and inspected after changes to confirm no text clipping or overflow at `1360x840`. → **Mitigation**: Include a dedicated `TestVisualAuditOffscreenRender` screenshot verification task in `tasks.md`.
