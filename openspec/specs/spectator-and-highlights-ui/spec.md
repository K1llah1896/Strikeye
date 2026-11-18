# spectator-and-highlights-ui Specification

## Purpose

Defines the Spectator multi-POV video playback center with tick-synchronized event timelines and key HUDs, as well as the Highlights library and Montage Workbench for trimming and exporting videos.

## Requirements

### Requirement: Strict real-MP4 vs unrendered POV separation in Spectator
The `Spectator` view SHALL display an overflow-safe scrollable round selector and a CT/T/Observer grouped 10-player + observer switcher annotated with native vector status indicators (`ready`, `queued`, `unrendered`, without relying on emoji glyphs) from `media_videos`, playing verified MP4s on ready selection and showing an unrendered summary card with "Enqueue Render" and "Start Render Now" actions instead of substituting another player's video.

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

### Requirement: Parsed highlight tag pills and localized clip types
The `Highlight Library` clip cards SHALL parse `TagsJSON` into clean visual tag pills (falling back gracefully if non-JSON) and render localized clip-type labels (e.g., `multi_kill` -> `多杀`, `clutch` -> `残局`) instead of raw JSON arrays or snake_case strings.

#### Scenario: Rendering a highlight clip with JSON tags
- **WHEN** a highlight clip has `ClipType: "multi_kill"` and `TagsJSON: '["4K","Headshot"]'`
- **THEN** the card displays `多杀` and separate `4K`, `Headshot` pills without raw brackets or quotes

### Requirement: Lighter Spectator round selector with overflow cue
The `Spectator` round selector SHALL use compact, low-chrome round pills inside its horizontal scroll strip and provide a subtle overflow indicator or scroll hint when rounds exceed the visible width.

#### Scenario: Viewing a 23-round match in Spectator
- **WHEN** the user opens `Spectator` for a 23-round match at `1360x840`
- **THEN** the round selector displays compact pills with a clear scroll/overflow cue instead of heavy bordered boxes clipped abruptly at the edge

### Requirement: Clean localized unrendered POV badge in Spectator
The `Spectator` unrendered POV summary card SHALL display `未生成独立视角视频` without raw English parenthetical suffixes such as `(unrendered)`.

#### Scenario: Viewing an unrendered POV in Spectator
- **WHEN** the user selects a player/round with no verified MP4 in `Spectator`
- **THEN** the status capsule reads `未生成独立视角视频` with no raw English enum suffix

### Requirement: Clean highlight cards without tick range and with non-wrapping single-line tags
The `Highlight Library` clip cards SHALL omit raw tick ranges (`Ticks: xxx ~ xxx`) and SHALL render parsed tag pills inside a single-line horizontal scroll container with non-shrinking pill items so tag text never wraps across multiple lines.

#### Scenario: Viewing a highlight clip card with multiple tags
- **WHEN** a highlight clip has multiple tags (including CJK and emoji labels) in a narrow `420px` list column
- **THEN** the card does not display `Ticks: ...` and all tag pills remain on a single horizontal line scrollable without wrapping text internally
