# match-library-and-analysis-ui Specification

## Purpose

Defines the Match Library browser, bundled map radar assets, and the five-tab Match Analysis workspace including scoreboard, economy curves, CS Data Radar workbench, 32Hz 2D Replay, and KDE Heatmaps.

## Requirements

### Requirement: Match Library search, filtering, and quick actions
The `Match Library` view SHALL query SQLite WAL for ingested demos and support filtering by keyword (map, player name, note, filename), map, platform, parse/render status, and favorites, displaying each match card anchored on prominent team names and scoreline, map/platform/status capsules, and clearly prioritized quick-navigation actions.

#### Scenario: Filter and open match analysis
- **WHEN** the user enters a map or player keyword and clicks "Match Analysis" on a parsed match card
- **THEN** the list filters to matching demos and navigates directly to `Match Analysis` for the selected demo

### Requirement: Manual demo import and watch-directory configuration
The `Match Library` toolbar SHALL allow users to manually add a local `.dem` file (triggering `demo.parse` over IPC) and configure watched demo directories.

#### Scenario: Manual demo addition
- **WHEN** the user selects a valid `.dem` file via "Add Demo"
- **THEN** the UI enqueues the file for Single-Pass parsing via IPC and displays its `parsing` status capsule until ready

### Requirement: Overview scoreboard and rounds/economy analysis tabs
The `Match Analysis` view SHALL provide an `Overview & Scoreboard` tab with MVP/highlight leader cards and a CT/T faction-grouped 10-player table with aligned columns sortable by K, D, A, K/D, ADR, KAST%, HS%, FK, FD, TK, Multi-Kills, Clutches, and `Strikeye Rating`, plus a `Rounds & Economy` tab plotting round-by-round win sequence badges and proportional team equipment-value bars with buy-type annotations.

#### Scenario: Sort scoreboard by Strikeye Rating
- **WHEN** the user clicks the `Strikeye Rating` column header
- **THEN** player rows re-order by Strikeye Rating descending or ascending with aligned numeric columns

### Requirement: CS Data Radar workbench with avatar/logo re-rendering
The `Players & CS Data Radar` tab SHALL support side-by-side comparison of any two players with six-axis and detailed stat comparison, batch generation of `2560x1440` radar cards and `4.0s` intro MP4s via `internal/radar`, custom player avatar and team logo binding with automatic re-render, and in-app preview.

#### Scenario: Re-render radar card on custom avatar update
- **WHEN** the user binds a custom avatar image to a player in the radar workbench
- **THEN** the corresponding `2560x1440` radar card regenerates and updates in the preview component

### Requirement: 32Hz 2D tactical replay and KDE heatmaps
The `2D Replay` and `Heatmaps` tabs SHALL read `cache/replay_32hz/<sha256>.rpl` (`CS2RPL01`) and bundled `assets/maps/` radar images + transform metadata (falling back to a coordinate grid if a map image is missing) inside a balanced split-pane layout (canvas alongside tactical controls, player/team/round-type filters, and legend) to render 32Hz player poses, view cones, flash rings, C4 carrier markers, firing rays, grenade arcs, smoke circles, molotov polygons, and KDE position/firefight/kill/death heatmaps.

#### Scenario: Play 2D replay without CS2 or video
- **WHEN** the user opens `2D Replay` for any parsed demo and selects a round
- **THEN** the native canvas animates all 10 players and utility effects at 32Hz alongside round/playback controls without requiring any rendered MP4 or launching CS2

### Requirement: Localized tactical labels instead of raw database enums
`Match Library` and `Match Analysis` SHALL render human-readable localized labels for parse status (`ready`, `parsing`, `pending`, `failed`), economy buy types (`full_buy`, `half_buy`, `eco`, `force_buy`, `pistol`), and round win reasons (`elimination`, `bomb_exploded`, `bomb_defused`, `time_ran_out`) rather than exposing raw snake_case identifiers.

#### Scenario: Viewing economy and match status badges
- **WHEN** a match with `ready` parse status and `full_buy` / `elimination` rounds is displayed in `Match Library` or `Rounds & Economy`
- **THEN** the UI renders localized tactical labels (e.g., `已解析`, `长枪局`, `全歼`) instead of raw database enum strings

### Requirement: Streamlined Match Library toolbar and centered scoreline anchor
The `Match Library` top toolbar SHALL avoid duplicating watch-directory management inline, and each match card SHALL anchor `Team A  Score : Score  Team B` cleanly with balanced spacing between the scoreline and match remarks.

#### Scenario: Browsing Match Library cards
- **WHEN** the user opens `Match Library`
- **THEN** the toolbar presents a compact filter/import layout and each match card displays a balanced team scoreline without a wide empty gap before the remark

### Requirement: Borderless scoreboard headers, economy column headers, and structured empty states
`Overview & Scoreboard` SHALL use a borderless, aligned table header row with active sort indicators instead of 13 bordered button boxes; `Rounds & Economy` SHALL display `TEAM A (CT)` and `TEAM B (T)` column headers; and `Radar` / `2D Replay` SHALL render structured empty-state guides (including when `replayIndex` is empty) instead of blank dark rectangles.

#### Scenario: Opening 2D Replay without cached replay frames
- **WHEN** the user opens `2D Replay` for a demo that has no `CS2RPL01` replay index
- **THEN** the round selector area displays an explicit empty-state message and the canvas shows a structured tactical placeholder guide

### Requirement: 2D Replay canvas displays map radar backdrop when frames are absent
When `2D Replay` is configured for a demo without `CS2RPL01` replay frames, the `560x560` radar canvas SHALL render the bundled map radar image (or procedural tactical grid fallback) instead of a solid black rectangle.

#### Scenario: Opening 2D Replay without cached replay frames
- **WHEN** the user opens `2D Replay` for a parsed `de_mirage` demo with no replay frames
- **THEN** the `560x560` canvas displays the `de_mirage` tactical radar backdrop

### Requirement: Pixel-aligned scoreboard column headers and rows
The `Overview & Scoreboard` header row and CT/T player data rows SHALL share identical horizontal padding and column widths so every metric header aligns directly above its values.

#### Scenario: Viewing Overview & Scoreboard columns
- **WHEN** the user views `Overview & Scoreboard`
- **THEN** all 13 sortable column headers (`K` through `Rating`) align vertically with the numeric cells below them

### Requirement: Collapsible manual demo import in Match Library
The `Match Library` toolbar SHALL keep manual `.dem` path entry collapsed behind a `+ 手动导入 Demo` toggle button by default, expanding the path input row on click.

#### Scenario: Toggling manual demo import
- **WHEN** the user clicks `+ 手动导入 Demo` in the `Match Library` toolbar
- **THEN** the local `.dem` path input and submit button appear below the filter bar

### Requirement: Demo parse progress bar in Match Library
The `Match Library` view SHALL display a live progress bar and stage label on any match card whose demo is currently parsing (`parse_status == "parsing"`), updated in real time from `parse.progress` IPC events.

#### Scenario: Viewing a demo currently being parsed
- **WHEN** a demo enters `parsing` state and `parse.progress` events arrive for its `demo_id`
- **THEN** the corresponding Match Library card renders a progress bar reflecting the reported percentage and current parse stage

### Requirement: Async loading progress in Match Analysis
The `Match Analysis` view SHALL display a progress bar overlay or loading card while loading match data, computing analytics, generating 10-player radar cards, or recomputing KDE heatmaps so the user has clear visual feedback during waiting periods.

#### Scenario: Opening Match Analysis for a demo
- **WHEN** `Match Analysis` begins loading a demo asynchronously
- **THEN** the view renders a progress bar with stage status (e.g., loading match stats, loading 2D replay index) until all data is ready

### Requirement: Time-accurate speed-selectable 2D tactical radar playback
The `2D Replay` tab in `Match Analysis` SHALL cache the pre-scaled map backdrop per map/layer configuration, advance frames using wall-clock elapsed time scaled by a user-selectable playback speed (`0.5x`, `1x`, `2x`, `4x`, `8x`) at the 32Hz base rate, and schedule continuous UI redraws while playing.

#### Scenario: Playing 2D tactical radar at 4x speed
- **WHEN** the user selects `4x` speed and clicks Play on `2D Replay`
- **THEN** the radar playback advances at 128 frames per second of wall-clock time (4x real-time 32Hz) smoothly without stalling behind single-frame render intervals
