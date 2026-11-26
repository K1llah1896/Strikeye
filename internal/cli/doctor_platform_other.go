//go:build !windows

package cli

func ipcSocketKind() string { return "unix_socket" }
