//go:build windows

package cli

func ipcSocketKind() string { return "named_pipe" }
