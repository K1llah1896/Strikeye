# Design

## Geometry: what actually compresses

The published `.tri` files are gzip streams. Wrapping them again in zstd saves
~0.4% — compressed data does not compress. Measured on `de_inferno` (40.0 MB
gzip, 99.6 MB raw): zstd-over-raw 38.1 MB, xz-over-raw 32.6 MB. So the packer
unwraps the gzip container and re-encodes the payload.

`xz` (LZMA2) wins because the payload is BVH nodes and float vertices: highly
repetitive but not byte-aligned, which LZMA models better than zstd's entropy
coder at these budgets. The cost is decode time: ~21 MB/s in the pure-Go
`ulikunitz/xz` decoder, i.e. ~5 s for the largest map, paid once because the
extracted `.tri` stays in the DataRoot cache.

Container detection lives in `mapgeom.decompressGeometry`: gzip (published
files), zstd, xz, or plain payload. The manifest records `encoding`, and `Size`
/`SHA256` describe the extracted bytes, so verification is unchanged.

## Geometry: which maps

The dataset covers 16 maps; the release carries the 11-map competitive pool
(`mapgeom.CoreMaps`, 119.9 MB as xz). The remaining five (cs_italy, cs_office,
de_eldorado, de_debris, de_poseidon) are casual/arms-race maps that nobody
analyses competitively; they stay fetchable through `maps-fetch`, and the
settings card grows a "补充其他地图几何" button when the bundle is present. Both
paths degrade exactly as before when geometry is absent (TTD 0,
`geometry_used=false`).

Alternative rejected: rebuilding the BVH at load time to drop it from the
payload (~40% of the bytes). It would change the occlusion numerics that TTD and
line-of-sight depend on, for a ~25 MB gain — not worth the risk here.

## FFmpeg: trim, then solid-compress

Upstream ships ffmpeg.exe, ffprobe.exe, ffplay.exe, HTML docs and licences. Only
the first two are ever executed. Packing them as their own solid tar gives the
cross-binary redundancy that a zip of separate entries cannot: 51.3 MB for
221 MB of input. A zip of the same two files is 80.6 MB; the untouched upstream
zip is 119 MB.

The container is `tar.zst`, decompressed once by `updater.ExtractToolArchive`
(which also still takes the legacy `.zip`). zstd is chosen over xz here on
purpose: a 220 MB LZMA2 decode in pure Go would cost ~15 s of first-launch
stall, while zstd decodes in under a second. `PackFFmpeg` verifies its own output
by extracting it and running the same binary detection a first launch uses.

The pinned upstream URL is gyan.dev's essentials build rather than BtbN's master
build: the master binaries are 172 MB each (sidecar 118 MB) for no feature
Strikeye uses, and essentials carries libx264, AAC and the filter set the
composer needs.

## Binaries

`-s -w` removes the symbol table and DWARF data: 83.6 MB → 68.0 MB for the GUI
binary (−18%), with no runtime effect. Packaging uses `zip -9` for the three
executables (136.9 MB → 74.7 MB) and `zip -0` for the already-compressed
sidecars, which saves minutes of packaging time for zero bytes.

## Fonts

`NotoSansSC-{Medium,Bold}.ttf` are 10.5 MB each and are embedded in all three
binaries. Restricting the outlines to GBK (22 090 runes) plus every non-ASCII
rune in `internal/` and `cmd/` yields 7.7 MB per weight — a 27% cut that keeps
all UI text and ordinary Chinese nicknames intact.

`scripts/subset-fonts.sh` regenerates the files from the upstream releases, and
`assets/fonts/coverage_test.go` walks the Go sources and fails when a CJK or
punctuation rune used by the UI is missing from the subset, so the tradeoff
cannot silently erode.

## Result

| Component | 0.3.1 | 0.4.0 |
| --- | --- | --- |
| `strikeye.exe` | 68.0 MB | 62.6 MB |
| `strikeye-agent.exe` | 48.7 MB | 43.2 MB |
| `strikeye-cli.exe` | 30.4 MB | 31.1 MB |
| `map_geometry.zip` | 199.7 MB | 119.9 MB |
| `ffmpeg-windows-amd64.*` | 119.5 MB | 51.3 MB |
| `hlae-windows-amd64.zip` | 9.1 MB | 9.1 MB |
| **release zip** | **527 MB** | **255 MB** |
