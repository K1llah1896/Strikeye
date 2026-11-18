# Design

## Context

See `proposal.md` for motivation and capability scope. Phase 1 already delivered:
- `internal/config`: `DemoWatchDirs`, `Paths`, `Render.PreBufferSec` (default `3.0`) / `Render.PostBufferSec` (default `2.0`), and `DataRoot` layout (`cache/replay_32hz/`).
- `internal/storage`: SQLite WAL database (`demos`, `match_players`, `match_rounds`, `match_events`, `highlight_clips`) and `DemoRepository` / `AnalysisRepository` with transactional `ReplaceMatch*` methods.
- `internal/ipc`: Named-pipe / local-stream JSON-RPC 2.0 server with event broadcast.
- `go.mod`: Pins `github.com/markus-wa/demoinfocs-golang/v5 v5.3.0`.

Legacy reference logic lives in `CS2-insight-agent-main/CS2-insight-agent-main/backend/app/features/demo_analysis/` (`match_workspace.py`, `tag_detection.py`, `tag_constants.py`). Real CS2 demo fixtures are present at `tests/furia-vs-falcons-m1-ancient.dem` and `tests/furia-vs-falcons-m2-dust2.dem`.

## Goals / Non-Goals

**Goals:**
- Single-pass parsing via `demoinfocs.Parser.ParseToEnd()` with bounded memory and zero re-parses after persistence.
- Strict parity with legacy competitive metrics (`Strikeye Rating`, capped damage, 5s trade window, KAST%, 1vN clutches, 5-tier economy) and legacy competitive highlight thresholds (`tag_constants.py`).
- Atomic `.rpl` (`CS2RPL01`) binary cache with round-indexed random access for both 32Hz 2D replay/grenades and raw-tick button edge streams.
- Clean wiring into `strikeye-cli` (standalone verification/benchmarking) and `strikeye-agent` (background watcher -> parser -> SQLite + IPC events).

**Non-Goals:**
- HLAE/CS2 rendering (`internal/recorder`), FFmpeg compositing (`internal/composer`), or MyGo UI views (`internal/ui`).
- Entertainment/fail/meme tags (`下饭`, `211`, `o`, `i`, `z`) or AI roast generation.
- Synthetic mouse trajectory generation when `UserCmd` is absent.

## Decisions

### 1. Single-Pass Collector Architecture (`internal/parser`)
Rather than running multiple passes or loading raw event tables into memory like the Python `demoparser2` pipeline, `single_pass.go` registers event/net-message/frame handlers on one `demoinfocs-golang/v5` parser instance and delegates state accumulation to focused collectors:
- `round_windows.go`: Tracks `MatchStartedChanged`, `RoundStart`, `RoundFreezetimeEnd`, `RoundEnd`, and parser EOF. Discards warmup/knife rounds, detects halftime team-side swaps (`halftime_round`), and closes the final round cleanly on EOF.
- `economy.go`: Captures per-player `EquipmentValueCurrent`, `AccountStart`, and `CashSpentThisRound` at `RoundFreezetimeEnd` and computes team economy via legacy priority voting (`pistol`, `full`, `force`, `semi`, `eco`).
- `metrics.go`: Maintains per-round victim HP (capped at 100), tracks 5-second trade pairs, opening duels, 1vN clutch eligibility/outcomes, and computes `Strikeye Rating` (`round(0.0073*KAST + 0.3591*KPR - 0.5329*DPR + 0.2372*Impact + 0.0032*ADR + 0.1587, 2)`).
- `highlights.go`: Evaluates competitive multi-kill grouping, clutches, defuse, and weapon-skill rules using `tag_constants.py` thresholds (e.g., `1500.0` units long-range) and `_TAG_COVERAGE_RULES` deduplication, while using `config.Render.PreBufferSec` / `PostBufferSec` defaults for clip buffer metadata.
- `grenade_effects.go`, `usercmd_input.go`, `replay_binary.go`: Collect 32Hz player/grenade/smoke/inferno state and raw-tick input transitions into a single `CS2RPL01` writer.

*Alternative considered*: Splitting metrics, highlights, and 32Hz replay into separate passes. Rejected because `ParseToEnd()` on a 500 MB demo is CPU-bound; single-pass cuts parse latency roughly 2–3x and guarantees consistent tick alignment.

### 2. Embedding Input Telemetry Inside `CS2RPL01` (Chunked Binary Layout)
Per design alignment, `UserCmd` / `m_nButtons` telemetry is embedded inside `<DataRoot>/cache/replay_32hz/<sha256>.rpl` rather than stored in a separate companion file or new SQLite table.

```text
+-------------------------------------------------------------------+
| Header (32B): Magic "CS2RPL01" | Version | TickRate | InputCap    |
+-------------------------------------------------------------------+
| Round Directory: [RoundNum, StartTick, EndTick, ChunkOffsets...]  |
+-------------------------------------------------------------------+
| Per-Round Payload:                                                |
|   - Chunk A: 32Hz Player Pose/State Frames (delta/compact binary) |
|   - Chunk B: Grenade Trajectories, Smoke Windows, Inferno Hulls   |
|   - Chunk C: Raw-Tick Input Edge Transitions (tick, slot, mask)   |
+-------------------------------------------------------------------+
```

- **Why chunked per-round**: 32Hz sampling is ideal for 2D radar playback, whereas button edges (counter-strafe, fire, scope) occur at full demo tick rate (64Hz). Separating Chunk C (sparse edge transitions) from Chunk A (32Hz frames) preserves exact tick precision without doubling 32Hz frame size, and allows `replay_binary.go` readers to seek directly to a single round.
- **Atomic write**: Write to `<sha256>.rpl.tmp` and `os.Rename` on parse completion so partial files never pollute `cache/replay_32hz/`.

### 3. Input Capability Probing (`CSVCMsg_UserCmd` + `m_nButtons` Fallback)
`usercmd_input.go` registers a net-message handler for `CSVCMsg_UserCmd` and also samples player pawn `m_nButtons` per tick.
- If `CSVCMsg_UserCmd` payloads are observed, `input_data_capability = 'full_usercmd'`.
- If `CSVCMsg_UserCmd` is stripped (common in Valve MM / some platform demos) but `m_nButtons` transitions are observed, `input_data_capability = 'entity_buttons_only'`.
- Otherwise `input_data_capability = 'none'`.

### 4. Watcher Stabilization & Deduplication (`internal/watcher`)
- Combines directory polling/fsnotify with a stabilization tracker: samples `(size, mtime)` every `1.5s`, requires 2 consecutive identical samples (`>= 3.0s`), size `> 1 MB`, non-exclusive read open, and `PBDEMS2` 8-byte header magic.
- Streams SHA-256 and calls `DemoRepository.UpsertByContentHash`. Only queues parsing when `parse_status` is `pending` (or explicitly requested re-parse).

### 5. CLI & Agent Integration
- `cmd/strikeye-cli`: Adds `parse --file <dem> [--data-root <dir>] [--force]` and `inspect --demo-id <id> | --rpl <path>` for offline verification and benchmarking against `tests/*.dem`.
- `cmd/strikeye-agent`: Wires watcher -> bounded parse worker -> SQLite persistence -> IPC event emission (`demo.discovered`, `parse.started`, `parse.completed`, `parse.failed`).

## Risks / Trade-offs

- **`demoinfocs-golang/v5` API differences vs `v4` & legacy Python `demoparser2`** → Validate `CSVCMsg_UserCmd`, `EquipmentValueCurrent`, `Inferno.Fires().Active().ConvexHull2D()`, and `spec_slot` directly against `tests/furia-vs-falcons-m1-ancient.dem` and `m2-dust2.dem` during implementation, adjusting field accessors if Valve/tournament demo schemas differ.
- **Large demo memory footprint during 32Hz + grenade accumulation** → Flush or compact per-round buffers into a temporary `.rpl.tmp` stream at each round end rather than holding an entire 40+ minute match of 32Hz frames in heap.
- **Cross-platform watcher tests on Linux vs Windows** → Isolate OS-specific default Steam/PerfectWorld/5E directory discovery into `probe_windows.go` / `probe_other.go` patterns while keeping stabilization and SHA-256 deduplication OS-agnostic.
