//go:build !windows

package main

// attachConsoleForCLI is a no-op where the process already inherits the
// terminal's standard streams.
func attachConsoleForCLI() {}

// detachConsoleForDesktop is a no-op where the desktop shell never owned a
// console window in the first place.
func detachConsoleForDesktop() {}
