# demo-watcher Specification

## Purpose

Discovers local CS2 demo files across configured and platform-probed directories, stabilizes in-progress writes, and deduplicates completed demos into SQLite by content hash.

## Requirements

### Requirement: Multi-directory demo discovery
The watcher SHALL monitor all configured `DemoWatchDirs` alongside auto-probed CS2, Perfect World, and 5E demo directories when present, ignoring files that are neither `.dem` nor a supported demo archive (`.zip`, `.gz`, `.bz2`, `.zst`), and recording the matched `watch_root` and canonical `source_platform` for every registered demo.

#### Scenario: Discovering demos across configured watch directories
- **WHEN** a new `.dem` file is placed in a configured watch directory
- **THEN** the watcher detects the file and associates it with its `watch_root` and inferred `source_platform`

#### Scenario: Archive candidate is discovered without being registered as a demo
- **WHEN** a `match.zip` is placed in a configured watch directory
- **THEN** the watcher treats it as an archive candidate, extracts its `.dem` entries, and only the extracted demo is registered

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

### Requirement: Canonical source platform registry
The watcher SHALL classify a demo path into one of the canonical source IDs `valve`, `perfect_world`, `5e`, `faceit`, `esl`, `esea`, `matchzy`, `pracc`, `renown`, `challengermode`, `esportal`, `fastcup`, `esplay`, `esportligaen`, `popflash`, `ebot`, or `unknown`, using a case-insensitive ordered rule table applied to directory and filename segments, and SHALL expose the same registry (ID plus display label) to the Match Library filter.

#### Scenario: Vendor filename is recognized
- **WHEN** a file named `faceit_mirage_2026.dem` appears in a configured watch directory
- **THEN** the demo is stored with `source_platform = faceit`

#### Scenario: Unrecognized source degrades to unknown
- **WHEN** a demo path matches no rule
- **THEN** the demo is stored with `source_platform = unknown` and remains parseable and filterable

#### Scenario: Legacy platform labels normalize
- **WHEN** a stored or configured value is `perfectworld`, `pw`, `5eplay`, or `5E`
- **THEN** it is normalized to `perfect_world` or `5e` respectively before comparison or persistence

### Requirement: Archive ingestion and safe extraction
The watcher SHALL detect demo archives by extension, extract every `.dem` entry into the configured import directory using flattened base names, write each entry to a temporary file and rename it only after a complete copy, and reject an archive that exceeds 32 entries, 4 GiB for a single entry, or 8 GiB in total.

#### Scenario: Zip archive yields a registered demo
- **WHEN** a stable `match.zip` containing `match.dem` is discovered
- **THEN** `match.dem` is extracted under the import directory, validated for `PBDEMS2` magic, and registered by content hash

#### Scenario: Archive traversal entry is neutralized
- **WHEN** a zip entry is named `../../evil.dem`
- **THEN** it is extracted as `evil.dem` inside the import directory and nothing is written outside it

#### Scenario: Oversized archive is refused
- **WHEN** an archive entry exceeds 4 GiB or the archive exceeds 8 GiB in total
- **THEN** extraction stops with a warning and no partial demo is registered

#### Scenario: Corrupt archive does not abort the scan
- **WHEN** an archive cannot be opened or decoded
- **THEN** the watcher records a warning and continues scanning remaining candidates

### Requirement: Sidecar match metadata application
When a `.info` sidecar exists next to a demo, the watcher SHALL parse it tolerantly and apply only non-destructive fields: `source` may set the platform when path classification yielded `unknown`, and `date`, team names, scores, and match ID may fill fields that are empty or still at defaults; it MUST NOT overwrite an existing platform, parsed teams, parsed score, or user-edited remark.

#### Scenario: Info file supplies missing metadata
- **WHEN** a demo with no parsed metadata has a sibling `.info` declaring teams, score, and date
- **THEN** the demo row is created with those values and the info-derived platform when path classification was unknown

#### Scenario: Malformed info file is ignored safely
- **WHEN** the `.info` file is not valid JSON
- **THEN** the demo is still registered and a warning is reported
