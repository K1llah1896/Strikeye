# Tasks

## 1. 2D Replay Backdrop & Scoreboard Alignment

- [x] 1.1 Update `Radar2DCanvas.ConfigureMap` in `internal/ui/components/radar_2d_canvas.go` to render the map radar backdrop (or tactical grid) immediately without deadlock, and verify `go test ./internal/ui/components/...` passes.
- [x] 1.2 Match `PaddingX(6)` on `renderScoreboardGroup` rows in `internal/ui/views/analysis/analysis.go` to pixel-align all 13 metric columns with the header row, and verify `go test ./internal/ui/views/analysis/...` passes.

## 2. Targeted View Polish (Spectator, Library, Settings)

- [x] 2.1 Remove `(unrendered)` from `renderUnrenderedCard` in `internal/ui/views/spectator/spectator.go` and verify `go test ./internal/ui/views/spectator/...` passes.
- [x] 2.2 Make manual `.dem` import collapsible behind `+ 手动导入 Demo` in `internal/ui/views/library/library.go` and verify `go test ./internal/ui/views/library/...` passes.
- [x] 2.3 Tighten `Settings & Tasks` vertical spacing in `internal/ui/views/settings/settings.go` so `4. 后台渲染任务队列` empty-state text is unclipped at `1360x840`, and verify `go test ./internal/ui/views/settings/...` passes.

## 3. Offscreen Visual Verification

- [x] 3.1 Run `go test -count=1 ./internal/ui/...` with `STRIKEYE_AUDIT_DIR` set and inspect `01_library.png`, `02_spectator.png`, `03_analysis.png`, `03d_replay2d.png`, and `05_settings.png` at `1360x840` to confirm all Round 2 issues are resolved.
