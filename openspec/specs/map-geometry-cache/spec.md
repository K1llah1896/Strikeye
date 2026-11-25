# map-geometry-cache Specification

## Purpose
按需获取并本地缓存 CS2 官方地图的三角面/BVH 几何数据，为视线可见性、TTD/TTK、墙体与烟雾交互等分析提供基础，同时避免把第三方二进制数据打进仓库。

## Requirements

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

### Requirement: Sidecar geometry source
系统 SHALL 优先从 `map_geometry.zip` 边车取图，边车位置为可执行文件目录、当前目录、DataRoot 或其 `cache`/`assets` 子目录；解压后 SHALL 按边车 `manifest.json` 的 `size`+`sha256` 校验。

#### Scenario: Sidecar fills a cold cache
- **WHEN** 缓存为空且 exe 旁存在 `map_geometry.zip`
- **THEN** 只解压请求的那张图，校验通过后写入缓存，后续调用直接复用

#### Scenario: Cached file wins over sidecar
- **WHEN** 目标 `.tri` 已存在且非空
- **THEN** 直接加载缓存文件，不读取边车

#### Scenario: Explicit sidecar overrides detection
- **WHEN** 调用方显式传入边车路径（例如 `corpus --geometry-bundle`）
- **THEN** 只从该边车取图，不再搜索默认位置

#### Scenario: Corrupt sidecar entry leaves nothing behind
- **WHEN** 边车中某张图的校验和与 manifest 不符
- **THEN** 该次调用报告几何不可用，缓存目录不留下 `.part` 或半成品文件

#### Scenario: Missing bundle degrades gracefully
- **WHEN** 既无缓存也无边车
- **THEN** 调用方得到“几何不可用”并让依赖几何的指标归零，而不是报错

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
