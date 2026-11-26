# Release size reduction

## Why

The 0.3.1 release package reached 503 MB. Two thirds of that is data that ships
inside the zip (collision geometry, FFmpeg, HLAE), and the rest is three Go
binaries that each embed the same CJK fonts. Users download the package once per
release on a home connection, so the size is a product cost, not just an
inconvenience.

Nothing that a fresh install needs can move back behind a download button: the
whole point of the bundled datasets is that no click and no network is required.
So the size has to come out of the encodings and the payload selection.

## What changes

- **Geometry bundle** carries the competitive pool only (11 maps) and stores the
  raw `CSDAGEO` payload as an xz frame instead of wrapping the published gzip
  files. The loader detects gzip/zstd/xz/plain containers, so caches from older
  builds keep working. `assets-pack --all-geometry` restores the previous
  coverage for anyone packaging the long tail.
- **FFmpeg sidecar** becomes `ffmpeg-windows-amd64.tar.zst`: only `ffmpeg.exe`
  and `ffprobe.exe` (ffplay and the HTML docs are dropped) in one solid zstd
  frame. 51 MB instead of 119 MB. The legacy `.zip` name is still honoured.
- **Binaries** are linked with `-s -w` and compressed with `zip -9`; the
  already-compressed sidecars are stored (`zip -0`) so packaging stays fast.
- **CJK fonts** are subset to GBK plus every non-ASCII rune the Go sources can
  print, with `scripts/subset-fonts.sh` as the reproducible regenerator and a
  coverage test that fails when a new UI string outruns the subset.

## Impact

- Affected specs: `bundled-map-data` (geometry bundle, tool archives, size
  budget), `ci-pipeline` (release packaging).
- Affected code: `internal/mapassets`, `internal/mapgeom`, `internal/toolbundle`,
  `internal/updater`, `cmd/strikeye-cli` (`assets-pack`), settings UI copy,
  `assets/fonts`, `.github/workflows/release.yml`.
- Measured: `strikeye-windows-amd64.zip` 527 MB → 255 MB with no capability
  removed; casual-map geometry stays one click away in settings.
