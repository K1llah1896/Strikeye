# Proposal: Map Geometry Cache

## Why

Line-of-sight analytics (TTD/TTK, wallbang and smoke interaction) need real map
geometry. The research report listed "地图几何" as missing and assumed it had to
be generated from the local VPK. A public dataset already exists:
`gitlab.com/akiver/cs2-map-geometry` (v1), the same geometry CS:DM's
`cs-demo-analyzer` uses, published as `.tri` BVH files with a checksummed
manifest. Fetching it removes the VPK dependency entirely.

## What Changes

- New `internal/mapgeom`: `.tri` reader (gzip, `CSDAGEO\0`, v1), BVH node
  validation, Möller–Trumbore occlusion tracer, manifest fetch, and checksummed
  download with `.part` → rename.
- `strikeye-cli maps` lists cached geometry; `maps-fetch --map <name>|--all`
  downloads into `<DataRoot>/cache/map_geometry`.
- Data is user-triggered and cached locally; nothing is vendored into the repo.

## Impact

- Affected specs: `map-geometry-cache` (new)
- Affected code: `internal/mapgeom` (new), `cmd/strikeye-cli`
- Network access only when `maps-fetch` runs; offline paths never fetch.
