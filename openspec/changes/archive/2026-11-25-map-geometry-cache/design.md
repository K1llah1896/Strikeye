# Design: Map Geometry Cache

## Dataset

`https://gitlab.com/akiver/cs2-map-geometry/-/raw/v1` publishes
`manifest.json` (`formatVersion: 1`, per-map `file`, `size`, `sha256`) and one
`.tri` file per map (16 maps, e.g. `de_dust2.tri` 6.2 MB, `de_ancient.tri`
16.1 MB). The repository carries no LICENSE file, so Strikeye does not vendor the
binaries: the operator runs `maps-fetch`, the source URL is printed in the
output, and only the cache is written. The project's own generated data should
replace this source once available.

## File format

A gzip stream containing little-endian sections:

```
magic "CSDAGEO\0" | version u32 | vertexCount u32 | indexCount u32 | nodeCount u32
vertices: vertexCount * 3 * f32
indices:  indexCount  * u32   (triangles in BVH leaf order)
nodes:    nodeCount   * (min f32[3], max f32[3], leftOrFirst u32, count u32)
```

`count > 0` marks a leaf whose triangles start at `leftOrFirst`; otherwise
`leftOrFirst` is the left child and `leftOrFirst+1` the right child. Sections are
read with a hard size cap and node references are validated on load so a corrupt
file cannot panic the tracer.

## Occlusion

`Tracer.IsOccluded` slab-tests child AABBs, then Möller–Trumbore per triangle,
short-circuiting on the first hit. Intersections within `1e-4` of the segment end
are ignored so a target standing on a surface is still visible; smoke and fire
are separate, simpler sphere/capsule factors handled by callers.

## Verification

Synthetic quad geometry covers load, checksum, truncation rejection, and
occlusion behaviour; an `httptest` server covers manifest parsing, download,
cache reuse, checksum mismatch, and unknown maps. Real-data verification fetched
`de_dust2`/`de_ancient` (244,628 verts / 420,394 tris / 271,147 nodes for
dust2) and confirmed occluded and clear sight lines.
