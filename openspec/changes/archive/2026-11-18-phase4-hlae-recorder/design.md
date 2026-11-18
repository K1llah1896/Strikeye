# Design

## Context

See `proposal.md` for motivation. Existing foundations in the repository already provide:
- `internal/config`: `RenderConfig` (`Policy`, `Width`, `Height`, `FPS`, `Encoder`, `PreBufferSec`, `PostBufferSec`, `KeyOverlayEnabled`, `FrameMeldEnabled`, `HighlightImportanceMin`), `PrimarySteamID64`, and `ProbeToolchain` (Steam/CS2/HLAE/FFmpeg/FFprobe paths).
- `internal/storage`: `RenderTasks()` (`EnqueueIdempotent`, `NextQueued`, status/PID/media updates), `Media()`, `Demos()`, and `Analysis()`.
- `internal/ipc`: `MutexUISingleton`, `MutexCS2RenderActive`, and JSON-RPC 2.0 server with event publishing.
- `internal/composer`: `ProbeEncoder`, key-input HUD RGBA overlay, and `FrameMeld` adapter.
- `cmd/strikeye-agent`: currently runs watcher + parse worker + basic RPCs, with no render scheduler or crash recovery.

The development environment is Linux, while the target runtime for HLAE/CS2 is Windows.

## Goals / Non-Goals

**Goals:**
- Implement `internal/recorder` with clean interface boundaries so the entire scheduling, CFG backup/restore, `mirv_cmd` script generation, log-bridge parsing, FFmpeg/composer muxing, and crash-recovery state machine can be tested end-to-end on Linux using a Fake Runner.
- Isolate Windows-specific process/Job Object/cs2.exe enumeration APIs in `//go:build windows` files with safe stubs in `//go:build !windows` files so `GOOS=windows` cross-compilation and native Linux tests both pass.
- Integrate crash recovery, render scheduling, and `render.*` IPC handlers into `cmd/strikeye-agent` and dry-run/self-check subcommands into `cmd/strikeye-cli`.

**Non-Goals:**
- Implementing the MyGo Native UI or Guard Dialog (deferred to Phase 5).
- Modifying any CS2 VPKs, `gameinfo.gi`, or using OBS/WebSocket/skin-changer hooks.

## Decisions

### 1. Interface abstraction for `Runner` / `ProcessGuard` / `LogBridge`
- **Decision**: Define `SessionRunner` and `ProcessGuard` interfaces in `internal/recorder` so `Scheduler` orchestrates batch sessions identically against the real Windows HLAE/JobObject implementation and an in-memory/temp-dir Fake Runner.
- **Rationale**: Allows full end-to-end verification of batching, `demo_end_guard`, `render.abort_and_cleanup`, `ffprobe` gating, and crash recovery on Linux CI without needing a Windows GPU/CS2 install.
- **Alternative**: Putting Windows syscalls directly inside `Scheduler` (rejected: untestable on Linux and couples orchestration with OS APIs).

### 2. Post-capture muxing directly via `internal/composer` before `ffprobe` gate
- **Decision**: Each completed `mirv_streams` task segment directory is immediately muxed into MP4 using `internal/composer` (hardware encoder probe + optional key-input HUD overlay from `CS2RPL01`/`InputEdge` + optional FrameMeld) and verified via `ffprobe` before inserting into `media_videos`.
- **Rationale**: Matches Section 6.3 steps 10–11 of the architecture doc and guarantees every `media_videos` entry is immediately playable by Phase 5's Spectator view.
- **Alternative**: Storing raw `mirv_streams` frames/WAV and deferring MP4 encoding to a later phase (rejected: bloats disk usage and breaks `media_videos` contract).

### 3. Batch-per-demo execution with `mirv_cmd addAtTick` + `-con_logfile`
- **Decision**: Group all `queued` tasks for the same `demo_id` sorted by `start_tick` ascending into one `_strikeye_boot_<id>.cfg` using `mirv_cmd addAtTick` (with 128-tick pre-seek warmup and `demo_end_guard` clamping) and tail `-con_logfile` for `STRIKEYE_DEMO_LOADED` and `STRIKEYE_EVT:START/END`.
- **Rationale**: Avoids restarting CS2 per clip, achieves frame-accurate tick alignment without console overlay artifacts, and requires zero VPK edits.

### 4. Atomic CFG backup manifest + Windows Job Object (`KILL_ON_JOB_CLOSE`)
- **Decision**: Write `active_restore_manifest.json` before copying or touching any CS2 cfg/video files; on Windows, assign `HLAE.exe` to a Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and track `(PID, CreationTime, SessionToken)`.
- **Rationale**: Even on power loss or `SIGKILL`, `strikeye-agent` startup calls `RestoreIfPending()` to roll back player configs and reconcile orphaned `running` tasks.

## Risks / Trade-offs

- **[Risk] `mirv_streams` output file layout varies across HLAE versions (e.g. `take0000` subfolders, AVI vs TGA/PNG + WAV)** → **Mitigation**: Implement a dedicated `resolveCapturedStreams(tempDir)` helper supporting both AVI/MP4+WAV and image-sequence+WAV layouts, covered by unit tests.
- **[Risk] CS2 hangs on demo load or fails to emit `STRIKEYE_EVT:END`** → **Mitigation**: Enforce per-phase timeouts (demo load timeout + segment duration * factor + grace period) in `log_bridge` / `scheduler`, triggering controlled Job Object termination and retry/failure recording.
- **[Risk] Windows APIs (`CreateJobObjectW`, `AssignProcessToJobObject`, `CreateToolhelp32Snapshot`) cannot execute on Linux** → **Mitigation**: Keep Windows files minimal, verify with `GOOS=windows GOARCH=amd64 go build ./...` and `go vet`, and test all state transitions via Fake Runner on Linux.
