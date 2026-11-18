# Proposal: ui-visual-overhaul

## Why

A design review of Strikeye's Phase 5 MyGo Native UI (`internal/ui/`) identified critical visual and UX gaps: the registered `Rajdhani` and `Noto Sans SC` fonts and dark esports theme (`ui.Themed`) are not applied to native controls, match cards and economy/analysis panels bury key competitive data in unaligned plain-text rows, and several layouts suffer from overflow, dead whitespace, or missing UI bindings for already-defined state (such as two-player comparison and heatmap player filtering). An overhaul is needed now so the desktop client feels like a cohesive, high-contrast CS2 tactical analysis workbench rather than an unstyled developer prototype.

## What Changes

- **Global Theme, Typography & Shell (`internal/ui/theme`, `internal/ui/app.go`, `internal/ui/guard`)**:
  - Bind a unified dark esports `ui.Theme` via `ui.Themed` at the root view so native inputs, selects, checkboxes, sliders, and scrollbars match `#0F141C` / `#171E29`.
  - Apply `Rajdhani` to all numeric metrics (scores, Rating, ADR, K/D, ticks, timestamps) and `Noto Sans SC` to Chinese labels, with consistent vertical center alignment across all rows.
  - Replace the inline flow-inserted render confirmation card with a true centered modal overlay (`ui.Modal` / `ui.Overlay`), and upgrade the left navigation rail and secondary tab bars into distinct visual tiers.
- **Match Library & Match Analysis (`internal/ui/views/library`, `internal/ui/views/analysis`)**:
  - Redesign Match Library cards around a clear `Team A  Score : Score  Team B` visual anchor, map/platform badges, and primary vs. secondary action hierarchy.
  - Group the Analysis scoreboard by CT / T factions with aligned columns, add round-by-round win-sequence strips and proportional equipment-value bars in Rounds & Economy, render the side-by-side two-player comparison panel (`compareA` vs `compareB`) in the Radar workbench, and convert 2D Replay & Heatmaps into balanced split-pane layouts with the missing `hmPlayer` filter and density legend.
- **Spectator, Highlights & Settings (`internal/ui/views/spectator`, `internal/ui/views/highlights`, `internal/ui/views/settings`, `internal/ui/components`)**:
  - Make the Spectator round selector horizontally scrollable / half-grouped to prevent overflow on 24–30+ round matches, group the POV switcher by CT / T / Observer, and replace emoji dots (`🟢`/`🟡`/`⚪`) with crisp native vector status indicators.
  - Refine Highlight Library, Montage Workbench, Key HUD, and Settings & Tasks spacing, label clarity, and visual hierarchy without altering any underlying IPC, SQLite, or rendering logic.

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `native-ui-shell-and-guard`: Bind dark esports `ui.Theme` and embedded `Rajdhani` / `Noto Sans SC` typography across shell and guard views, use a centered modal overlay for render-now confirmation, and distinguish primary navigation from secondary tabs.
- `match-library-and-analysis-ui`: Anchor match cards on team scores, group scoreboard by CT/T, visualize round outcomes and economy bars, expose side-by-side two-player comparison in the radar workbench, and adopt a split-pane layout with player filtering for 2D Replay and Heatmaps.
- `spectator-and-highlights-ui`: Support overflow-safe round selection, CT/T/Observer POV grouping with native vector status indicators (no emoji dependency), and refined spacing/alignment across Highlights and Montage Workbench.

## Impact

- **Affected Packages**: `internal/ui/theme`, `internal/ui/app.go`, `internal/ui/guard`, `internal/ui/components`, `internal/ui/views/library`, `internal/ui/views/spectator`, `internal/ui/views/analysis`, `internal/ui/views/highlights`, `internal/ui/views/settings`.
- **Non-Breaking**: All existing SQLite repositories, JSON-RPC IPC contracts, FFmpeg/HLAE pipelines, and unit test behaviors remain unchanged.
