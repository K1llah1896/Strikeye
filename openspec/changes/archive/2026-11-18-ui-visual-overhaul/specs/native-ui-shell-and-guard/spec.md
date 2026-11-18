## MODIFIED Requirements

### Requirement: Main navigation shell and live agent status bar
The main window SHALL use a dark esports theme (`#0F141C` background, `#171E29` cards, `#F59E0B` / `#38BDF8` accents) bound to the UI context so native controls (inputs, selects, checkboxes, sliders, scrollbars) inherit dark surfaces, with `Rajdhani` applied to numeric metrics and `Noto Sans SC` applied to CJK text, a two-tier left navigation rail for all five views, a top status bar displaying live agent state and queued task counts via IPC event subscription, and a centered modal overlay for render-now confirmation.

#### Scenario: Live task status update
- **WHEN** `strikeye-agent` publishes a demo parse or render task state change event
- **THEN** the top status bar updates its status indicator and pending task count without requiring a manual refresh

#### Scenario: Render-now confirmation uses centered modal overlay
- **WHEN** the user clicks "Start Render Now" in the UI
- **THEN** a centered modal overlay appears above the workspace without shifting or compressing the main layout flow
