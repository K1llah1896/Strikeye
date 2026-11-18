# Spec Delta

## ADDED Requirements

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
