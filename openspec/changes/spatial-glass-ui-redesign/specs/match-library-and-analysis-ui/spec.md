# Spec Delta

## ADDED Requirements

### Requirement: Bento 2.0 Match Library cards and floating glass filter bar
The `Match Library` view SHALL present its search, map, platform, status, favorite toggles, and collapsible `+ 手动导入 Demo` action inside a floating translucent glass filter bar, and SHALL render each match as a Bento glass card featuring a high-contrast `Rajdhani` scoreline anchor, pill metadata badges, and streamlined quick-navigation actions.

#### Scenario: Viewing Match Library in Spatial Glass layout
- **WHEN** the user opens `Match Library` with ingested demos
- **THEN** the filter controls appear in a single floating glass capsule bar and each match card displays a prominent Bento scoreline with pill tags and quick actions

### Requirement: Match Analysis Hero summary banner, Bento KPI tiles, and borderless scoreboard
The `Match Analysis` view SHALL display a top Match Hero glass banner above a segmented pill tab switcher (`Overview`, `Economy`, `Radar`, `2D Replay`, `Heatmaps`), render `Overview & Scoreboard` leader highlights as Bento KPI tiles with large `Rajdhani` metrics, and use borderless alternating-luminance rows for the 10-player scoreboard.

#### Scenario: Switching Match Analysis tabs via segmented pill control
- **WHEN** the user clicks a tab in the `Match Analysis` segmented pill switcher
- **THEN** the active pill highlights smoothly with the active accent tint and switches the workspace below the Hero summary banner

### Requirement: Unified Spatial Glass control panels for 2D Replay and Heatmaps
The `2D Replay` and `Heatmaps` tabs SHALL frame the tactical radar canvas inside a rounded Spatial Glass container alongside a translucent tactical control panel using pill selectors, clean sliders, and vector legend indicators.

#### Scenario: Viewing 2D Replay and Heatmaps side-by-side panels
- **WHEN** the user opens `2D Replay` or `Heatmaps`
- **THEN** the radar canvas and its tactical filter/playback controls render inside matched Spatial Glass surfaces with balanced spacing
