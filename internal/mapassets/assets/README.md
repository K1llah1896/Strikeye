# Bundled map assets

This directory holds the offline map datasets that ship inside the binaries so a
fresh install never has to download anything before parsing or analysing a match.

| File | Source | Size | Notes |
|---|---|---|---|
| `navs.zip` | `pnxenopoulos/awpy-data` release `2000950` (`navs.zip`) | 7.7 MB | CS2 navigation meshes, one `<map>.nav` per map |
| `callouts.csv` | `sink3x/cs2-server-admin-callouts` @ `e222d67` (`schematic/data/nav_places.csv`) | 780 KB | world-space callout boxes (MIT) |

Collision geometry (`.tri`) is ~200 MB, too large to embed, so it ships as
`map_geometry.zip` next to the executables and is extracted per map on demand
(see `internal/mapassets`).

The blobs are build inputs, not sources: they are git-ignored and produced by

    go run ./cmd/strikeye-cli assets-pack

which verifies every download against the checksums pinned in `mapassets.go`.
