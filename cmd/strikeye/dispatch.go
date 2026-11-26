package main

import (
	"path/filepath"
	"strings"
)

// dispatchMode names the entry point a single binary should run.
type dispatchMode int

const (
	// modeDesktop runs the native UI shell, which also owns the agent spawn.
	modeDesktop dispatchMode = iota
	// modeAgent runs the headless background agent in this process.
	modeAgent
	// modeCLI runs one command-line subcommand and exits.
	modeCLI
)

// resolveMode maps a program name and its arguments onto an entry point.
//
// One binary carries all three entry points: the release ships `strikeye.exe`
// alone, `strikeye cli <command>` reaches the CLI, `strikeye agent` (or the
// legacy `--agent` flag) runs the background service, and everything else opens
// the desktop shell. The program name is consulted as well, so a copy renamed to
// `strikeye-cli.exe` — or a developer build from `cmd/strikeye-cli` — keeps
// behaving like the split binaries of older releases.
func resolveMode(program string, args []string) (dispatchMode, []string) {
	switch classifyProgram(program) {
	case modeAgent:
		return modeAgent, args
	case modeCLI:
		return modeCLI, args
	}
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "cli":
			return modeCLI, args[1:]
		case "agent":
			return modeAgent, args[1:]
		}
	}
	return modeDesktop, args
}

// classifyProgram recognizes the entry points encoded in the executable name.
func classifyProgram(program string) dispatchMode {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(program), ".exe"))
	switch {
	case name == "strikeye-cli", strings.HasSuffix(name, "-cli"):
		return modeCLI
	case name == "strikeye-agent", strings.HasSuffix(name, "-agent"):
		return modeAgent
	}
	return modeDesktop
}
