# Tasks

## 1. Theme Formatters & Borderless Tab Differentiation

- [x] 1.1 Add `ParseStatusLabel`, `BuyTypeLabel`, `WinReasonLabel`, `ClipTypeLabel`, and `ParseTags` helpers in `internal/ui/theme/format.go` with unit tests in `internal/ui/theme/theme_test.go` and verify `go test ./internal/ui/theme/...` passes.
- [x] 1.2 Update `theme.TabButton` in `internal/ui/theme/theme.go` to use a borderless bottom-indicator style distinct from `ActionButton` and verify `go test ./internal/ui/theme/...` passes.

## 2. Match Library & Settings Polish

- [x] 2.1 Streamline `internal/ui/views/library/library.go` toolbar (remove redundant inline watch-dir input), localize `ParseStatusLabel` badges, and balance the scoreline strip layout; verify `go test ./internal/ui/views/library/...` passes.
- [x] 2.2 Make `ffmpeg`/`ffprobe` labels OS-aware in `internal/ui/views/settings/settings.go` and compact the watch-directory textarea height so the render queue is reachable earlier; verify `go test ./internal/ui/views/settings/...` passes.

## 3. Match Analysis, Spectator & Highlights Polish

- [x] 3.1 Replace the 13 bordered scoreboard sort boxes in `internal/ui/views/analysis/analysis.go` with a borderless aligned header row + active sort arrow, add `TEAM A (CT)` / `TEAM B (T)` headers and localized `WinReasonLabel`/`BuyTypeLabel` in `renderEconomy`, and upgrade empty states in `renderRadarWorkbench` and `renderReplay2D` (including empty `replayIndex` fallback); verify `go test ./internal/ui/views/analysis/...` passes.
- [x] 3.2 Lighten `Spectator` round selector pills in `internal/ui/views/spectator/spectator.go` with an overflow/scroll cue when rounds exceed visible width; verify `go test ./internal/ui/views/spectator/...` passes.
- [x] 3.3 Parse `TagsJSON` into visual tag pills and localize `ClipTypeLabel` in `internal/ui/views/highlights/highlights.go`; verify `go test ./internal/ui/views/highlights/...` passes.

## 4. Offscreen Visual Verification

- [x] 4.1 Run `go test ./internal/ui/...` and `STRIKEYE_AUDIT_DIR=... go test -v -run TestVisualAuditOffscreenRender ./internal/ui`, then visually inspect all rendered PNGs at `1360x840` to confirm all 3 top design-review issues are resolved with no regressions.
