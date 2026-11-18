# Proposal

## Why

Phases 1–3 established Strikeye's storage, single-pass demo analysis, radar rendering, and FFmpeg composition pipeline, but `strikeye-agent` still cannot execute automated background POV video capture. Phase 4 introduces the HLAE + CS2 native rendering engine so newly parsed demos can be automatically or manually recorded into verified MP4 media without modifying game VPKs or disturbing player configs and active CS2 sessions.

## What Changes

- Introduce `internal/recorder` to implement:
  - Render policy task planning (`smart_highlights`, `full_pov`, `manual_only`) and batch-per-demo scheduling with strict UI/CS2 mutual exclusion.
  - Player CS2 configuration snapshot backup and atomic rollback (`cs2_cfg_backup.go` with `active_restore_manifest.json`).
  - Process ownership guard and Windows Job Object (`KILL_ON_JOB_CLOSE`) lifecycle management, plus external `cs2.exe` deferral.
  - Zero-VPK HLAE `-customLoader` + `mirv_streams` + `mirv_cmd addAtTick` launcher with `demo_end_guard` boundary protection.
  - Real-time `-con_logfile` tailing bridge (`STRIKEYE_DEMO_LOADED`, `STRIKEYE_EVT:START/END`) and interface abstractions for cross-platform/Fake Runner testing.
  - Post-capture muxing via `internal/composer` (hardware encoder probe, optional key-input HUD overlay, optional FrameMeld) and `ffprobe` quality verification gate into `media_videos`.
- Integrate crash recovery, render scheduling, and `render.*` IPC RPCs/events into `cmd/strikeye-agent`.
- Add render pipeline inspection/dry-run/self-check subcommands in `cmd/strikeye-cli`.

## Capabilities

### New Capabilities
- `render-scheduler-and-safety`: Render policy task planning, per-demo batch scheduling, UI/CS2 mutual-exclusion state machine, external CS2 deferral, Job Object process ownership cleanup, CS2 user CFG atomic backup/restore, and agent crash recovery.
- `hlae-cs2-recorder`: Zero-VPK HLAE `-customLoader` + `mirv_streams` + `mirv_cmd addAtTick` capture execution, `demo_end_guard`, `-con_logfile` event bridge, FFmpeg/composer MP4 muxing, and `ffprobe` quality gate.

### Modified Capabilities
<!-- None: existing ipc-and-mutex, ffmpeg-composer, config-and-paths, and sqlite-storage-and-migration requirements remain unchanged. -->

## Impact

- New package: `internal/recorder/` (with `_windows.go` and `_other.go` platform implementations and Fake Runner tests).
- Modified binaries: `cmd/strikeye-agent` (crash recovery, render scheduler worker, `render.start_now` / `render.abort_and_cleanup` / `render.status` / `render.enqueue` RPCs) and `cmd/strikeye-cli` (render self-check / dry-run subcommands).
- Reuses `internal/config`, `internal/storage`, `internal/ipc`, `internal/parser` (`CS2RPL01`/input timelines), and `internal/composer`.
