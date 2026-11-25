# Spec Delta

## Purpose

Extends demo discovery from three hardcoded sources to a canonical 16-source registry, adds automatic extraction of archived demos, and applies advisory `.info` sidecar metadata at ingestion.

## MODIFIED Requirements

### Requirement: Multi-directory demo discovery
The watcher SHALL monitor all configured `DemoWatchDirs` alongside auto-probed CS2, Perfect World, and 5E demo directories when present, ignoring files that are neither `.dem` nor a supported demo archive (`.zip`, `.gz`, `.bz2`, `.zst`), and recording the matched `watch_root` and canonical `source_platform` for every registered demo.

#### Scenario: Discovering demos across configured watch directories
- **WHEN** a new `.dem` file is placed in a configured watch directory
- **THEN** the watcher detects the file and associates it with its `watch_root` and inferred `source_platform`

#### Scenario: Archive candidate is discovered without being registered as a demo
- **WHEN** a `match.zip` is placed in a configured watch directory
- **THEN** the watcher treats it as an archive candidate, extracts its `.dem` entries, and only the extracted demo is registered

## ADDED Requirements

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
