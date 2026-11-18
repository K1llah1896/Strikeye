# Tasks: Phase 5 — MyGo Native UI Desktop Client

## 1. Dependencies, Map Assets & Agent IPC Extensions

- [x] 1.1 Add `github.com/egoist/mygo` to `go.mod`/`go.sum` and verify `GOOS=windows go build ./cmd/strikeye` compiles cleanly.
- [x] 1.2 Create `assets/maps/` with embedded CS2 radar PNGs + coordinate metadata JSONs (migrated from legacy assets where available, plus procedural grid fallback metadata) and verify `go test ./assets/...` passes.
- [x] 1.3 Extend `cmd/strikeye-agent/main.go` with `montage.export` (running `composer.BuildMontage` with progress events), `render.cancel`, `render.retry`, and `config.reload` IPC handlers, and verify new unit tests in `cmd/strikeye-agent/main_test.go` pass.

## 2. Startup Lifecycle, Render Guard Dialog & Main Shell

- [x] 2.1 Implement `internal/ui/theme/` (dark esports palette `#0F141C` / `#171E29` / `#F59E0B` / `#38BDF8`, font loading, reusable Native UI cards, capsules, buttons, and table primitives) and verify theme helpers compile and pass unit tests.
- [x] 2.2 Implement `internal/ui/guard/` for the Render Mutex Guard Dialog (polling `render.status`, "Continue Background Rendering" exit, and "Safely Abort & Open Strikeye" sending `render.abort_and_cleanup` and awaiting mutex release) and verify state transitions with unit tests.
- [x] 2.3 Implement `cmd/strikeye/main.go` startup flow (probe `Local\StrikeyeCS2RenderActiveMutex`, acquire `Local\StrikeyeUISingletonMutex`, auto-spawn `strikeye-agent` if unreachable) and `internal/ui/app.go` (left navigation rail, top live status bar with IPC event subscription, "Start Render Now" confirmation modal + `os.Exit(0)` handoff), verifying both `go test ./cmd/strikeye ./internal/ui/...` and `GOOS=windows go build ./cmd/strikeye`.

## 3. Core Native Components

- [x] 3.1 Implement `internal/ui/components/video_player.go` (embedded FFmpeg RGBA frame decoder + audio sync, play/pause, seek scrub, `0.25x`–`2.0x` speed, volume, fullscreen toggle, Event Timeline with `event_time - 2.0s` jump, and dual-slider `In/Out` trim controls) and verify seek/speed/timeline math with unit tests.
- [x] 3.2 Implement `internal/ui/components/key_hud_widget.go` (tick-synced `WASD`, crouch, jump, walk, fire, scope indicator from `CS2RPL01` / `m_nButtons`, plus explicit "no input data" state when `InputCap` is absent) and verify tick-to-button mapping with unit tests.
- [x] 3.3 Implement `internal/ui/components/radar_2d_canvas.go` (32Hz `CS2RPL01` player dots, health rings, view cones, flash rings, C4 marker, firing rays, grenade trajectories, smoke circles, molotov polygons, and KDE heatmaps over `assets/maps/` or fallback grid) and verify world-to-radar coordinate projection and KDE grid tests pass.
- [x] 3.4 Implement `internal/ui/components/radar_hex_preview.go` (CS Data Radar static card preview and animation status widget) and verify preview state binding tests pass.

## 4. Five Workspace Views

- [x] 4.1 Implement `internal/ui/views/library/` (`Match Library`: SQLite WAL search/filter by keyword/map/platform/status/favorites, parse & video status capsules, favorite/note editor, manual `.dem` import via `demo.parse`, watch-directory config, and quick-action navigation) and verify filter/query unit tests pass.
- [x] 4.2 Implement `internal/ui/views/spectator/` (`Spectator`: match/round selector, 10-player + observer switcher strictly backed by verified `media_videos` MP4s or unrendered summary card with "Enqueue Render" / "Start Render Now", round-relative tick sync across POV switches, Event Timeline, and Key HUD) and verify POV media-state & tick-sync tests pass.
- [x] 4.3 Implement `internal/ui/views/analysis/` (`Match Analysis`: 5 tabs for Overview & sortable 10-player Scoreboard, Rounds & Economy chart, Players comparison & CS Data Radar workbench with custom avatar/logo re-render & 4.0s MP4 batch export, 32Hz 2D Replay, and KDE Heatmaps) and verify scoreboard sorting and economy/heatmap filter tests pass.
- [x] 4.4 Implement `internal/ui/views/highlights/` (`Highlights & Montage`: Highlight Library with multi-criteria filters, embedded preview, `In/Out` trim & re-render; Montage Workbench with clip ordering, per-clip radar intro toggle, intro/outro/BGM/xfade/name-card/subtitle config, and async `montage.export` progress) and verify montage spec assembly tests pass.
- [x] 4.5 Implement `internal/ui/views/settings/` (`Settings & Tasks`: tool path probes & manual override, directory/storage backup & cache cleanup, render policy/quality/PrimarySteamID64 config, live `render_tasks` queue with priority/cancel/retry, and agent log viewer) and verify config/task action tests pass.

## 5. Cross-Platform Build & Validation

- [x] 5.1 Run full `go test ./...` and `GOOS=windows go build ./cmd/strikeye ./cmd/strikeye-agent ./cmd/strikeye-cli`, confirming zero WebView/HTML/JS/React/Python/OBS runtime dependencies in `go.mod` and `internal/ui/`.
- [x] 5.2 Run `openspec validate phase5-mygo-native-ui --strict` and verify all specs and tasks are valid and complete.
