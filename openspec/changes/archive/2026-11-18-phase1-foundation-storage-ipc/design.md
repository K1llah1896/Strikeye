# Design

## Context

See `proposal.md` for Phase 1 scope and motivation. Strikeye's architecture is defined in `docs/STRIKEYE_DEVELOPMENT_DOCUMENTATION.md`, while the legacy Python/SQLite implementation lives in `./CS2-insight-agent-main`.

Key environmental constraints shaping Phase 1:
1. **Target vs Development OS**: Target runtime is Windows 10/11 desktop (`strikeye.exe` + `strikeye-agent.exe`), whereas development and automated CI tests also run on `linux/amd64` with Go `1.27.4`.
2. **No-CGO / Pure-Go Preference**: MyGo Native UI uses pure Go without CGO; keeping Phase 1 dependencies free of CGO enables seamless `GOOS=windows` cross-compilation and fast `go test ./...` execution.
3. **Parser Baseline**: The team confirmed `github.com/markus-wa/demoinfocs-golang/v5@v5.3.0` (instead of `v4` in the doc) to ensure CS2 demo + `UserCmd` support in Phase 2.
4. **Legacy DB Reality Check**: Inspection of `./CS2-insight-agent-main/CS2-insight-agent-main/backend/app/databases.py` shows legacy data resides in a single SQLite file `cs2-insight.db` (`%APPDATA%\CS2 Insight Agent\data\cs2-insight.db`) containing `demo_files`, `recorded_clips`, `montage_projects`, and `montage_exports` (rather than separate `demos.db` / `montage.db`).

## Goals / Non-Goals

**Goals:**
- Initialize a clean Go Monorepo (`module strikeye`) that passes `go test ./...` on Linux and `GOOS=windows GOARCH=amd64 go build ./...` without CGO.
- Implement `internal/config`, `internal/storage`, and `internal/ipc` with clear interfaces ready for Phase 2–5 consumers.
- Ensure zero data loss on legacy `cs2-insight.db` import and on schema upgrades via pre-migration snapshots.

**Non-Goals:**
- Implementing Demo watching/parsing (`internal/watcher`, `internal/parser` — Phase 2).
- Implementing Radar rendering or FFmpeg composition (`internal/radar`, `internal/composer` — Phase 3).
- Implementing HLAE/CS2 launch, CFG backup/restore, or process Job Objects (`internal/recorder` — Phase 4).
- Implementing MyGo Native UI screens (`internal/ui` — Phase 5).

## Decisions

### 1. Pure-Go SQLite (`modernc.org/sqlite`) over `mattn/go-sqlite3`
- **Choice**: Use `modernc.org/sqlite` with standard `database/sql`.
- **Rationale**: Requires zero CGO/MinGW toolchain to cross-compile Windows `.exe` binaries from Linux and matches MyGo's no-CGO design.
- **Alternative Considered**: `mattn/go-sqlite3` (faster raw throughput in some microbenchmarks, but forces CGO + `x86_64-w64-mingw32-gcc` setup on every dev/CI host).
- **Mitigation**: Enable WAL mode (`PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`) and batch writes inside explicit transactions so throughput is more than sufficient for Strikeye's single-desktop workload.

### 2. Pin `demoinfocs-golang/v5@v5.3.0` in Root `go.mod`
- **Choice**: Declare `github.com/markus-wa/demoinfocs-golang/v5 v5.3.0` in `go.mod` during Phase 1 setup (even though `internal/parser` lands in Phase 2) and record the module layout in `cmd/strikeye-cli`.
- **Rationale**: Locks the agreed CS2 parser version upfront as the single source of truth across all phases.

### 3. Platform-Abstracted Config Probing, Mutexes, and IPC Transport
- **Choice**:
  - `internal/config`: Define `EnvProbe` / OS-specific `probe_windows.go` (reading Steam registry keys `HKCU\Software\Valve\Steam` / `HKLM\SOFTWARE\WOW6432Node\Valve\Steam`, `%LOCALAPPDATA%`, `libraryfolders.vdf`, HLAE/FFmpeg candidates) and `probe_other.go` (PATH/env/custom root probing for Linux/macOS tests). Completely omit any OBS registry/path checks.
  - `internal/ipc`:
    - **Locks**: Define a `Lock` interface (`TryAcquire() (bool, error)`, `Release() error`, `IsHeld() (bool, error)`). Implement `mutex_windows.go` via `golang.org/x/sys/windows` `CreateMutex` / `OpenMutex` (`Local\StrikeyeAgentSingletonMutex`, `Local\StrikeyeUISingletonMutex`, `Local\StrikeyeCS2RenderActiveMutex`) and `mutex_unix.go` via `flock` lockfiles under `os.TempDir()` for deterministic cross-process unit tests on Linux.
    - **Transport**: Wrap `net.Listener` / `net.Conn` with newline-delimited JSON-RPC 2.0 + notification event frames (`method: "event.<topic>"` without `id`). On Windows, bind `\\.\pipe\strikeye-ipc` via pure-Go `github.com/Microsoft/go-winio`; in portable unit tests, accept injected Unix sockets or `net.Pipe` / custom listener paths.
- **Rationale**: Allows full concurrency, lock-contention, and RPC integration tests to execute natively under `go test ./...` on Linux while ensuring real Win32 semantics on Windows.

### 4. Schema Migration & Pre-Migration Snapshot Strategy
- **Choice**: Maintain an explicit `schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)` table inside `strikeye.db`. When opening an existing DB that has pending migrations and non-zero schema/data, execute `VACUUM INTO '<DataRoot>/backup/db_snapshots/strikeye_v<old>_<ts>.db'` (or consistent hot-backup copy) *before* running the migration transaction.
- **Rationale**: `VACUUM INTO` produces a consistent, standalone SQLite snapshot even in WAL mode without copying detached `-wal`/`-shm` sidecars.

### 5. Legacy Importer Decoupling (`legacy_migrator.go`)
- **Choice**: Support importing from `cs2-insight.db` (primary legacy layout) plus optional explicit paths (`demos.db`, `montage.db` if provided). Copy the legacy DB + WAL/SHM if present to `<DataRoot>/backup/legacy_import_<timestamp>/`, open the copied/source DB read-only, map `demo_files` -> `demos` (computing `content_sha256` if the `.dem` file exists on disk, or falling back to a deterministic `legacy:<path>` key when the original `.dem` was moved, while keeping uniqueness safe), and validate `recorded_clips` MP4 paths through an injected `MediaVerifier` callback (defaulting to file existence + non-empty check in Phase 1, upgradable to `ffprobe` in Phase 3/4).
- **Rationale**: Preserves user remarks, `display_name`, and verified clips without requiring Phase 3/4 `ffprobe` wrapper code to exist yet.

## Risks / Trade-offs

- **[Risk] Legacy `.dem` files moved or deleted since old DB creation** -> **Mitigation**: Do not compute SHA-256 over missing files; record missing `.dem` gracefully or mark synthetic `legacy_missing_sha256` if importing metadata-only rows, and skip missing MP4s in `recorded_clips` with structured migration warnings.
- **[Risk] Windows Named Pipe ACLs or stale pipe/mutex handles after crash** -> **Mitigation**: Win32 mutexes are automatically released by the kernel when the owning process exits (`WAIT_ABANDONED` handled as safely acquirable after cleanup); `go-winio` handles pipe instance cleanup on listener close.
- **[Risk] `modernc.org/sqlite` binary size (~10–15MB)** -> **Mitigation**: Acceptable for desktop Go binaries and eliminates CGO build fragility completely.

## Migration Plan

1. Initialize `go.mod`, directory layout, and entrypoint skeletons (`cmd/strikeye`, `cmd/strikeye-agent`, `cmd/strikeye-cli`).
2. Implement and unit-test `internal/config`, `internal/storage` (including `legacy_migrator.go`), and `internal/ipc`.
3. Verify both `go test -race ./...` on Linux and `GOOS=windows GOARCH=amd64 go build ./...`.
