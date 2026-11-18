# Spec Delta: native-ui-shell-and-guard

## Purpose

Defines the standalone MyGo Native UI desktop shell, startup render-mutex guard dialog, agent lifecycle coordination, dark esports theme, and the Settings & Tasks management view.

## ADDED Requirements

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
The main window SHALL use a dark esports theme (`#0F141C` background, `#171E29` cards, `#F59E0B` / `#38BDF8` accents) with a left navigation rail for all five views and a top status bar displaying live agent state and queued task counts via IPC event subscription.

#### Scenario: Live task status update
- **WHEN** `strikeye-agent` publishes a demo parse or render task state change event
- **THEN** the top status bar updates its status indicator and pending task count without requiring a manual refresh

### Requirement: Settings & Tasks view and agent IPC extensions
The `Settings & Tasks` view SHALL provide environment probing (`cs2.exe`, `HLAE.exe`, `ffmpeg.exe`, `ffprobe.exe`), directory and storage management (backup DB & clean invalid cache), render policy/quality configuration, and a live `render_tasks` queue with priority, cancel, and retry controls backed by `render.cancel`, `render.retry`, `config.reload`, and `montage.export` IPC handlers.

#### Scenario: Retry failed task and reload config
- **WHEN** the user clicks retry on a failed render task or saves updated settings
- **THEN** the UI invokes `render.retry` or saves config and calls `config.reload` over IPC, updating the task queue or environment status immediately
