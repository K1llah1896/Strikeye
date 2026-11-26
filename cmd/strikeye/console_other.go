//go:build !windows

package main

// attachConsoleForCLI is a no-op where the process already inherits the
// terminal's standard streams.
func attachConsoleForCLI() {}
