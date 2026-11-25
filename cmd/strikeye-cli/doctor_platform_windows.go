//go:build windows

package main

func ipcSocketKind() string { return "named_pipe" }
