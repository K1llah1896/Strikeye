# Tasks

## 1. Go Monorepo & Dependency Baseline

- [x] 1.1 Initialize root `go.mod` (`module strikeye`, Go 1.27), add `github.com/markus-wa/demoinfocs-golang/v5 v5.3.0`, `modernc.org/sqlite`, `github.com/Microsoft/go-winio`, and `golang.org/x/sys`, and verify `go mod tidy` succeeds cleanly
- [x] 1.2 Create entrypoint skeletons `cmd/strikeye/main.go`, `cmd/strikeye-agent/main.go`, and `cmd/strikeye-cli/main.go` (supporting a basic `--version` / `env-check` flag), and verify `go build ./cmd/...` and `GOOS=windows GOARCH=amd64 go build ./cmd/...` both compile without CGO

## 2. Config & Toolchain Probing (`internal/config`)

- [x] 2.1 Implement `DataRoot` layout bootstrap and `Config` struct with atomic `config.json` load/save and corrupt-config timestamped quarantine, and verify unit tests cover clean init, atomic save, and malformed JSON recovery
- [x] 2.2 Implement Steam/CS2/HLAE/FFmpeg/FFprobe path probing (`probe_windows.go` for Windows registry/VDF/`%LOCALAPPDATA%` + `probe_other.go` for portable env/PATH probing) with zero OBS probing, and verify unit tests cover available, missing, and overridden tool paths

## 3. SQLite Storage & Legacy Migration (`internal/storage`)

- [x] 3.1 Implement `internal/storage/db.go` and `migrations.go` with SQLite WAL/FK pragmas, `schema_migrations` versioning, all 10 core tables/indexes from the spec, and pre-migration `VACUUM INTO` snapshot backups in `backup/db_snapshots/`, and verify unit tests confirm WAL mode, FK cascades, and snapshot creation on schema upgrade
- [x] 3.2 Implement `demo_repo.go`, `analysis_repo.go`, `render_task_repo.go`, `media_repo.go`, and `montage_repo.go` with `content_sha256` and `task_uid` idempotency plus `media_videos` upsert/replacement, and verify unit tests cover CRUD, duplicate prevention, and cascade deletes
- [x] 3.3 Implement `legacy_migrator.go` to back up and read legacy `cs2-insight.db` (plus optional `demos.db`/`montage.db`) without mutating source files, import `demo_files` remarks/titles and verified `recorded_clips` MP4s, and verify unit tests cover valid import, duplicate skipping, and missing MP4 handling using synthetic SQLite fixtures

## 4. Cross-Process Mutex & Named Pipe IPC (`internal/ipc`)

- [x] 4.1 Implement cross-process `Lock` abstractions (`Local\StrikeyeAgentSingletonMutex`, `Local\StrikeyeUISingletonMutex`, `Local\StrikeyeCS2RenderActiveMutex`) in `mutex_windows.go` (Win32 mutexes) and `mutex_unix.go` (`flock` lockfiles), and verify concurrent unit tests confirm non-blocking contention, release, and re-acquisition
- [x] 4.2 Implement JSON-RPC 2.0 server/client and asynchronous event subscription stream over `\\.\pipe\strikeye-ipc` (`transport_windows.go` via `go-winio`) and portable listener abstraction (`transport_other.go`), and verify unit tests cover concurrent RPC round-trips, malformed frame errors, event ordering, and graceful shutdown

## 5. Cross-Package Integration & Verification

- [x] 5.1 Wire `strikeye-cli` `doctor` subcommand to exercise `config` probing, `storage` initialization, and `ipc` self-check, and verify `go test -race ./...` passes on Linux and `GOOS=windows GOARCH=amd64 go build ./...` compiles all binaries cleanly
