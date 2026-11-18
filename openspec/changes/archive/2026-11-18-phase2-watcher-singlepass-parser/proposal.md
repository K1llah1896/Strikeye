# Proposal

## Why

Phase 1 established Strikeye's configuration, SQLite WAL storage, and IPC/lock foundation, but the system cannot yet discover local CS2 `.dem` files or extract match analytics, highlights, 2D replay geometry, and player input telemetry. Phase 2 builds the automated demo ingestion and Single-Pass parsing pipeline so every newly finished match is parsed once and persisted for downstream rendering (Phase 3/4) and Native UI browsing (Phase 5) without ever re-parsing the raw demo.

## What Changes

- Add `internal/watcher` to discover `.dem` files across configured and auto-probed demo directories, debounce in-progress writes until file size/mtime stabilize and `PBDEMS2` headers are valid, compute streaming SHA-256 digests, and register demos idempotently in SQLite.
- Add `internal/parser` using `github.com/markus-wa/demoinfocs-golang/v5` (`v5.3.0`) to perform a Single-Pass `ParseToEnd()` scan that extracts:
  - Active round boundaries, warmup/knife filtering, halftime swaps, MR12/overtime resolution, EOF round finalization, and 5-tier team economy classification (`pistol`, `full`, `force`, `semi`, `eco`).
  - Match/player combat metrics (capped damage, ADR, utility ADR, 5-second trade kills/deaths, KAST%, opening duels, 1vN clutches, and unified `Strikeye Rating`) aligned with `CS2-insight-agent-main` definitions.
  - Competitive highlight clip detection and tag deduplication aligned with legacy competitive rules while excluding entertainment/meme/fail tags and AI commentary.
  - Compact `CS2RPL01` binary replay cache (`<DataRoot>/cache/replay_32hz/<sha256>.rpl`) containing 32Hz player poses, grenade trajectories/smoke/inferno convex hulls, and embedded raw-tick `CSVCMsg_UserCmd` / `m_nButtons` input edge chunks (`input_data_capability`: `full_usercmd`, `entity_buttons_only`, or `none`).
- Wire `strikeye-cli` (`parse` / `inspect` subcommands) for standalone single-pass execution, cache verification, and benchmarking against real `.dem` fixtures in `tests/`, and wire `strikeye-agent` to run the watcher-to-parser pipeline with IPC event notifications.

## Capabilities

### New Capabilities
- `demo-watcher`: Watches configured and platform-probed CS2 demo directories, stabilizes in-progress `.dem` writes, and deduplicates demos into SQLite by `content_sha256`.
- `single-pass-parser`: Executes a single-pass `demoinfocs-golang/v5` parse to extract rounds, team economy, combat events, player statistics, and `Strikeye Rating` into persistent storage.
- `competitive-highlights`: Detects competitive multi-kill, clutch, opening, defuse, and weapon-skill highlight clips with priority-based tag deduplication, excluding entertainment/fail/meme tags.
- `replay-and-input-stream`: Encodes and reads the `CS2RPL01` binary cache embedding 32Hz 2D replay frames, grenade/smoke/inferno geometry, and raw-tick `UserCmd`/`m_nButtons` player input streams.

### Modified Capabilities

## Impact

- **New packages**: `internal/watcher`, `internal/parser`.
- **Modified binaries**: `cmd/strikeye-cli` (new `parse` and `inspect` commands), `cmd/strikeye-agent` (background watcher + parser worker loop + IPC status events).
- **Storage & Cache**: Reads/writes `demos`, `match_players`, `match_rounds`, `match_events`, `highlight_clips` via existing Phase 1 repositories; writes `.rpl` files under `<DataRoot>/cache/replay_32hz/`.
- **Test assets**: Uses `tests/furia-vs-falcons-m1-ancient.dem` and `tests/furia-vs-falcons-m2-dust2.dem` for end-to-end single-pass parsing, cache reuse, and metric verification.
