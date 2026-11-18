# Spec Delta

## Purpose

Discovers local CS2 demo files across configured and platform-probed directories, stabilizes in-progress writes, and deduplicates completed demos into SQLite by content hash.

## ADDED Requirements

### Requirement: Multi-directory demo discovery
The watcher SHALL monitor all configured `DemoWatchDirs` alongside auto-probed CS2, Perfect World, and 5E demo directories when present, ignoring non-`.dem` files and recording the matched `watch_root` and `source_platform`.

#### Scenario: Discovering demos across configured watch directories
- **WHEN** a new `.dem` file is placed in a configured watch directory
- **THEN** the watcher detects the file and associates it with its `watch_root` and inferred `source_platform`

### Requirement: Write-completion stabilization before registration
The watcher SHALL sample newly detected `.dem` files every 1.5 seconds and MUST NOT mark a file ready for parsing until its size and modification time remain unchanged across at least 2 consecutive samples (>= 3.0 seconds), its size exceeds 1 MB, it can be opened without exclusive write lock conflict, and its header starts with the `PBDEMS2` magic bytes.

#### Scenario: In-progress demo write is deferred
- **WHEN** a `.dem` file's size increases between consecutive 1.5-second samples
- **THEN** the watcher defers registration and continues sampling until stability criteria are satisfied

#### Scenario: Invalid or truncated demo header rejected
- **WHEN** a stabilized `.dem` file is smaller than 1 MB or lacks the `PBDEMS2` header magic
- **THEN** the watcher skips automatic parsing and logs a diagnostic warning without crashing

### Requirement: Content-hash deduplication on ingestion
The watcher SHALL compute a streaming SHA-256 digest for each stabilized `.dem` file and register it idempotently via `content_sha256`, updating path metadata if moved without duplicating demo rows or re-triggering completed parses.

#### Scenario: Duplicate demo file ignored for re-parsing
- **WHEN** a `.dem` file with an already-parsed `content_sha256` appears under a new path or filename
- **THEN** the existing demo record updates its path metadata while preserving `parse_status` and user annotations
