# Proposal

## Why

Strikeye's current MyGo Native UI relies heavily on flat dark surfaces, uniform 1px solid borders (`#222228`), text-only navigation items, and dense boxy containers, which makes the desktop workbench feel dated and rigid rather than modern and high-tech. By adopting a **Spatial Glass + Bento 2.0** design language inspired by contemporary dark-mode mobile and dashboard UIs, Strikeye can deliver visual depth, atmospheric clarity, and strong typographic hierarchy while preserving its native Skia/shader performance and existing functional workflows.

## What Changes

- **Spatial Glass Design System & Theme Primitives**:
  - Replace the flat pure-black default appearance with a **Spatial Obsidian** glass preset featuring an ambient radial-glow backdrop (`RadialGradient` + `Blur`), upgraded translucent frosted glass shader (`frostedGlassShader` with luminous rim highlights), soft spatial shadows, and larger corner radii (`18px–22px` cards, full-pill badges).
  - Introduce inline SVG vector iconography (`ui.Icon` primitives) across navigation, status pills, actions, and empty states.
  - Add modern UI primitives (`GlassCard`, `BentoCard`, `SegmentedControl`, `StatTile`, `PillBadge`) and expand typographic contrast (`28px–36px` `Rajdhani` Hero KPIs paired with `10px–11px` uppercase tracked labels and generous breathing room).
- **Floating Glass Shell & Seamless Header**:
  - Redesign the main window shell into a floating glass navigation rail with vector icons and active glow pills, paired with a borderless seamless top header bar and dynamic status pill.
- **Global Bento 2.0 Redesign Across All 5 Views**:
  - **Match Library**: Floating glass filter capsule bar and Bento match cards anchored on high-contrast `Rajdhani` scorelines, map/platform pills, and streamlined quick actions.
  - **Spectator**: Cinema-grade floating video viewport, low-chrome pill round track, luminous event timeline, and glass CT/T/Observer POV switcher.
  - **Match Analysis**: Match Hero summary banner, segmented pill tab switcher, Bento KPI leader tiles, borderless alternating-luminance scoreboard rows, and balanced glass tactical sidebars for 2D Replay & KDE Heatmaps.
  - **Highlights & Montage Workbench**: Bento clip cards with single-line glass tag pills, integrated trim controls, and a dual-pane glass Montage Workbench.
  - **Settings & Tasks**: Aligned Bento glass settings groups preserving the `1360x840` unclipped render-queue visibility constraint.

## Capabilities

### New Capabilities
- None (all UI surfaces and theme capabilities are already governed by the three existing UI specs below).

### Modified Capabilities
- `native-ui-shell-and-guard`: Upgrade the theme system, default preset, shell navigation rail, top status bar, and `Settings & Tasks` layout to the Spatial Glass + Bento 2.0 design language with inline SVG iconography and modern glass primitives.
- `match-library-and-analysis-ui`: Redesign `Match Library` and `Match Analysis` views with Bento 2.0 card hierarchy, Hero KPI tiles, segmented pill tabs, and Spatial Glass tactical control panels while preserving all existing data, filtering, sorting, 2D replay, and heatmap behaviors.
- `spectator-and-highlights-ui`: Redesign `Spectator`, `Highlight Library`, and `Montage Workbench` views with a floating cinema viewport, pill timeline/round tracks, glass POV cards, and Bento clip/workbench panels while preserving strict MP4-vs-unrendered separation and export flows.

## Impact

- **Theme & Primitives**: `internal/ui/theme/theme.go`, new `internal/ui/theme/icons.go` (inline SVG icons), `internal/ui/theme/theme_test.go`
- **App Shell & Guard**: `internal/ui/app.go`, `internal/ui/guard/guard.go`, `internal/ui/app_test.go`, `internal/ui/visual_audit_test.go`
- **Views & Components**:
  - `internal/ui/views/library/library.go`
  - `internal/ui/views/spectator/spectator.go`
  - `internal/ui/views/analysis/analysis.go`
  - `internal/ui/views/highlights/highlights.go`
  - `internal/ui/views/settings/settings.go`
  - `internal/ui/components/video_player.go`, `internal/ui/components/key_hud_widget.go`, `internal/ui/components/radar_2d_canvas.go`, `internal/ui/components/radar_hex_preview.go`
- **Config Defaults**: `internal/config/config.go` (if default theme preset identifier or opacity defaults are updated)
- **No Breaking Backend Changes**: SQLite schema, `strikeye-agent` JSON-RPC protocol, parser, recorder, and FFmpeg composer remain unchanged.
