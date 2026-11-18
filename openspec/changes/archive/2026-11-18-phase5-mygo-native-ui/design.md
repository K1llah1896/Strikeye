# Design: Phase 5 — MyGo Native UI Desktop Client

## Context

See `proposal.md` for motivation and capability scope. Phases 1–4 already provide:
- `internal/config`: `Load`, `Save`, `Probe`, `DataLayout`
- `internal/storage`: SQLite WAL repositories (`DemoRepository`, `AnalysisRepository`, `MediaRepository`, `RenderTaskRepository`, `MontageRepository`)
- `internal/parser`: `ReadReplayIndex`, `ReadReplayRound` (`CS2RPL01` 32Hz binary frames + `ReplayPlayerPose` + `m_nButtons`)
- `internal/radar`: `Service` for `2560x1440` cards and `4.0s` intro MP4s
- `internal/composer`: `BuildMontage` FFmpeg pipeline
- `internal/ipc` & `cmd/strikeye-agent`: `MutexUISingleton`, `MutexCS2RenderActive`, Named Pipe JSON-RPC 2.0 + event bus (`agent.ping`, `demo.list`, `demo.parse`, `render.status`, `render.start_now`, `render.abort_and_cleanup`, `render.enqueue`)

The remaining gap is the pure-Go MyGo Native UI desktop client (`cmd/strikeye`, `internal/ui/*`), map radar assets (`assets/maps/`), and four additional agent IPC methods (`montage.export`, `render.cancel`, `render.retry`, `config.reload`).

## Goals / Non-Goals

**Goals:**
- 100% pure-Go desktop client using `github.com/egoist/mygo/ui` (no CGO, no WebView2, no HTML/JS/React).
- Strict UI/CS2 mutual exclusion via `Local\StrikeyeCS2RenderActiveMutex` and `Local\StrikeyeUISingletonMutex`.
- Embedded FFmpeg-backed video/audio playback inside MyGo Native UI with exact tick synchronization for Event Timelines, POV switching, Key HUDs, and dual-slider `In/Out` trimming.
- Smooth 32Hz 2D Replay & KDE Heatmap rendering from `CS2RPL01` binary frames.
- Clean separation between fast SQLite WAL read queries in the UI and stateful background mutations over JSON-RPC 2.0 IPC.

**Non-Goals:**
- Removing legacy `./CS2-insight-agent-main` files or running the full Phase 6 cleanup/acceptance suite (deferred to Phase 6).
- Modifying demo parsing schemas or HLAE/CS2 capture scripts.

## Decisions

### 1. Embedded FFmpeg Video & Audio Playback (`internal/ui/components/video_player.go`)
- **Decision**: Implement a pure-Go `VideoPlayer` controller backed by `ffmpeg` frame-stream decoding (RGBA frames piped into a MyGo GPU image/texture surface) paired with `ffmpeg` PCM audio playback/seek synchronization, rather than shelling out to an external player (`ffplay` or OS default).
- **Rationale**: External players cannot embed inside the Spectator/Highlights workspace, cannot sync with the Event Timeline below the progress bar, cannot preserve round-relative tick position when switching between rendered POVs, and cannot drive the tick-synchronized Key HUD.
- **Alternative Considered**: Launching `ffplay` as a child window; rejected because it breaks integrated layout, keyboard/mouse scrubbing, and tick-accurate overlays.

### 2. Read-Local SQLite WAL + Write-via-IPC Architecture
- **Decision**: `strikeye.exe` opens `strikeye.db` in read-only/WAL mode (`internal/storage`) for instant list/filter/scoreboard/event queries, while all background mutations (`demo.parse`, `render.enqueue`, `render.cancel`, `render.retry`, `render.start_now`, `render.abort_and_cleanup`, `montage.export`, `config.reload`) go through `internal/ipc` to `strikeye-agent`.
- **Rationale**: Avoids serializing large 10-player scoreboards, event lists, and highlight catalogs over Named Pipes while keeping a single writer/orchestrator (`strikeye-agent`) for `render_tasks`, `config.json`, and FFmpeg/HLAE processes.

### 3. Startup Mutual-Exclusion & Guard Dialog (`cmd/strikeye` + `internal/ui/guard`)
- **Decision**: Before creating any main MyGo window, `cmd/strikeye/main.go` checks `Local\StrikeyeCS2RenderActiveMutex`. If held, it opens only the lightweight Guard Dialog polling `render.status` at 500ms intervals. Only after the render mutex is free does it acquire `Local\StrikeyeUISingletonMutex`, auto-launch `strikeye-agent` (via `CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS` on Windows) if `agent.ping` fails, and start the main shell.
- **Rationale**: Guarantees CS2 and the main Strikeye UI never coexist, and ensures double-clicking `strikeye.exe` during a render gives immediate feedback and a safe abort path.

### 4. Map Radar Assets (`assets/maps/`) with Procedural Grid Fallback
- **Decision**: Bundle map radar PNGs and coordinate transform JSONs (`pos_x`, `pos_y`, `scale`, optional `z_cutoff` for multi-layer maps like Nuke/Vertigo) in `assets/maps/` via `go:embed`. When a map's PNG is absent, `radar_2d_canvas.go` draws a clean dark tactical coordinate grid using the replay's bounding box so 2D Replay and Heatmaps still function on any map.
- **Rationale**: Prevents blank screens on community/new maps while providing authentic radar backgrounds for standard competitive maps.

### 5. Extending `strikeye-agent` IPC for Phase 5 Actions
- **Decision**: Add `montage.export` (runs `composer.BuildMontage` asynchronously and emits `montage.progress` / `montage.completed` events), `render.cancel`, `render.retry`, and `config.reload` to `cmd/strikeye-agent/main.go`.
- **Rationale**: Keeps long-running FFmpeg montage exports and task state transitions managed by the background agent with live progress pushed to the UI.

## Risks / Trade-offs

- **[Risk] High-FPS 1080p/1440p FFmpeg pipe decoding CPU overhead in UI** -> **Mitigation**: Scale decoded preview frames to the player viewport resolution (e.g., max 1280x720 or 1920x1080) inside `ffmpeg` (`-vf scale=...`) and decode on a dedicated goroutine with frame-drop/skip catch-up so the MyGo UI thread remains locked at smooth frame rates.
- **[Risk] `github.com/egoist/mygo/ui` API differences or cross-compilation quirks from Linux** -> **Mitigation**: Verify `GOOS=windows go build ./cmd/strikeye` early in Task 1 and isolate all MyGo widget/canvas calls behind `internal/ui/theme` and `internal/ui/components` adapters so business/state logic stays unit-testable in pure Go without a display server.
- **[Risk] Audio/Video drift on seek or speed change (`0.25x`–`2.0x`)** -> **Mitigation**: Restart/seek the `ffmpeg` decode pipeline from the exact target timestamp on scrub, event-anchor click, or speed change (`atempo` filter for audio), using monotonic audio/clock timestamps as the master sync source.
