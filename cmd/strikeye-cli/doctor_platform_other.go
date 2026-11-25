//go:build !windows

package main

func ipcSocketKind() string { return "unix_socket" }
