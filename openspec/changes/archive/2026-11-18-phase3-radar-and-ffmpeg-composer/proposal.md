# Proposal

## Why

With Phase 1 (config, SQLite storage, IPC) and Phase 2 (demo watcher, single-pass parser, metrics, highlights, and `CS2RPL01` replay/input binary cache) complete, Strikeye needs Phase 3 to turn parsed match metrics, `InputEdge` streams, and video clips into broadcast-grade visual assets and exported montages—replacing the legacy Python/Pillow/MoviePy pipeline with a fast, pure-Go 2D renderer and FFmpeg composition engine.

## What Changes

- **Embedded open-source fonts (`assets/fonts/`)**: Migrate `Rajdhani` (`Bold`, `SemiBold`) and `NotoSansSC` (`Medium`, `Bold`) from the legacy repository and embed them via `go:embed` for consistent CJK + Latin rendering.
- **Pure-Go 6-axis CS Data Radar engine (`internal/radar/`)**:
  - Derive player 6-axis stats (`KPR`, `Surviving`, `ADR`, `KAST`, `Multi-kill` in rounds `/ 8.0`, `Rating`) with `1.6` normalization ceiling and match-average reference polygon.
  - Render `2560x1440` neon-styled static PNG cards with circular portraits, `1.8x` radial-feathered team logo backdrops, and `radar_cards` SQLite persistence.
  - Render `4.0s @ 24fps` (96 frames) intro animations via multi-core goroutine pool and RGBA rawvideo pipe to FFmpeg (`1920x1080` MP4), following the strict monotonic `ease-in (0-30%) -> ease-out-cubic (30-60%) -> freeze (60-100%)` curve with zero overshoot and an always-visible match average polygon.
  - Automatically regenerate (or clear if FFmpeg is unavailable) cached animations when a card's portrait or team logo changes.
- **FFmpeg post-processing & montage composer (`internal/composer/`)**:
  - Probe hardware encoders in priority order (`h264_nvenc` -> `h264_qsv` -> `h264_amf` -> `libx264`) via 1-frame smoke encoding.
  - Render transparent key-input HUD frames from `CS2RPL01` `InputEdge` timelines and stream them as an RGBA pipe into FFmpeg `overlay` filters.
  - Render lower-third player name cards (`name_card_render.go`) applied only to real gameplay clips.
  - Adapt optional FrameMeld CLI post-processing (`framemeld_adapter.go`).
  - Implement `montage_builder.go` supporting Intro, hard-cut radar intro segments, trimmed highlight clips, `xfade` transitions (with deduplicated group concatenation), subtitles, BGM mixing with `afade`, Outro, `ffprobe` verification, and `media_videos` / `montage_exports` persistence.

## Capabilities

### New Capabilities
- `radar-engine`: Pure-Go 6-axis CS Data Radar derivation, `2560x1440` static card rendering, `4.0s` intro animation encoding, and portrait/team-logo regeneration lifecycle.
- `ffmpeg-composer`: Hardware encoder probing, RGBA-piped key HUD overlay, lower-third player name cards, optional FrameMeld adapter, and FFmpeg montage export pipeline with `ffprobe` verification.

### Modified Capabilities
<!-- None: all existing Phase 1/2 specs remain unchanged -->

## Impact

- **New packages**: `assets/fonts/`, `internal/radar/`, `internal/composer/`.
- **Dependencies**: Adds pure-Go 2D drawing (`github.com/fogleman/gg`) and OpenType font rendering (`golang.org/x/image`) to `go.mod`.
- **Existing packages consumed**: `internal/config` (paths & render defaults), `internal/storage` (`AnalysisRepository`, `MediaRepository`, `MontageRepository`), `internal/parser` (`InputEdge`, `Key*` masks, `ReplayRoundData`).
- **Runtime tools**: Uses `ffmpeg` and `ffprobe` binaries configured in `ToolPaths` (or discovered in `PATH`).
