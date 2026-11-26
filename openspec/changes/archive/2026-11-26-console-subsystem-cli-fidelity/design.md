# Design

## Linkage choice

Two ways to ship one binary with both a GUI and a CLI:

| | GUI subsystem (`-H windowsgui`) | Console subsystem |
| --- | --- | --- |
| double-click the shell | no console at all | console window exists, hidden and freed immediately |
| `strikeye cli …` in cmd/PowerShell | prompt returns first, output interleaves, exit code lost | blocks, streams, exit code propagates |
| `strikeye cli …` from Explorer | output dropped, files fine | own console appears, output visible, files fine |

Script fidelity is a functional property; a console window that exists for a few
tens of milliseconds is cosmetic and can be removed before anything is painted in
the common case. So the console subsystem wins.

## Detaching without stealing the user's terminal

`GetConsoleProcessList` returns how many processes share the console. One means
Windows created it for this process (double-click, or launched by a shell that
owns no console) — hide it with `ShowWindow(SW_HIDE)` and release it with
`FreeConsole`. Two or more means a real terminal is attached (the user ran
`strikeye` in cmd) — leave everything alone: hiding it would hide their shell, and
the console log is useful when debugging startup.

`detachConsoleForDesktop` runs first thing in `main` for the desktop path only, so
`--version`/`--env-check` from a terminal keep printing into that terminal.

## The agent child

The self-spawned agent already passes `DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP`
plus `HideWindow`, so a console-subsystem child still has no console and no
window. Nothing changes there.

## Compatibility with GUI-linked builds

`attachConsoleForCLI` stays: when the process already owns a console it returns
immediately, and when it has none it borrows the parent's and rebinds the standard
streams. A developer who builds with the README's `-H windowsgui` line gets a
working CLI, minus the "parent waits" guarantee that only the console subsystem
provides.

## Verification

`file strikeye.exe` reports `PE32+ executable (console) MS Windows`; the release
zip is built from the same command. The dispatch table itself is covered by
`cmd/strikeye/dispatch_test.go`, and the CLI path is exercised end to end on Linux
(`strikeye cli doctor|navs|parse|killctx|export`).
