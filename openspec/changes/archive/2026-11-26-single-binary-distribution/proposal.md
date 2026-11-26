# Single-binary distribution

## Why

The release package ships three executables — `strikeye.exe`,
`strikeye-agent.exe`, `strikeye-cli.exe` — that share one module, one set of
embedded assets and one set of CJK fonts. Two of them are strictly redundant:
`strikeye.exe --agent` already runs the agent in-process and the desktop shell
spawns itself that way, and the CLI is just a set of subcommands.

That redundancy costs twice. In bytes: the fonts, radar images and nav meshes
are embedded three times, and the three binaries take 137 MB of the package. In
operational clarity: three files to sign, document, version and keep in sync
during a hot update, plus a fallback path that can spawn a stale sibling copy.

## What changes

- `strikeye.exe` becomes the only shipped executable. It dispatches on
  `argv[1]`: nothing → desktop shell, `agent` → headless background service,
  `cli <command>` → the command line. The program name is honoured too, so a copy
  named `strikeye-cli.exe` or `strikeye-agent.exe` behaves like the old binaries.
- The desktop shell keeps self-spawning its agent through `--agent`, now from the
  same file with no fallback ambiguity.
- The command line runs inside a GUI-subsystem binary, so the CLI entry point
  attaches to the parent console before parsing (the `python.exe` trick).
- The hot updater deletes the superseded split executables when a package brings
  the merged binary, and leaves them alone for packages that still ship them.
- `cmd/strikeye-cli` and `cmd/strikeye-agent` stay as thin development shims so
  `go run ./cmd/strikeye-cli …` and CI scripts keep working; they are no longer
  packaged.

## Impact

- Affected specs: `native-ui-shell-and-guard` (dispatch contract),
  `ci-pipeline`, `bundled-map-data` (one executable in the package).
- Affected code: `internal/cli` (moved from `cmd/strikeye-cli`),
  `cmd/strikeye` (dispatch, console attach), `internal/updater`,
  `.github/workflows/release.yml`.
- Measured: `strikeye-windows-amd64.zip` 255 MB → 215 MB; the executable section
  drops from 137 MB to 63.6 MB with no capability removed.
