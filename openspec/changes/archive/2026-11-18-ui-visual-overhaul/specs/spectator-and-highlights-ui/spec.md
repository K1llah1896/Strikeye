## MODIFIED Requirements

### Requirement: Strict real-MP4 vs unrendered POV separation in Spectator
The `Spectator` view SHALL display an overflow-safe scrollable round selector and a CT/T/Observer grouped 10-player + observer switcher annotated with native vector status indicators (`ready`, `queued`, `unrendered`, without relying on emoji glyphs) from `media_videos`, playing verified MP4s on ready selection and showing an unrendered summary card with "Enqueue Render" and "Start Render Now" actions instead of substituting another player's video.

#### Scenario: Switch between two rendered POVs in the same round
- **WHEN** the user switches from Player A (rendered) to Player B (rendered) during round playback
- **THEN** the player loads Player B's verified MP4 and seeks to the equivalent round-relative tick/time

#### Scenario: Select an unrendered POV
- **WHEN** the user clicks a player/round combination with no verified `media_videos` entry
- **THEN** the viewport displays that player's round K/D, survival summary, and "Enqueue Render" / "Start Render Now" buttons without playing another perspective
