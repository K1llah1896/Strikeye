# Tasks

## 1. CS2 User Config Snapshot Backup & Process Ownership Guard

- [x] 1.1 Implement `internal/recorder/cs2_cfg_backup.go` to scan Steam `userdata/<account_id>/730/local/cfg/` and `game/csgo/cfg/` for `config.cfg`, `cs2_user_keys_*.vcfg`, `cs2_user_convars_*.vcfg`, and `cs2_video.txt`, write `active_restore_manifest.json`, and perform atomic `RestoreIfPending()` rollback; verify with unit tests covering backup, partial failure, and idempotent restore.
- [x] 1.2 Implement `internal/recorder/process_guard.go` (plus `process_guard_windows.go` and `process_guard_other.go`) with `ProcessGuard` interface, Windows Job Object (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`), `(PID, CreationTime, SessionToken)` tracking, and external non-Strikeye `cs2.exe` detection; verify with unit tests for ownership matching and `GOOS=windows GOARCH=amd64 go build ./internal/recorder/...`.

## 2. HLAE Launcher, Tick Script Generator & Log Bridge

- [x] 2.1 Implement `internal/recorder/hlae_launcher.go` to create temporary `_strikeye_playback_*.dem` copies, generate `_strikeye_boot_<id>.cfg` with `mirv_streams` + `mirv_cmd addAtTick` (128-tick pre-seek warmup, `demo_end_guard` clamping, `STRIKEYE_DEMO_LOADED` / `STRIKEYE_EVT:START` / `STRIKEYE_EVT:END`), and build zero-VPK `-customLoader` + `-insecure` HLAE launch arguments; verify with unit tests asserting argument safety and tick clamping.
- [x] 2.2 Implement `internal/recorder/log_bridge.go` to tail `-con_logfile` incrementally and emit structured demo-loaded and segment start/end events with context cancellation and timeout handling; verify with unit tests simulating appended log lines and timeouts.

## 3. Media Muxing, ffprobe Quality Gate & Render Scheduler

- [x] 3.1 Implement `internal/recorder/media_verifier.go` to resolve `mirv_streams` raw outputs, mux to MP4 via `internal/composer` (hardware encoder probe, optional key-input HUD overlay, optional FrameMeld), run `ffprobe` validation (streams, FPS, non-zero duration), and register `media_videos`; verify with unit/integration tests covering valid and corrupt FFprobe outputs.
- [x] 3.2 Implement `internal/recorder/scheduler.go` with policy-based task planning (`smart_highlights`, `full_pov`, `manual_only`), per-demo `start_tick`-ordered batch execution, UI 15s cooldown & `Local\StrikeyeCS2RenderActiveMutex` coordination, external CS2 deferral, `AbortAndCleanup()`, and `RecoverInterrupted()` startup reconciliation; verify with Fake Runner end-to-end tests on Linux covering batching, abort/rollback, and crash recovery.

## 4. Agent & CLI Integration and Full Verification

- [x] 4.1 Integrate `RecoverInterrupted()`, post-parse policy planning, background render scheduler loop, and `render.start_now` / `render.abort_and_cleanup` / `render.status` / `render.enqueue` RPCs + events into `cmd/strikeye-agent/main.go`; verify with `cmd/strikeye-agent/main_test.go` tests.
- [x] 4.2 Add `render` dry-run / self-check subcommand to `cmd/strikeye-cli/main.go` and verify with `cmd/strikeye-cli/main_test.go`.
- [x] 4.3 Run `go test ./...`, `go vet ./...`, and `GOOS=windows GOARCH=amd64 go build ./...` and verify all tests and cross-platform builds pass cleanly.
