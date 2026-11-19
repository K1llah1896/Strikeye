# Tasks

## 1. Spatial Glass Design System & Vector Iconography

- [x] 1.1 Create `internal/ui/theme/icons.go` with inline SVG vector icons (`ui.SVG` helpers) for all 5 navigation views, actions, status indicators, and empty states, and verify compilation with `go test ./internal/ui/theme/...`
- [x] 1.2 Upgrade `internal/ui/theme/theme.go` with Spatial Obsidian default palette, ambient radial-glow backdrop renderer (`AmbientBackdrop`), luminous rim-light `frostedGlassShader` / `GlassCard` / `BentoCard`, `StatTile`, `SegmentedControl`, and `PillBadge` primitives, and update `internal/ui/theme/theme_test.go` to pass `go test ./internal/ui/theme/...`

## 2. Floating Glass Application Shell & Guard Dialog

- [x] 2.1 Refactor `internal/ui/app.go` to render the `AmbientBackdrop`, seamless borderless top header with dynamic status pill, floating glass navigation rail with vector icons, and glass render-now confirmation modal, and verify with `go test ./internal/ui`
- [x] 2.2 Update `internal/ui/guard/guard.go` to use Spatial Glass card styling and vector status indicators, and verify with `go test ./internal/ui/guard/...`

## 3. Match Library & Match Analysis Bento 2.0 Redesign

- [x] 3.1 Refactor `internal/ui/views/library/library.go` with a floating glass filter capsule bar (keeping collapsible `+ 手动导入 Demo`) and Bento match cards anchored on large `Rajdhani` scorelines, and verify with `go test ./internal/ui/views/library/...`
- [x] 3.2 Refactor `internal/ui/views/analysis/analysis.go` with a Match Hero summary banner, `SegmentedControl` tab bar, Bento KPI leader tiles, borderless alternating-luminance scoreboard rows, and Spatial Glass control panels for 2D Replay & KDE Heatmaps (updating `internal/ui/components/radar_2d_canvas.go` and `radar_hex_preview.go` framing as needed), and verify with `go test ./internal/ui/views/analysis/... ./internal/ui/components/...`

## 4. Spectator, Highlights & Settings Spatial Glass Redesign

- [x] 4.1 Refactor `internal/ui/views/spectator/spectator.go` (and `internal/ui/components/video_player.go`, `key_hud_widget.go` styling) to present a borderless rounded cinema viewport, pill round track, luminous event timeline, and glass CT/T/Observer POV switcher, and verify with `go test ./internal/ui/views/spectator/... ./internal/ui/components/...`
- [x] 4.2 Refactor `internal/ui/views/highlights/highlights.go` with Bento highlight cards (preserving single-line non-wrapping glass tag pills and no raw tick range) and a dual-pane Spatial Glass Montage Workbench, and verify with `go test ./internal/ui/views/highlights/...`
- [x] 4.3 Refactor `internal/ui/views/settings/settings.go` into aligned Bento glass settings groups while preserving `1360x840` unclipped `4. 后台渲染任务队列` visibility and OS-aware binary labels, and verify with `go test ./internal/ui/views/settings/...`

## 5. End-to-End Visual & Integration Verification

- [x] 5.1 Run full UI test suite and offscreen visual audit (`go test -v ./internal/ui/...`) and confirm all tests pass and rendered PNG snapshots reflect the Spatial Glass + Bento 2.0 design without clipping
