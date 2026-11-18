# Spec Delta: match-library-and-analysis-ui

## Purpose

Defines the Match Library browser, bundled map radar assets, and the five-tab Match Analysis workspace including scoreboard, economy curves, CS Data Radar workbench, 32Hz 2D Replay, and KDE Heatmaps.

## ADDED Requirements

### Requirement: Match Library search, filtering, and quick actions
The `Match Library` view SHALL query SQLite WAL for ingested demos and support filtering by keyword (map, player name, note, filename), map, platform, parse/render status, and favorites, displaying parse and video status capsules alongside quick-navigation actions.

#### Scenario: Filter and open match analysis
- **WHEN** the user enters a map or player keyword and clicks "Match Analysis" on a parsed match card
- **THEN** the list filters to matching demos and navigates directly to `Match Analysis` for the selected demo

### Requirement: Manual demo import and watch-directory configuration
The `Match Library` toolbar SHALL allow users to manually add a local `.dem` file (triggering `demo.parse` over IPC) and configure watched demo directories.

#### Scenario: Manual demo addition
- **WHEN** the user selects a valid `.dem` file via "Add Demo"
- **THEN** the UI enqueues the file for Single-Pass parsing via IPC and displays its `parsing` status capsule until ready

### Requirement: Overview scoreboard and rounds/economy analysis tabs
The `Match Analysis` view SHALL provide an `Overview & Scoreboard` tab with MVP/highlight leader cards and a 10-player table sortable by K, D, A, K/D, ADR, KAST%, HS%, FK, FD, TK, Multi-Kills, Clutches, and `Strikeye Rating`, plus a `Rounds & Economy` tab plotting round outcomes and team equipment value with buy-type annotations.

#### Scenario: Sort scoreboard by Strikeye Rating
- **WHEN** the user clicks the `Strikeye Rating` column header
- **THEN** all 10 player rows re-order by Strikeye Rating descending or ascending

### Requirement: CS Data Radar workbench with avatar/logo re-rendering
The `Players & CS Data Radar` tab SHALL support side-by-side comparison of any two players, batch generation of `2560x1440` radar cards and `4.0s` intro MP4s via `internal/radar`, custom player avatar and team logo binding with automatic re-render, and in-app preview.

#### Scenario: Re-render radar card on custom avatar update
- **WHEN** the user binds a custom avatar image to a player in the radar workbench
- **THEN** the corresponding `2560x1440` radar card regenerates and updates in the preview component

### Requirement: 32Hz 2D tactical replay and KDE heatmaps
The `2D Replay` and `Heatmaps` tabs SHALL read `cache/replay_32hz/<sha256>.rpl` (`CS2RPL01`) and bundled `assets/maps/` radar images + transform metadata (falling back to a coordinate grid if a map image is missing) to render 32Hz player poses, view cones, flash rings, C4 carrier markers, firing rays, grenade arcs, smoke circles, molotov polygons, and KDE position/firefight/kill/death heatmaps.

#### Scenario: Play 2D replay without CS2 or video
- **WHEN** the user opens `2D Replay` for any parsed demo and selects a round
- **THEN** the native canvas animates all 10 players and utility effects at 32Hz without requiring any rendered MP4 or launching CS2
