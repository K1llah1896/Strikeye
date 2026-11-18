# Spec Delta

## ADDED Requirements

### Requirement: Theme configuration persistence and normalization
The system SHALL persist a `theme` configuration block in `config.json` containing the selected preset name, custom accent color hex, custom background image path, background overlay opacity, and card opacity, and SHALL normalize missing or out-of-range values to safe defaults during load.

#### Scenario: Missing or invalid theme fields on load
- **WHEN** `config.json` omits the `theme` block or contains an unrecognized preset name, invalid accent hex, or out-of-range opacity
- **THEN** configuration normalization fills the default esports dark preset, `#F59E0B` accent color, and clamped opacity values without failing startup

#### Scenario: Saving updated theme settings
- **WHEN** the user saves theme changes from the UI
- **THEN** `config.json` is updated atomically with the new `theme` block and remains valid JSON on disk
