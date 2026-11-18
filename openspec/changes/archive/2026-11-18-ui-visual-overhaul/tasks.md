# Tasks: ui-visual-overhaul

## 1. Design System, Theme & Shell Overhaul

- [x] 1.1 Implement `theme.EsportsTheme()` (`ui.Theme`), `theme.StatText` / `theme.ScoreText` (`Rajdhani` + `tnum`), `theme.RowCenter`, `theme.StatusDot`, `theme.TabButton`, and two-tier `theme.NavButton` in `internal/ui/theme/theme.go`; verify with `go test ./internal/ui/theme/...`.
- [x] 1.2 Wrap `App.render` (`internal/ui/app.go`) and `guard.Controller.RunModal` (`internal/ui/guard/guard.go`) in `ui.Themed(c, theme.EsportsTheme())`, convert the render-now confirmation to `ui.Modal`, and align top header bar items vertically; verify with `go test ./internal/ui ./internal/ui/guard/...`.

## 2. Match Library & Match Analysis Views

- [x] 2.1 Redesign Match Library cards (`internal/ui/views/library/library.go`) with scoreline visual anchors (`Team A  Score : Score  Team B` in `Rajdhani`), metadata capsules, aligned toolbar rows, and primary/secondary action hierarchy; verify with `go test ./internal/ui/views/library/...`.
- [x] 2.2 Upgrade Analysis `Overview & Scoreboard` (`internal/ui/views/analysis/analysis.go`) to CT/T faction-grouped tables with aligned headers/cells and `Rajdhani` numeric columns, and upgrade `Rounds & Economy` with a round-outcome sequence strip and proportional team equipment-value bars; verify with `go test ./internal/ui/views/analysis/...`.
- [x] 2.3 Add the side-by-side two-player comparison panel (`compareA` vs `compareB`) in Analysis `Players & CS Data Radar`, and convert `2D Replay` and `Heatmaps` into balanced split-pane layouts with the wired `hmPlayer` filter and density legend; verify with `go test ./internal/ui/views/analysis/...`.

## 3. Spectator, Highlights, Settings & Components

- [x] 3.1 Update Spectator (`internal/ui/views/spectator/spectator.go`) with `ui.ScrollHorizontal` round pills, CT/T/Observer grouped POV switcher using `theme.StatusDot` (no emoji glyphs), and polished unrendered summary card; verify with `go test ./internal/ui/views/spectator/...`.
- [x] 3.2 Refine Highlight Library, Montage Workbench (`internal/ui/views/highlights/highlights.go`), Settings & Tasks (`internal/ui/views/settings/settings.go`), and player/HUD components (`internal/ui/components/`) for consistent spacing, vertical alignment, and `Rajdhani` numeric badges; verify with `go test ./internal/ui/views/highlights/... ./internal/ui/views/settings/... ./internal/ui/components/...`.

## 4. Offscreen Visual Verification & Iterative Polish

- [x] 4.1 Render offscreen snapshots of all 5 primary views + Guard modal using `ui.Render` with seeded fixture data, inspect the rendered PNGs for hierarchy, alignment, overflow, and contrast, and iterate on any remaining visual defects; verify all `go test ./...` and `go build ./cmd/strikeye` pass cleanly.
