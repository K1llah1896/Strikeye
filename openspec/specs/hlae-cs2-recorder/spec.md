# hlae-cs2-recorder Specification

## Purpose

Defines zero-VPK HLAE `-customLoader` + `mirv_streams` + `mirv_cmd addAtTick` capture execution, `-con_logfile` event bridging, composer MP4 muxing, and `ffprobe` quality verification.

## Requirements

### Requirement: Zero-VPK HLAE launch and insecure flags
The launcher SHALL invoke `HLAE.exe -customLoader -noGui -autoStart` pointing to `AfxHookSource2.dll` and `cs2.exe` with `-insecure -novid -nojoy -console -sw -w <width> -h <height> -con_logfile _strikeye_session_<id>.log +exec _strikeye_boot_<id>.cfg`, never modifying VPKs or `gameinfo.gi` and never passing `-secure` or `+connect`.

#### Scenario: Launch arguments exclude VPK edits and online flags
- **WHEN** the launcher builds the HLAE command line and boot CFG for a batch session
- **THEN** `-insecure` is present, `-secure` and `+connect` are absent, and no files in `csgo/` VPKs or `gameinfo.gi` are modified

### Requirement: Tick-accurate mirv_cmd scheduling and demo_end_guard
The launcher SHALL schedule `spec_mode 5`, `spec_player <slot>`, `mirv_streams record start`, and `mirv_streams record end` via `mirv_cmd addAtTick` with a 128-tick pre-seek warmup, clamping every segment `end_tick` to at most `demo_end_tick - ceil(1.5 * tick_rate)`.

#### Scenario: Segment end tick clamped before demo EOF
- **WHEN** a segment's requested `end_tick` exceeds `demo_end_tick - ceil(1.5 * tick_rate)`
- **THEN** the generated CFG clamps `end_tick` to the safe boundary before emitting `mirv_cmd addAtTick`

### Requirement: Console logfile event bridge
The log bridge SHALL tail `_strikeye_session_<id>.log` in real time to detect `STRIKEYE_DEMO_LOADED`, `STRIKEYE_EVT:START:<task_id>:<seg_idx>`, and `STRIKEYE_EVT:END:<task_id>:<seg_idx>`, propagating cancellation or timeout errors cleanly.

#### Scenario: Log bridge emits segment start and end events
- **WHEN** CS2 writes `STRIKEYE_EVT:START` and `STRIKEYE_EVT:END` markers to `-con_logfile`
- **THEN** the bridge delivers matching segment lifecycle events to the recorder session

### Requirement: Composer MP4 muxing and ffprobe quality gate
After raw `mirv_streams` capture completes for a task, the recorder SHALL mux the video/audio into MP4 using `internal/composer` (hardware encoder probe, optional key-input HUD overlay, optional FrameMeld) and verify duration, FPS, and stream integrity via `ffprobe` before persisting to `media_videos`.

#### Scenario: Verified MP4 persisted to media_videos
- **WHEN** FFmpeg muxing finishes and `ffprobe` confirms valid video/audio streams and non-zero duration
- **THEN** the output MP4 is registered in `media_videos` and `render_tasks.output_media_id` is updated with status `completed`

#### Scenario: Corrupt output fails quality gate
- **WHEN** `ffprobe` reports missing streams, zero duration, or unreadable container
- **THEN** the task is marked `failed` (or retried if `retry_count < max_retries`) without registering a broken `media_videos` row
