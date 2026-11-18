# Proposal

## Why

Strikeye is migrating from the legacy Python/FastAPI + Tauri/React + OBS architecture in `./CS2-insight-agent-main` to a pure-Go, decoupled Windows desktop architecture (`strikeye.exe` + `strikeye-agent.exe` using MyGo Native UI, `demoinfocs-golang/v5`, HLAE, FFmpeg, and SQLite). Before Demo parsing (Phase 2), media/radar rendering (Phase 3), HLAE capture (Phase 4), or Native UI (Phase 5) can be built, the project needs its Go module foundation, configuration/path probing, SQLite persistence + legacy import layer, and cross-process IPC/mutex primitives in place.

## What Changes

- Initialize the root Go Monorepo module (`module strikeye`) and directory skeleton (`cmd/strikeye`, `cmd/strikeye-agent`, `cmd/strikeye-cli`, `internal/{config,storage,ipc}`).
- Lock core Phase 1–2 dependencies upfront: pure-Go SQLite (`modernc.org/sqlite`) to avoid CGO cross-compilation friction, and `github.com/markus-wa/demoinfocs-golang/v5@v5.3.0` as the agreed CS2 parser baseline (overriding the `v4` label in the development doc).
- Implement `internal/config`: `DataRoot` layout bootstrap, atomic `config.json` load/save with corrupt-config quarantine, and Steam/CS2/HLAE/FFmpeg path probing without any OBS discovery paths.
- Implement `internal/storage`: SQLite WAL connection management, versioned schema migrations for all 9 core Strikeye tables (`demos`, `match_players`, `match_rounds`, `match_events`, `highlight_clips`, `media_videos`, `render_tasks`, `radar_cards`, `montage_projects`, `montage_exports` — 10 tables total per DDL), automatic pre-migration DB snapshots, repository DAOs with idempotent deduplication (`content_sha256`, `task_uid`), and lossless legacy asset import from `cs2-insight.db` (and fallback legacy filenames).
- Implement `internal/ipc`: Windows Named Pipe (`\\.\pipe\strikeye-ipc`) JSON-RPC 2.0 request/response + event subscription protocol, and cross-process mutex abstractions (`StrikeyeAgentSingletonMutex`, `StrikeyeUISingletonMutex`, `StrikeyeCS2RenderActiveMutex`) with platform-abstracted test doubles so all Phase 1 unit tests execute cleanly on both Linux dev hosts and Windows targets.
- Keep `./CS2-insight-agent-main` intact as read-only reference; Phase 2–6 engines and UI views are out of scope for this change.

## Capabilities

### New Capabilities
- `config-and-paths`: Manages global Strikeye configuration (`config.json`), `DataRoot` directory initialization, and environment/tool path probing for Steam, CS2, HLAE, and FFmpeg/FFprobe.
- `sqlite-storage-and-migration`: Manages SQLite WAL storage, versioned migrations, repository DAOs, automatic pre-migration snapshots, and lossless import of legacy `CS2-insight-agent` database records and verified media paths.
- `ipc-and-mutex`: Provides Windows Named Pipe JSON-RPC 2.0 + event-stream IPC and cross-process singleton and CS2 render-exclusivity lock primitives with testable platform abstractions.

### Modified Capabilities
<!-- None: this is the initial Phase 1 capability foundation. -->

## Impact

- Creates root `go.mod`, `go.sum`, `cmd/{strikeye,strikeye-agent,strikeye-cli}/main.go`, and `internal/{config,storage,ipc}/...`.
- Introduces Windows-specific build-tagged implementations (`*_windows.go`) alongside portable interfaces/test stubs (`*_unix.go` / `*_other.go`) so `go test ./...` runs on Linux while `GOOS=windows go build ./...` compiles target binaries.
- Reads (never mutates) legacy `%APPDATA%\CS2 Insight Agent\data\cs2-insight.db` during optional migration, writing snapshots into `<DataRoot>/backup/` before importing.
