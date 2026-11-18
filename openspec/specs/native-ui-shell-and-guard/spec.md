# native-ui-shell-and-guard Specification

## Purpose

Defines the standalone MyGo Native UI desktop shell, startup render-mutex guard dialog, agent lifecycle coordination, dark esports theme, and the Settings & Tasks management view.

## Requirements

### Requirement: Render mutex guard dialog on startup
When `strikeye.exe` starts while `Local\StrikeyeCS2RenderActiveMutex` is held, the application SHALL NOT open the main UI window and SHALL instead display a lightweight native Guard Dialog polling `render.status` over IPC.

#### Scenario: Continue background rendering
- **WHEN** the user clicks "Continue Background Rendering" in the Guard Dialog
- **THEN** `strikeye.exe` exits immediately without disturbing the active HLAE/CS2 render session

#### Scenario: Safely abort rendering and open UI
- **WHEN** the user clicks "Safely Abort & Open Strikeye" in the Guard Dialog
- **THEN** `strikeye.exe` sends `render.abort_and_cleanup` over IPC, waits for `Local\StrikeyeCS2RenderActiveMutex` to be released, acquires `Local\StrikeyeUISingletonMutex`, and opens the main UI window

### Requirement: UI singleton, agent auto-launch, and render-now handoff
When no render mutex is active, `strikeye.exe` SHALL acquire `Local\StrikeyeUISingletonMutex`, automatically launch `strikeye-agent` in the background if `agent.ping` fails, and render the main MyGo Native UI window without any WebView, HTML, or JS runtime.

#### Scenario: User triggers immediate render from UI
- **WHEN** the user clicks "Start Render Now" in the UI and confirms the modal prompt
- **THEN** `strikeye.exe` sends `render.start_now` to `strikeye-agent` and terminates cleanly so the agent can acquire the render mutex

### Requirement: Main navigation shell and live agent status bar
The main window SHALL use the active Strikeye theme palette (defaulting to the dark esports preset `#0F141C` background, `#171E29` cards, `#F59E0B` / `#38BDF8` accents) bound to the UI context so native controls (inputs, selects, checkboxes, sliders, scrollbars) inherit themed surfaces, with `Rajdhani` applied to numeric metrics and `Noto Sans SC` applied to CJK text, a two-tier left navigation rail for all five views, a top status bar displaying live agent state, active parse progress, and queued task counts via IPC event subscription, and a centered modal overlay for render-now confirmation.

#### Scenario: Live task status update
- **WHEN** `strikeye-agent` publishes a demo parse or render task state change event
- **THEN** the top status bar updates its status indicator and pending task count without requiring a manual refresh

#### Scenario: Render-now confirmation uses centered modal overlay
- **WHEN** the user clicks "Start Render Now" in the UI
- **THEN** a centered modal overlay appears above the workspace without shifting or compressing the main layout flow

### Requirement: Settings & Tasks view and agent IPC extensions
The `Settings & Tasks` view SHALL provide environment probing (`cs2.exe`, `HLAE.exe`, `ffmpeg.exe`, `ffprobe.exe`), directory and storage management (backup DB & clean invalid cache), render policy/quality configuration, and a live `render_tasks` queue with priority, cancel, and retry controls backed by `render.cancel`, `render.retry`, `config.reload`, and `montage.export` IPC handlers.

#### Scenario: Retry failed task and reload config
- **WHEN** the user clicks retry on a failed render task or saves updated settings
- **THEN** the UI invokes `render.retry` or saves config and calls `config.reload` over IPC, updating the task queue or environment status immediately

### Requirement: Distinct borderless secondary tab styling
Secondary workspace tabs (`TabButton`) SHALL use a borderless bottom-indicator visual style (active bottom accent bar and crisp text contrast) so they are visually distinct from bordered action buttons (`ActionButton`, `AccentButton`).

#### Scenario: Active vs inactive secondary tab
- **WHEN** a user views a multi-tab workspace such as `Match Analysis` or `Highlights`
- **THEN** the active tab displays a bottom accent indicator without a heavy outer action-button border, while inactive tabs remain muted and flat

### Requirement: OS-aware tool labels and compact Settings & Tasks layout
The `Settings & Tasks` view SHALL display OS-appropriate binary labels (omitting `.exe` suffixes for `ffmpeg` and `ffprobe` on non-Windows platforms) and keep the watch-directory editor compact so the `render_tasks` queue section remains accessible without excessive vertical scrolling.

#### Scenario: Viewing Settings & Tasks on Linux
- **WHEN** the user opens `Settings & Tasks` on Linux with `/usr/bin/ffmpeg` detected
- **THEN** the tool row displays `ffmpeg` and `ffprobe` without `.exe` suffixes and the task queue section sits higher in the viewport

### Requirement: Unclipped Settings & Tasks queue visibility at default window height
The `Settings & Tasks` view SHALL use compact vertical card padding and spacing so `4. 后台渲染任务队列` and its empty-state prompt remain unclipped inside the `1360x840` main window.

#### Scenario: Viewing Settings & Tasks at 1360x840
- **WHEN** the user opens `Settings & Tasks` at the default `1360x840` window size with an empty render queue
- **THEN** the `4. 后台渲染任务队列` header and its empty-state message fit cleanly above the bottom edge without half-line clipping

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
