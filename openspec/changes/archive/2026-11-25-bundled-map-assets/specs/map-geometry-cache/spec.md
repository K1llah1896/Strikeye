# Spec Delta

## Purpose

在既有“按需下载并缓存碰撞几何”的能力之上，增加随包边车作为首选来源，使几何在离线安装下也可用。

## MODIFIED Requirements

### Requirement: Checksummed download and cache
The system SHALL resolve geometry from the cheapest available source and verify size and SHA-256 before use:

- a cached `.tri` of the right size SHALL be reused as-is;
- otherwise, when a `map_geometry.zip` sidecar exists (executable directory, current directory, DataRoot or its `cache`/`assets` children), the requested map SHALL be extracted from it and verified against the `manifest.json` inside the archive;
- otherwise the pinned dataset download paths (`maps-fetch`) remain available and write into `<DataRoot>/cache/map_geometry`;
- when no source has the map, callers SHALL degrade to "geometry unavailable" instead of failing the parse.

#### Scenario: Fresh download
- **WHEN** a requested map is absent from the cache and no sidecar archive exists
- **THEN** the file is downloaded to a `.part` temporary and renamed only after size and checksum match

#### Scenario: Cached file reuse
- **WHEN** the cached file already matches the manifest checksum
- **THEN** no network request is made for that map

#### Scenario: Checksum mismatch
- **WHEN** the downloaded bytes do not match the manifest checksum
- **THEN** the temporary file is removed, no final file is written, and the command fails

#### Scenario: Unknown map
- **WHEN** a requested map is not in the manifest
- **THEN** the command fails naming that map

#### Scenario: Sidecar fills a cold cache
- **WHEN** the cache is empty and `map_geometry.zip` sits next to the executable
- **THEN** only the requested map is extracted, verified against the archive manifest, and reused on later calls

#### Scenario: Explicit sidecar overrides detection
- **WHEN** a caller passes an explicit sidecar path (for example `corpus --geometry-bundle`)
- **THEN** that archive is the only source consulted, and default locations are not searched
