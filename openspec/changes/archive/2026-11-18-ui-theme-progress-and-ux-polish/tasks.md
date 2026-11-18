# Tasks

## 1. Theme Configuration & Dynamic Theme Engine

- [x] 1.1 Add `ThemeConfig` (`preset`, `accent_color`, `background_image`, `overlay_opacity`, `card_opacity`) to `internal/config/config.go` with defaults and `Normalize()` clamping, and verify `go test ./internal/config/...` passes.
- [x] 1.2 Refactor `internal/ui/theme/theme.go` to support built-in presets, custom accent color overrides, card opacity, and cached background image decoding/bitmap management via `theme.Apply(cfg.Theme)`, and verify `go test ./internal/ui/theme/...` passes.
- [x] 1.3 Update `internal/ui/app.go` to initialize the active theme from config on startup, render the optional cached background image + readability overlay beneath the shell, and add an "外观与主题" section to `internal/ui/views/settings/settings.go` that applies and persists `ThemeConfig` cleanly (verified via `go test ./internal/ui/...`).

## 2. Progress Bar Primitive & Waiting/Loading Flows

- [x] 2.1 Add a reusable `theme.ProgressBar(c, progress, label)` component in `internal/ui/theme/theme.go` supporting determinate (`0.0–1.0`) and indeterminate states, and verify it compiles and renders cleanly in theme tests.
- [x] 2.2 Add throttled `OnProgress(stage string, progress float64)` reporting to `parser.ParseOptions` / `ParseAndPersist` in `internal/parser/single_pass.go`, and publish `parse.progress` IPC events from `cmd/strikeye-agent/main.go` (verified via `go test ./internal/parser/...` and `go build ./cmd/strikeye-agent`).
- [x] 2.3 Subscribe to `parse.progress` in `internal/ui/app.go` and `internal/ui/views/library/library.go` to render live parse progress bars in the top status bar and on `parsing` match cards, and verify with unit tests in `internal/ui/views/library/` and `internal/ui/`.
- [x] 2.4 Convert `NavigateTo` demo loading (`analysis.SelectDemo`, `spectator.SelectDemo`, `highlights.LoadDemo`) and heavy Match Analysis operations (radar card generation, heatmap recomputation) to asynchronous execution with mutex-guarded loading progress bars so the UI thread never blocks (verified via `go test ./internal/ui/...`).

## 3. 2D Tactical Radar Playback Acceleration

- [x] 3.1 Cache the pre-scaled `image.RGBA` map backdrop in `Radar2DCanvas.ConfigureMap` (`internal/ui/components/radar_2d_canvas.go`) and copy it directly in `RenderReplayFrame` instead of re-scaling the PNG every frame; verify with `go test ./internal/ui/components/...`.
- [x] 3.2 Update `renderReplay2D` in `internal/ui/views/analysis/analysis.go` to use wall-clock delta-time frame stepping (`elapsed * 32.0 * speed`), continuous redraw scheduling while playing, and a `0.5x / 1x / 2x / 4x / 8x` speed selector; verify with `go test ./internal/ui/views/analysis/...`.

## 4. Highlight Library Card Cleanup

- [x] 4.1 Remove `Ticks: %d ~ %d` from `renderClipCard` in `internal/ui/views/highlights/highlights.go` and place tag pills inside `ui.ScrollHorizontal` with `.Shrink(0)` on each pill so tags stay single-line without wrapping; verify with `go test ./internal/ui/views/highlights/...`.

## 5. Integration & Verification

- [x] 5.1 Run `go test ./...` across the workspace and verify all packages build and pass without regressions.
