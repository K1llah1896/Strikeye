# Spec Delta: spectator-and-highlights-ui

## Purpose

Defines the Spectator multi-POV video playback center with tick-synchronized event timelines and key HUDs, as well as the Highlights library and Montage Workbench for trimming and exporting videos.

## ADDED Requirements

### Requirement: Strict real-MP4 vs unrendered POV separation in Spectator
The `Spectator` view SHALL display round selectors and a 10-player + observer switcher annotated with real media status (`ready`, `queued`, `unrendered`) from `media_videos`, playing verified MP4s on ready selection and showing an unrendered summary card with "Enqueue Render" and "Start Render Now" actions instead of substituting another player's video.

#### Scenario: Switch between two rendered POVs in the same round
- **WHEN** the user switches from Player A (rendered) to Player B (rendered) during round playback
- **THEN** the player loads Player B's verified MP4 and seeks to the equivalent round-relative tick/time

#### Scenario: Select an unrendered POV
- **WHEN** the user clicks a player/round combination with no verified `media_videos` entry
- **THEN** the viewport displays that player's round K/D, survival summary, and "Enqueue Render" / "Start Render Now" buttons without playing another perspective

### Requirement: Embedded FFmpeg player, event timeline anchors, and Key HUD
The embedded native video player SHALL support play/pause, seek scrubbing, `0.25x`/`0.5x`/`1.0x`/`1.5x`/`2.0x` playback speeds, volume, fullscreen toggle, a synchronized Event Timeline that seeks to `event_time - 2.0s` on click, and an optional Key HUD (`WASD`, crouch, jump, walk, fire, scope) synchronized to the video tick or explicitly stating when `svc_UserCmd` data is absent.

#### Scenario: Click kill anchor on Event Timeline
- **WHEN** the user clicks a kill or bomb event anchor below the progress bar
- **THEN** playback jumps to 2.0 seconds before that event's timestamp and the Key HUD updates to match the target tick

### Requirement: Highlight Library filtering, dual-slider trim, and re-render
The `Highlight Library` tab SHALL list detected highlights across parsed demos with filters for match, player, round, event type, tag, video readiness, and favorites, supporting embedded preview, dual-slider `In/Out` tick trimming, and re-enqueueing at the adjusted tick range.

#### Scenario: Adjust highlight In/Out points and re-render
- **WHEN** the user drags the `In/Out` trim sliders on a highlight and clicks "Re-render with New Tick Range"
- **THEN** a render task is enqueued via IPC using the updated start and end ticks

### Requirement: Montage Workbench composition and async FFmpeg export
The `Montage Workbench` tab SHALL allow users to select rendered clips, reorder sequences, set per-clip trims, toggle per-clip 4.0s CS Data Radar intro insertion, configure intro/outro, BGM, `xfade` transitions, lower-third player name cards, and subtitles, and export an MP4 asynchronously via `montage.export` with live progress updates.

#### Scenario: Export montage with radar intro and BGM
- **WHEN** the user configures a montage sequence and clicks "Export Montage MP4"
- **THEN** `strikeye-agent` runs `BuildMontage`, streams progress events to the UI, and registers the resulting MP4 in `media_videos`
