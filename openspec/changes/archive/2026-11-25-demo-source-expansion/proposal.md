# Proposal

## Why

Strikeye only classifies three demo sources (`valve`, `perfect_world`, `5e`) and cannot ingest archives, while the Match Library filter already offers `faceit`—a filter that can never match stored data. Comparable open-source tools (CS:DM, cs-demo-analyzer) recognize 14–16 sources and auto-extract `zip/gz/bz2/zst`. Real demo libraries are mostly FACEIT/ESL/MatchZy downloads delivered as archives, so a large share of a player's demos never enter the library at all.

## What Changes

- **Canonical source registry** (`internal/watcher/sources.go`): 16 platform IDs with display labels and an ordered, case-insensitive classification table covering path segments and filename prefixes; `NormalizePlatform` migrates legacy labels (`perfectworld` → `perfect_world`).
- **Archive ingestion** (`internal/watcher/archive.go`): detect `.zip`, `.dem.gz`/`.gz`, `.bz2`, `.zst`; extract every `.dem` entry into a managed import directory with zip-slip protection and size ceilings; extracted files then flow through the existing stabilization/`PBDEMS2`/SHA-256 pipeline.
- **Sidecar `.info` metadata** (`internal/watcher/info.go`): tolerant JSON parsing of the `matchId`/`source`/`date`/`team1`/`team2`/`score` shape emitted by community tools; overrides `source_platform`, and fills `match_date`, team names, and scores only when the demo row has no value yet. Never overwrites user annotations.
- **Watcher wiring** (`internal/watcher/watcher.go`): archives are candidates alongside `.dem`; extraction is idempotent and de-duplicated by content hash; new `Options.ImportDir` (defaults to `<DataRoot>/media/imports`).
- **Match Library filter** (`internal/ui/views/library/library.go`): platform options generated from the registry (fixes `faceit` mismatch and the `perfectworld` vs `perfect_world` bug), and `全部平台`/`unknown` handled explicitly.

## Capabilities

### New Capabilities
<!-- None -->

### Modified Capabilities
- `demo-watcher`: canonical multi-source platform classification, archive extraction, and `.info` sidecar metadata application.
- `match-library-and-analysis-ui`: platform filter options are derived from the canonical source registry instead of a hardcoded, partially-unmatchable list.

## Impact

- `internal/watcher/{sources.go,archive.go,info.go,watcher.go,discovery.go,watcher_test.go}`
- `internal/storage/demo_repo.go` (metadata-only update path)
- `internal/ui/views/library/library.go`, `internal/ui/views/library/library_test.go`
- `internal/agent/agent.go` (pass import directory)
- `go.mod` / `go.sum` (add `github.com/klauspost/compress` for bz2/zstd)
- Existing tests that assert `ClassifyPlatform` on legacy paths stay green (rule ordering preserves them).
