# Proposal

## Why

Strikeye's MyGo Native UI currently hardcodes a single dark esports palette, blocks the UI thread or lacks progress feedback during demo parsing and heavy analysis loading, plays 32Hz 2D tactical radar replays far slower than real time due to per-frame rendering bottlenecks and single-frame stepping, and clutters Highlight Library cards with raw tick ranges (`Ticks: xxx ~ xxx`) and wrapping tag pills. Addressing these four UX gaps together makes the desktop workbench customizable, responsive, and visually clean.

## What Changes

- **Customizable Theme System**:
  - Introduce `ThemeConfig` in `config.json` (preset themes, custom accent color, custom background image path, background overlay opacity, and card opacity) with atomic persistence and safe normalization.
  - Refactor `internal/ui/theme` from static dark-only globals to a dynamic theme state that updates palette tokens and caches scaled background bitmaps.
  - Render the optional background image and readability overlay behind the main shell in `internal/ui/app.go`, and add an Appearance & Theme section in `Settings & Tasks`.
- **Progress Bars for Waiting & Loading Scenarios**:
  - Add a reusable `theme.ProgressBar` component supporting determinate percentage (`0.0–1.0`) and indeterminate loading states.
  - Add progress reporting to `parser.ParseAndPersist` and publish `parse.progress` IPC events from `strikeye-agent` so the top status bar and Match Library cards show live demo parsing progress.
  - Convert heavy view transitions (`NavigateTo` -> `Match Analysis`, `Spectator`, `Highlights`) and long-running UI operations (radar card generation, heatmap recomputation) to asynchronous loading with visible progress bars instead of blocking the UI thread.
- **2D Tactical Radar Playback Acceleration**:
  - Cache the pre-scaled map radar backdrop (`image.RGBA`) in `Radar2DCanvas.ConfigureMap` to eliminate per-frame full-image scaling overhead.
  - Switch `renderReplay2D` in `Match Analysis` from single-frame-per-render stepping to wall-clock delta-time stepping (`elapsed * 32.0 * speed`) with continuous redraw scheduling while playing, plus a playback speed selector (`0.5x`, `1x`, `2x`, `4x`, `8x`).
- **Highlight Library Card Cleanup**:
  - Remove the `Ticks: %d ~ %d` text from `Highlight Library` clip cards.
  - Render highlight tag pills inside a single-line `ui.ScrollHorizontal` strip with `Shrink(0)` on each pill so tags never wrap across lines.

## Capabilities

### New Capabilities
- None (all changes extend existing capabilities).

### Modified Capabilities
- `config-and-paths`: Persist and normalize `ThemeConfig` (preset, custom accent color, background image path, overlay opacity, card opacity) in `config.json`.
- `native-ui-shell-and-guard`: Support dynamic theme presets, custom accent colors, background images with readability overlays, Appearance settings in `Settings & Tasks`, reusable `ProgressBar` primitives, and non-blocking async view loading indicators.
- `single-pass-parser`: Emit stage and percentage progress callbacks during demo parsing so `strikeye-agent` can broadcast `parse.progress` IPC events.
- `match-library-and-analysis-ui`: Display live demo parse progress bars on Match Library cards, async loading progress bars in Match Analysis, and time-accurate, speed-selectable (`0.5x`–`8x`) 2D tactical radar playback with cached map backdrops.
- `spectator-and-highlights-ui`: Omit `Ticks: xxx ~ xxx` from Highlight Library clip cards and render tag pills in a non-wrapping single-line horizontal scroll strip.

## Impact

- **Configuration & Agent**: `internal/config/config.go`, `internal/parser/single_pass.go`, `cmd/strikeye-agent/main.go`.
- **UI Theme & Shell**: `internal/ui/theme/theme.go`, `internal/ui/app.go`, `internal/ui/views/settings/settings.go`.
- **UI Views & Components**: `internal/ui/views/library/library.go`, `internal/ui/views/analysis/analysis.go`, `internal/ui/views/highlights/highlights.go`, `internal/ui/views/spectator/spectator.go`, `internal/ui/components/radar_2d_canvas.go`.
