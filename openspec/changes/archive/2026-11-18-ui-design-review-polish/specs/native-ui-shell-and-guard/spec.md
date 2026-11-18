# Spec Delta

## ADDED Requirements

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
