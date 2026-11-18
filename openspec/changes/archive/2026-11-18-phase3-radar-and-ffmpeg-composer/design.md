# Design

## Context

Phase 1 and Phase 2 already provide `internal/config` (`DataLayout`, `ToolPaths`, `RenderConfig`), `internal/storage` (`radar_cards`, `montage_projects`, `montage_exports`, `media_videos`, `highlight_clips` tables and repositories), and `internal/parser` (`InputEdge`, `Key*` bitmasks, `ReplayRoundData`). See `proposal.md` for motivation and `specs/` for behavioral requirements.

## Goals / Non-Goals

**Goals:**
- Pure-Go 2D rendering (zero CGO) for radar cards, radar animations, key HUD overlays, and player name cards using embedded open-source fonts (`Rajdhani` and `NotoSansSC`).
- Zero intermediate PNG dumps on disk for 96-frame radar animations and key HUD overlays by streaming raw RGBA buffers directly into `ffmpeg` stdin pipes.
- Deterministic, testable FFmpeg command builders and `ffprobe` verification gates before any media row is marked `verified` or `completed`.

**Non-Goals:**
- HLAE/CS2 live demo recording orchestration (Phase 4).
- MyGo Native UI screens and interactive montage timeline editor (Phase 5).
- Removal of legacy Python/React/OBS directories (Phase 6).

## Decisions

1. **Pure-Go 2D Graphics + Embedded Fonts (`assets/fonts/`)**
   - *Choice*: Use `github.com/fogleman/gg` + `golang.org/x/image/font/opentype` with `go:embed` font files (`Rajdhani-Bold.ttf`, `Rajdhani-SemiBold.ttf`, `NotoSansSC-Medium.ttf`, `NotoSansSC-Bold.ttf`) migrated from the legacy repository.
   - *Rationale*: Avoids CGO/Skia/Cairo build complexity on Windows and Linux while supporting crisp anti-aliased polygons, radial feathering, and CJK + Latin typography.
   - *Alternative*: Calling ImageMagick/Pillow subprocesses—rejected to eliminate Python/external image tool dependencies.

2. **RGBA Pipe Streaming for Key HUD & Radar Animation (`Plan A`)**
   - *Choice*: Stream `image.RGBA.Pix` (`-f rawvideo -pix_fmt rgba`) directly to FFmpeg stdin pipes (`anim_generator.go` and `key_overlay_render.go`).
   - *Rationale*: Eliminates hundreds/thousands of temporary PNG files on disk, reduces I/O overhead, and works seamlessly with `montage_builder` / post-processing filters.
   - *Alternative*: Pre-rendering PNG sequences or `.mov` alpha intermediates—rejected due to disk churn and cleanup complexity.

3. **Multi-kill Dimension Definition**
   - *Choice*: Use `MultiKillRounds` (2K+3K+4K+5K rounds) normalized against full-scale `8.0` rounds (`STRIKEYE_DEVELOPMENT_DOCUMENTATION.md` §7.3), clamped to `NORMALIZE_CEILING = 1.6`.
   - *Rationale*: Matches the primary Strikeye architecture doc and directly maps to `MatchPlayer.MultiKillRounds` already stored in SQLite.

4. **Montage Segment & Hard-Cut Grouping**
   - *Choice*: Partition clips into contiguous `xfade` groups split at any `radar_segments` insertion index (`before_clip_index`), concatenate each group once (`first_row_of_group` guard), and apply `name_card_render` only to real gameplay segments.
   - *Rationale*: Prevents the legacy `xfade` group duplication bug and ensures radar intro cards are never distorted by cross-fade transitions or overlaid with clip name cards.

## Risks / Trade-offs

- **FFmpeg filtergraph complexity on long montages** -> Normalize all segments to uniform resolution/fps/audio (`48000 Hz` stereo AAC) during per-segment preparation before `xfade` / `concat`, keeping filtergraphs predictable and isolating failures.
- **CPU usage during 96-frame 2560x1440 animation rendering** -> Bound worker goroutines to `runtime.NumCPU()` (or configurable worker count) and reuse per-worker RGBA buffers where safe.
- **No GPU encoder in headless CI/Linux** -> `encoder_probe.go` gracefully falls back to `libx264` so all rendering and montage tests run deterministically everywhere.
