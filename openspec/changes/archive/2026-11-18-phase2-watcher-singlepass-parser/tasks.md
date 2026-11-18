# Tasks

## 1. Demo Watcher & Deduplication (`internal/watcher`)

- [x] 1.1 Implement platform-aware demo directory discovery (`DemoWatchDirs` + Steam/PerfectWorld/5E auto-probing) and source platform classification, verified by unit tests in `internal/watcher/watcher_test.go`.
- [x] 1.2 Implement write-completion stabilization (1.5s size/mtime sampling across >= 3.0s, > 1 MB threshold, non-exclusive open, `PBDEMS2` header check) and streaming SHA-256 registration via `DemoRepository.UpsertByContentHash`, verified by unit tests covering in-progress writes, invalid headers, and duplicate hash deduplication.

## 2. Single-Pass Round, Economy & Combat Metrics (`internal/parser`)

- [x] 2.1 Implement `round_windows.go` and `economy.go` on `demoinfocs-golang/v5` (`v5.3.0`) to filter warmup/knife rounds, resolve halftime swaps and EOF round closure, and compute 5-tier team economy votes (`pistol`, `full`, `force`, `semi`, `eco`), verified by unit tests on synthetic/fixture round sequences.
- [x] 2.2 Implement `metrics.go` for capped damage (100 HP/round), ADR, utility ADR, 5-second trade kills/deaths, KAST%, opening duels, 1vN clutches, and `Strikeye Rating` matching `CS2-insight-agent-main` formulas, verified by unit tests comparing expected calculations against known inputs.
- [x] 2.3 Implement `single_pass.go` orchestration and transactional SQLite persistence (`demos`, `match_players`, `match_rounds`, `match_events`), including `parse_status = 'failed'` error recording on corrupt streams and single-parse reuse, verified by integration tests with an ephemeral SQLite DB.

## 3. Competitive Highlight Detection (`internal/parser/highlights.go`)

- [x] 3.1 Implement competitive highlight detection (multi-kills 2K–5K ACE, clutches, opening kills, knife/backstab, extreme/ninja defuses, and `tag_constants.py`-aligned skill rules such as `1500.0` long-range) plus `_TAG_COVERAGE_RULES` deduplication, strictly excluding fail/`下饭`/`211`/`o`/`i`/`z` tags, verified by unit tests covering tag priority deduplication and exclusion of meme/fail clips.

## 4. `CS2RPL01` 32Hz Replay & Embedded Input Stream (`internal/parser`)

- [x] 4.1 Implement `grenade_effects.go` and `usercmd_input.go` to sample 32Hz player poses/states, grenade trajectories, smoke windows, inferno 2D convex hulls, and raw-tick `CSVCMsg_UserCmd` / `m_nButtons` button transitions (`full_usercmd` / `entity_buttons_only` / `none`), verified by unit tests for capability detection and zero synthetic mouse paths.
- [x] 4.2 Implement `replay_binary.go` atomic `CS2RPL01` writer (`.rpl.tmp` -> `.rpl` under `<DataRoot>/cache/replay_32hz/<sha256>.rpl`) and round-indexed random-access reader, verified by round-trip encode/decode unit tests.

## 5. CLI, Agent Wiring & Real Demo Verification

- [x] 5.1 Add `parse` and `inspect` subcommands to `cmd/strikeye-cli` and wire `internal/watcher` -> `internal/parser` -> SQLite + IPC status events (`demo.discovered`, `parse.started`, `parse.completed`, `parse.failed`) in `cmd/strikeye-agent`, verified by CLI and agent smoke tests.
- [x] 5.2 Run `strikeye-cli parse` against `tests/furia-vs-falcons-m1-ancient.dem` and `tests/furia-vs-falcons-m2-dust2.dem`, verifying single-pass DB persistence, `.rpl` cache generation, round/score/player/highlight sanity, and full `go test ./...` + `openspec validate phase2-watcher-singlepass-parser --strict` pass.
