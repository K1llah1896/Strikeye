# Design

## Dispatch rules

`resolveMode(program, args)` is a pure function, tested on its own, and the only
place that decides which entry point runs:

| Input | Mode |
| --- | --- |
| program name ends in `-cli` (after stripping `.exe`) | CLI |
| program name ends in `-agent` | agent |
| first argument is `cli` (case-insensitive) | CLI, argument dropped |
| first argument is `agent` | agent, argument dropped |
| anything else | desktop shell |

The program-name rule exists because Windows has no usable argv[0] symlink story
and because developers still build `./cmd/strikeye-cli`. It also means an old
`.lnk`, script or shortcut pointing at `strikeye.exe doctor` keeps working — that
path just falls through to the shell and prints its usage, which is a footgun, so
`strikeye cli` is the documented spelling and the shims are what old scripts use.

`--agent` keeps working as a GUI flag: the self-spawn path predates the merge and
is exercised by the guard dialog flow.

## Console attach on Windows

The shipped binary links with `-H windowsgui`, otherwise every launch of the
desktop shell flashes a console window. A GUI-subsystem process has no console,
so `strikeye cli export …` would write into the void.

`attachConsoleForCLI` calls `AttachConsole(ATTACH_PARENT_PROCESS)` and rebinds
`os.Stdout`/`os.Stderr` (and `os.Stdin` for the interactive cases) to
`CONOUT$`/`CONIN$`. When there is no parent console — launched from Explorer or a
detached parent — the attach fails and writes go nowhere, exactly as before; the
CLI's file outputs are unaffected. Implemented with `syscall.NewLazyDLL` so no
new dependency is needed.

## Why not a `--cli` flag instead of a subcommand

A flag would have to be parsed before the GUI's flag set, and `strikeye --version`
meaning two different things depending on the mode is worse than a word. The
subcommand reads like every other CLI on the platform.

## Package layout

The zip keeps its shape minus two entries:

```
strikeye.exe
map_geometry.zip            (stored, 119.9 MB)
ffmpeg-windows-amd64.tar.zst (stored, 51.3 MB)
hlae-windows-amd64.zip      (stored, 9.1 MB)
SHA256SUMS.txt
```

Upgrade path: a 0.4.0 install that updates to 0.5.0 keeps `strikeye-agent.exe`
and `strikeye-cli.exe` on disk until the updater removes them. That removal is
deliberate (a stale agent binary is a real fallback hazard) and it only runs when
the package carries `strikeye[.exe]` and does not carry the split names, so
downgrade packages are untouched.

## Measured

| | 0.4.0 | 0.5.0 |
| --- | --- | --- |
| executables on disk | 62.6 + 43.2 + 31.1 = 136.9 MB | 63.6 MB |
| release zip | 255.1 MB | 215.0 MB |
