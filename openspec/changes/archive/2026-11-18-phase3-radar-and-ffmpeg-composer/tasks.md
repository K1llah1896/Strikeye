# Tasks

## 1. Fonts, Assets & Go Dependencies

- [x] 1.1 Copy `Rajdhani-Bold.ttf`, `Rajdhani-SemiBold.ttf`, `NotoSansSC-Medium.ttf`, and `NotoSansSC-Bold.ttf` into `assets/fonts/` and create an embedded font loader package (`assets/fonts/embed.go`) with unit test verifying all 4 fonts parse via `golang.org/x/image/font/opentype`.
- [x] 1.2 Add `github.com/fogleman/gg` and `golang.org/x/image` to `go.mod` and verify `go build ./...` succeeds.

## 2. Six-Axis Radar Engine (`internal/radar`)

- [x] 2.1 Implement `internal/radar/model.go` for 6-axis stat derivation (`KPR 0.85`, `Surviving 0.44`, `ADR 85.0`, `KAST 0.78`, `Multi-kill 8`, `Rating 1.30`), `1.6` normalization ceiling, and match-average calculation, with unit tests covering clamping, zero-round safety, and match averages.
- [x] 2.2 Implement `internal/radar/card_renderer.go` to render `2560x1440` neon radar cards (grid, watermark, blue full-scale ring, red match-average hexagon, CT/T glowing polygon, CJK/Latin labels, circular 640px portrait or initial placeholder, and 1.8x radial-feathered team logo backdrop), with unit tests verifying PNG dimensions and appearance variants.
- [x] 2.3 Implement `internal/radar/anim_generator.go` with the monotonic `ease-in (0-30%) -> ease-out-cubic (30-60%) -> freeze (60-100%)` curve, multi-core RGBA frame rendering piped to FFmpeg (`1920x1080 @ 24fps`, 96 frames), and portrait/logo update regeneration or fallback cleanup, with unit tests verifying curve monotonicity (no overshoot), MP4 output via `ffprobe`, and appearance-change lifecycle.
- [x] 2.4 Add a radar service/store helper integrating `internal/radar` with `internal/storage.AnalysisRepository` (`UpsertRadarCard`, `ListRadarCards`) and `config.DataLayout`, verified by an integration test using SQLite.

## 3. FFmpeg Post-Processing & Montage Composer (`internal/composer`)

- [x] 3.1 Implement `internal/composer/encoder_probe.go` to probe `h264_nvenc -> h264_qsv -> h264_amf -> libx264` with 1-frame smoke encoding and fallback, plus `ffprobe` media verification helper, verified by unit/integration tests against local `ffmpeg`/`ffprobe`.
- [x] 3.2 Implement `internal/composer/key_overlay_render.go` to map `parser.InputEdge` timelines to frame timestamps and stream transparent RGBA key HUD frames (`W/A/S/D`, `Shift`, `Ctrl`, `Space`, `M1/M2`, `R/E/F`) into FFmpeg `overlay`, verified by unit tests for tick-to-button lookup and an FFmpeg overlay smoke test.
- [x] 3.3 Implement `internal/composer/name_card_render.go` to generate lower-third player name card PNGs (avatar/fallback, nickname, round number, highlight tag) and verify dimensions and alpha channel in unit tests.
- [x] 3.4 Implement `internal/composer/framemeld_adapter.go` for optional FrameMeld CLI invocation and graceful no-op/skip when unconfigured, verified by unit tests.
- [x] 3.5 Implement `internal/composer/montage_builder.go` supporting Intro, hard-cut Radar intro segments, trimmed highlight clips, gameplay-only name cards, deduplicated `xfade` transition groups, subtitles, BGM `afade` mixing, Outro, `ffprobe` verification, and `media_videos` / `montage_exports` persistence, verified by an end-to-end FFmpeg synthetic-clip export test checking duration, audio/video sync, and DB records.

## 4. Validation & CLI Integration

- [x] 4.1 Run `go test ./...` and `openspec validate phase3-radar-and-ffmpeg-composer --strict` and confirm all tests and spec validations pass.
