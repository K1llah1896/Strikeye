# Spec Delta

## Purpose

按需获取并本地缓存 CS2 官方地图的三角面/BVH 几何数据，为视线可见性、TTD/TTK、墙体与烟雾交互等分析提供基础，同时避免把第三方二进制数据打进仓库。

## ADDED Requirements

### Requirement: Geometry dataset manifest
The system SHALL read the remote geometry manifest and expose its map list.

#### Scenario: Valid manifest
- **WHEN** the manifest reports `formatVersion` 1 with at least one map
- **THEN** the map names are available for fetching

#### Scenario: Unsupported manifest
- **WHEN** the manifest format version is not 1 or lists no maps
- **THEN** the fetch fails with an unsupported-manifest error

### Requirement: Checksummed download and cache
The system SHALL download missing geometry into `<DataRoot>/cache/map_geometry` and verify size and SHA-256 before use.

#### Scenario: Fresh download
- **WHEN** a requested map is absent from the cache
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

### Requirement: Geometry file parsing
The system SHALL parse `.tri` v1 files into vertices, indices, and a BVH.

#### Scenario: Valid file
- **WHEN** a v1 file is read
- **THEN** vertex, index, and node counts match the header and the file SHA-256 is reported

#### Scenario: Corrupt file
- **WHEN** the magic is wrong or a section is truncated
- **THEN** parsing fails with an invalid-geometry error instead of panicking

### Requirement: Line-of-sight query
The system SHALL answer whether a segment between two world points is blocked.

#### Scenario: Blocked segment
- **WHEN** the segment passes through a wall triangle
- **THEN** the query reports occluded

#### Scenario: Clear segment
- **WHEN** no triangle lies between the points
- **THEN** the query reports clear

#### Scenario: Endpoint on surface
- **WHEN** the only intersection is at the segment endpoint within 1e-4
- **THEN** the query reports clear

### Requirement: Geometry CLI
The system SHALL expose cache listing and fetching through the CLI.

#### Scenario: List cache
- **WHEN** `strikeye-cli maps` runs
- **THEN** it prints the cache directory and cached map names

#### Scenario: Fetch selection
- **WHEN** `strikeye-cli maps-fetch --map de_dust2 --map de_ancient` or `--all` runs
- **THEN** each requested map resolves to a cached path

#### Scenario: Missing selection
- **WHEN** neither `--map` nor `--all` is provided
- **THEN** the command fails with `--map or --all is required`
