# Design: ui-visual-overhaul

## Context

See `proposal.md` for motivation and affected packages. Strikeye's desktop UI is built on `github.com/egoist/mygo/ui` (`v0.10.0`), which provides `ui.Theme` / `ui.Themed`, `ui.Modal`, `ui.ScrollHorizontal`, `Element.Font`, `Element.AlignItems`, and `ui.Render` (offscreen RGBA snapshot rendering for automated visual inspection). Currently, `internal/ui/theme/theme.go` registers `Noto Sans SC` and `Rajdhani` but never binds a dark `ui.Theme` or applies `.Font("Rajdhani")`, causing native controls (`Select`, `TextInput`, `Checkbox`, `Slider`) to render with default light-mode styling and numeric metrics to lose their esports identity.

## Goals / Non-Goals

**Goals:**
- Establish a single source of truth for dark esports styling in `internal/ui/theme` using `ui.Theme` (`EsportsTheme()`) + `ui.Themed` and typography helpers (`ScoreText`, `StatNumber`, `TabButton`, `NavItem`, `StatusDot`, `RowCenter`).
- Fix all top visual defects identified in the UI design review: layout shifts from inline modal insertion, round-button horizontal overflow in Spectator, emoji glyph dependencies, dead whitespace next to 560x560 canvases, plain-text economy rows, and unrendered state fields (`compareA`/`compareB`, `hmPlayer`).
- Enable automated offscreen visual verification via `ui.Render` (writing PNG snapshots to scratch/artifact inspection during implementation) in addition to `go test ./...`.

**Non-Goals:**
- Changing SQLite schemas, JSON-RPC IPC protocols, demo parsing, or FFmpeg/HLAE rendering logic.
- Adding decorative animations or non-functional chrome that clutters tactical data.

## Decisions

### 1. Root `ui.Themed` + `Rajdhani` / `Noto Sans SC` Font Helpers
- **Decision**: Define `theme.EsportsTheme() *ui.Theme` setting `Dark: true`, `Background: BgBase`, `Surface: BgInput`, `SurfaceHover: BgElevated`, `Border: BorderSubtle`, `Text: TextPrimary`, `TextMuted: TextMuted`, `Accent: AccentAmber`, `AccentText: BgBase`, `Font: "Noto Sans SC"`, `Scrollbar: BorderSubtle`, `Radius: 6`, and wrap root views in `ui.Themed(c, theme.EsportsTheme())` (both `App.render` and `guard.Controller.RunModal`). Provide `theme.StatText(c, s)` / `theme.ScoreText(c, s)` using `.Font("Rajdhani").FontFeatures("tnum")` for tabular numeric alignment.
- **Rationale**: Wrapping with `ui.Themed` ensures every native widget (`TextInput`, `Select`, `Checkbox`, `Slider`, `Scroll`) automatically adopts the dark palette without per-call hacks.
- **Alternative considered**: Styling every individual `TextInput` and `Select` manually—rejected because `Select` popups and `Slider` tracks only read `ui.Theme`.

### 2. Centered Modal Overlay for Render-Now Confirmation
- **Decision**: Replace the inline `if showModal { a.renderConfirmModal(c) }` inside `App.render`'s main `Column` with `ui.Modal(c, &a.showRenderModal, func() { ... })`.
- **Rationale**: `ui.Modal` renders in a centered overlay layer with backdrop dimming without displacing the header or workspace.

### 3. Visual Hierarchy & Layout Refactors per View
- **Navigation & Secondary Tabs**: Upgrade `theme.NavButton` into a structured two-line button/column item (Chinese primary title `13px Bold` + uppercase English subtitle `10px Muted`, left amber indicator bar `#F59E0B` on active, subtle `BgElevated` hover/active surface instead of `#0F141C` cutouts). Add `theme.TabButton` with bottom/border accent distinct from `ActionButton`.
- **Match Library (`library.go`)**: Reorganize each match card into a top metadata bar (map capsule, platform capsule, duration, parse/video status capsules) and a central scoreboard strip (`Team A` vs `Team B` with `Rajdhani` `22px Bold` scoreline), followed by remarks and a clear action row (`进入独立观战` as primary `AccentButton`, secondary actions grouped cleanly).
- **Spectator (`spectator.go`)**: Wrap round buttons in `ui.ScrollHorizontal(c)` (with compact `R1`..`RN` pills) so 24–30+ rounds never overflow. Group the right-side POV list into `CT 阵营`, `T 阵营`, and `导播视角 (Observer)` sections, replacing emoji dots with `theme.StatusDot` (8x8px colored rounded `ui.Box`) and showing per-player round `K/D` badges.
- **Analysis (`analysis.go`)**:
  - *Overview*: Group scoreboard rows into `CT` and `T` sections with matching column widths for headers and cells (`AlignItems(ui.Center)`).
  - *Rounds & Economy*: Add a horizontal round-outcome timeline strip (colored `R01`..`RN` winner badges) and proportional CT vs T equipment-value bars (`$4,200` vs `$4,100`) per round with buy-type tags.
  - *Radar Workbench*: Render a side-by-side two-player comparison card (`compareA` vs `compareB` using `ui.Select` and 6-metric delta rows: Rating, ADR, KAST%, KPR, Surviving, Multi-kills) above/beside the `RadarHexPreview`.
  - *2D Replay & Heatmaps*: Use a two-column split layout (`560x560` canvas on the left, `fill` tactical control/legend/player-status panel on the right), and wire the existing `v.hmPlayer` filter into `ui.Select` + `BuildHeatmapPoints` so player-filtered heatmaps actually work and are accessible.
- **Highlights & Settings (`highlights.go`, `settings.go`, `guard.go`)**: Apply `RowCenter` alignment, clean card headers, consistent input widths/labels, and `Rajdhani` numeric badges.

## Risks / Trade-offs

- **Offscreen Rendering Font Fallback in Linux Headless Sandbox**: `ui.Render` / `Tester` in Linux without a GPU or system font might behave differently from Windows DirectWrite/Skia. → **Mitigation**: `theme.RegisterFonts()` embeds `Noto Sans SC` and `Rajdhani` TTF bytes directly from `assets/fonts`, so `ui.Render` uses the exact embedded fonts consistently across platforms.
- **FilterMatches / BuildHeatmapPoints Test Compatibility**: Existing unit tests in `analysis_test.go`, `library_test.go`, `spectator_test.go`, `highlights_test.go` test view helpers and state structs. → **Mitigation**: Keep all exported function signatures (`FilterMatches`, `SortPlayers`, `BuildHeatmapPoints`, `ResolvePOVStates`, `FilterClips`, `BuildMontageSpec`, `ApplyChoicesToConfig`) backward-compatible and extend `BuildHeatmapPoints` only to honor `f.PlayerName` when non-empty and not `"全部选手"`.
