//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func detachAgentProcess(cmd *exec.Cmd) {
	const createNewProcessGroup = 0x00000200
	const detachedProcess = 0x00000008
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess,
		HideWindow:    true,
	}
}
