//go:build windows

package recorder

import (
	"context"
	"os/exec"
	"syscall"
)

func startDetachedCommand(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
