# Spec Delta

## MODIFIED Requirements

### Requirement: Main navigation shell and live agent status bar
The main window SHALL use the active Strikeye theme palette (defaulting to the dark esports preset `#0F141C` background, `#171E29` cards, `#F59E0B` / `#38BDF8` accents) bound to the UI context so native controls (inputs, selects, checkboxes, sliders, scrollbars) inherit themed surfaces, with `Rajdhani` applied to numeric metrics and `Noto Sans SC` applied to CJK text, a two-tier left navigation rail for all five views, a top status bar displaying live agent state, active parse progress, and queued task counts via IPC event subscription, and a centered modal overlay for render-now confirmation.

#### Scenario: Live task status update
- **WHEN** `strikeye-agent` publishes a demo parse or render task state change event
- **THEN** the top status bar updates its status indicator and pending task count without requiring a manual refresh

#### Scenario: Render-now confirmation uses centered modal overlay
- **WHEN** the user clicks "Start Render Now" in the UI
- **THEN** a centered modal overlay appears above the workspace without shifting or compressing the main layout flow

## ADDED Requirements

### Requirement: Customizable theme presets, accent colors, and background images
The UI theme system SHALL support switching between built-in theme presets (including dark esports and light/alternative presets), overriding the primary accent color via preset swatches or hex input, and rendering a user-selected local background image behind the main workspace with a configurable readability overlay and card opacity.

#### Scenario: Applying a custom accent color and background image
- **WHEN** the user selects a theme preset, enters a valid hex accent color, and selects a local PNG/JPG background image
- **THEN** the main shell renders the cached background image with the configured overlay tint, applies the accent color to active indicators and primary buttons, and updates all views immediately without restarting

#### Scenario: Invalid or missing background image fallback
- **WHEN** the configured background image path does not exist or fails to decode
- **THEN** the shell falls back cleanly to the solid theme base background color without crashing or blocking rendering

### Requirement: Appearance settings in Settings & Tasks
The `Settings & Tasks` view SHALL include an Appearance & Theme configuration section allowing users to choose a theme preset, pick or input an accent color, specify or clear a background image path, adjust overlay and card opacity sliders, and persist the configuration.

#### Scenario: Saving appearance configuration
- **WHEN** the user adjusts theme settings in `Settings & Tasks` and clicks save
- **THEN** the active UI theme updates immediately and the settings are written to `config.json`

### Requirement: Reusable progress bar and non-blocking async view loading
The UI shell SHALL provide a reusable progress bar primitive supporting determinate (`0.0–1.0`) and indeterminate loading states, and SHALL execute heavy demo selection transitions (`Match Analysis`, `Spectator`, `Highlights`) asynchronously so the UI remains responsive and displays a loading progress bar until data is ready.

#### Scenario: Navigating to a heavy view displays a loading progress bar
- **WHEN** the user clicks to open `Match Analysis` for a parsed demo
- **THEN** the workspace switches immediately and displays a loading progress bar with stage text until SQLite queries and initial canvas setup complete
