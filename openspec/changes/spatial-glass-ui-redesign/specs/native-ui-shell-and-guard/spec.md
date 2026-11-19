# Spec Delta

## MODIFIED Requirements

### Requirement: Main navigation shell and live agent status bar
The main window SHALL use the active Strikeye Spatial Glass theme palette (defaulting to the Spatial Obsidian preset with an obsidian `#05070B` base, ambient radial-glow backdrop, translucent frosted glass surfaces, and luminous `#38BDF8` / `#22C55E` accents) bound to the UI context so native controls inherit themed surfaces, with `Rajdhani` applied to numeric metrics and `Noto Sans SC` applied to CJK text, a floating glass left navigation rail with vector icons for all five views, a seamless top status bar displaying live agent state, active parse progress, and queued task counts via IPC event subscription, and a centered glass modal overlay for render-now confirmation.

#### Scenario: Live task status update
- **WHEN** `strikeye-agent` publishes a demo parse or render task state change event
- **THEN** the top status bar updates its status indicator and pending task count without requiring a manual refresh

#### Scenario: Render-now confirmation uses centered modal overlay
- **WHEN** the user clicks "Start Render Now" in the UI
- **THEN** a centered glass modal overlay appears above the workspace without shifting or compressing the main layout flow

## ADDED Requirements

### Requirement: Spatial Glass ambient backdrop and translucent card primitives
The UI theme system SHALL render an ambient radial-glow backdrop behind workspace panels by default and provide reusable translucent glass card, Bento KPI tile, pill badge, and segmented pill tab primitives that use subtle rim highlights and spatial shadows instead of heavy solid 1px borders.

#### Scenario: Default Spatial Obsidian glass rendering
- **WHEN** the user launches Strikeye with the default theme configuration
- **THEN** the main shell renders the ambient radial-glow backdrop and translucent frosted glass cards with rounded corners and soft spatial depth

### Requirement: Inline SVG vector iconography for navigation and controls
The UI shell and workspace views SHALL render crisp inline SVG vector icons alongside navigation rail items, primary actions, status pills, and structured empty states instead of relying solely on text labels or thin color bars.

#### Scenario: Navigation rail displays vector icons
- **WHEN** the user views the left navigation rail
- **THEN** each of the five workspace entries displays its corresponding vector icon alongside its localized title and active pill highlight
