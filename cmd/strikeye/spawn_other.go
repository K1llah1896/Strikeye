//go:build !windows

package main

import "os/exec"

func detachAgentProcess(_ *exec.Cmd) {}
