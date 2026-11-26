# Console subsystem for CLI fidelity

## Why

0.5.0 merged the three executables but kept the desktop shell's
`-H windowsgui` linkage, so the whole binary runs as a GUI-subsystem process.
Windows treats such a process specially: `cmd.exe` and PowerShell do not wait for
it, so `strikeye cli export …` returned the prompt immediately, printed its
output after it, and never handed its exit code to the script.

Borrowing the parent console (`AttachConsole`) fixed where the bytes land but not
who waits for them. The CLI looked fine interactively and broke in scripts.

## What changes

- The executable links as a console application, so `strikeye cli …` blocks,
  streams and exits like any command-line tool, and `%ERRORLEVEL%` propagates.
- The desktop shell hides and frees its own console when Windows created one just
  for it (double-click), so no terminal window is left behind.
- A console shared with a real terminal is never hidden: the user's shell window
  and its log output stay put.
- Keeping `attachConsoleForCLI` means a binary rebuilt with `-H windowsgui`
  (README's manual build line used to do that) still prints somewhere.

## Impact

- Affected specs: `native-ui-shell-and-guard` (dispatch requirement, CLI output
  scenario).
- Affected code: `cmd/strikeye/{main,console_windows,console_other}.go`,
  `.github/workflows/release.yml`, README and the research report.
