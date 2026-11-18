# Spec Delta

## Purpose

Provides hardware-accelerated FFmpeg encoding detection, RGBA-piped key-input HUD overlay rendering, lower-third player name cards, optional FrameMeld post-processing, and montage export orchestration with ffprobe verification.

## ADDED Requirements

### Requirement: Hardware Encoder Priority Probing
The system SHALL probe available H.264 encoders in priority order (`h264_nvenc`, `h264_qsv`, `h264_amf`, `libx264`) by verifying encoder presence and running a 1-frame smoke encode, selecting the first working encoder.

#### Scenario: Hardware encoder unavailable falls back to libx264
- **WHEN** `h264_nvenc`, `h264_qsv`, and `h264_amf` fail the 1-frame smoke encode and `libx264` succeeds
- **THEN** the probe selects `libx264` as the active encoder

### Requirement: RGBA-Piped Key HUD Overlay
The system SHALL render transparent RGBA key-input HUD frames (`W`, `A`, `S`, `D`, `Shift`, `Ctrl`, `Space`, `M1`, `M2`, `R`, `E`, `F`) synchronized to `[start_tick, end_tick]` from `InputEdge` timelines and pipe them directly into FFmpeg `overlay` without writing intermediate frame files.

#### Scenario: Key HUD reflects active button state at frame timestamp
- **WHEN** an `InputEdge` sets `KeyW | KeyFire` at tick `T` within `[start_tick, end_tick]`
- **THEN** the composited video frame corresponding to tick `T` displays `W` and `M1` highlighted in the HUD overlay

### Requirement: Lower-Third Player Name Card Rendering
The system SHALL render a lower-third player name card PNG containing the player's avatar (or fallback), nickname, round number, and highlight tag, and composite it only onto real gameplay clip segments.

#### Scenario: Name card excluded from non-gameplay segments
- **WHEN** a montage contains an Intro, a Radar intro segment, a highlight clip, and an Outro
- **THEN** the lower-third name card is overlaid only on the highlight clip segment

### Requirement: Montage Composition and Media Verification
The system SHALL compose montages supporting Intro, hard-cut Radar intro segments before target clips, trimmed highlight clips, `xfade` transitions (deduplicating transition group files), subtitles, BGM mixing with `afade`, and Outro, then verify the exported MP4 via `ffprobe` before recording it in `media_videos` and `montage_exports`.

#### Scenario: Radar intro segment forces hard-cut boundaries and avoids xfade duplication
- **WHEN** a radar intro segment is inserted before clip `i` in a multi-clip montage with `xfade` transitions enabled
- **THEN** the boundaries immediately before and after the radar segment use hard cuts, each `xfade` group output is concatenated exactly once, and the resulting MP4 passes `ffprobe` duration and stream checks before marking `montage_exports` completed
