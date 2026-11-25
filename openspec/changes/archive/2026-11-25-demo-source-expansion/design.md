# Design

## Context

`internal/watcher/discovery.go` classifies sources with a three-branch `switch`; `watcher.scanOnce` only walks for `*.dem`; `Library` hardcodes a filter list whose values (`faceit`, `perfectworld`) do not match stored values (`perfect_world`, `unknown`). Ingestion is otherwise strong: 1.5s sampling, stability window, `PBDEMS2` magic, SHA-256 upsert.

## Decisions

### 1. Ordered rule table, canonical IDs

Classification runs on the slash-normalized lowercase full path (directory + filename), so a file at `~/Downloads/faceit_mirage.dem` and a directory `…/FACEIT/demos` both resolve. Rules are ordered most-specific-first; `valve` is checked last because `/csgo/replays` can appear inside vendor folder names. IDs are snake_case and stored verbatim in `demos.source_platform`.

Why a table instead of a `switch`: 16 sources make ordering and coverage explicit, and the same table feeds the UI filter, so a stored value can never lack a filter option again.

`NormalizePlatform` maps the historical shorthands used by older rows/config (`perfectworld`, `pw`, `5eplay`, `5E`) onto canonical IDs. `ClassifyPlatform` returns `unknown` for no match; `unknown` is a legal stored value and is surfaced as `custom`-style "未知来源" in the filter.

### 2. Archives are extracted, not parsed in place

`ExtractArchives` writes `.dem` entries to `Options.ImportDir/<archive-basename>-<8-hex-of-path-hash>/<entry-base>`. The directory is derived from the archive path, so re-scanning the same archive is idempotent and never duplicates directories. Rules:

- only `*.dem` entries (case-insensitive) are written; nested paths are flattened to their base name;
- zip-slip is impossible because the destination name is `filepath.Base` of the entry and the destination root is fixed;
- per-entry limit 4 GiB, per-archive total 8 GiB, entry count 32 (guards zip bombs);
- extraction writes to `<dest>.part` then renames, so a crash cannot register a truncated demo;
- unsupported/corrupt archives surface a warning through the existing `OnWarning` hook, never abort the scan.

The extracted file goes through the normal `ValidateDemoFile` + `StreamSHA256` + `UpsertByContentHash` path, so dedupe semantics are unchanged and the archive itself is never stored as a demo.

### 3. `.info` is advisory, never authoritative over users

`.info` sits next to the demo (`match.dem.info` or `match.info`). Parsing is tolerant: unknown fields are ignored, numbers may be strings, and a malformed file is a warning only. Applied fields: `source` → platform (only if the path rule yielded `unknown`), `date`, `team1`/`team2`, `score1`/`score2` (only when the demo row is still at defaults and `parse_status` is not `ready`), plus `matchId` into `remark` only when remark is empty.

Rationale: a parsed demo's teams/score come from the demo itself and are more trustworthy; user edits must never be clobbered.

### 4. New dependency

`github.com/klauspost/compress` supplies `bzip2` and `zstd` readers (stdlib covers `zip`/`gzip`). It is pure Go, widely used, and keeps the single-binary, cgo-free build. Rejected: shelling out to 7-Zip (external tool, platform-specific) and skipping bz2/zst (leaves the CS:DM parity gap open).

## Risks

- Scanning a directory with huge archives adds walk-time cost: mitigated by extension filter before any extraction and by `fileTrack` memoization keyed on archive path.
- Duplicate extraction across archive renames: the extracted demo still dedupes by content hash, so only disk space is affected; the import directory is documented as cache-like and safe to delete.
- Legacy rows with `perfectworld` keep working because filtering normalizes both sides at query time (see UI spec delta).
