//go:build !windows

package recorder

import (
	"context"
	"os"
	"sync"
)

type unixProcessGuard struct{}

func newPlatformProcessGuard() ProcessGuard {
	return &unixProcessGuard{}
}

func (g *unixProcessGuard) HasExternalCS2(_ context.Context, _ []ProcessIdentity) (bool, error) {
	return false, nil
}

type unixJobHandle struct {
	mu   sync.Mutex
	pids []int
}

func (g *unixProcessGuard) NewJobGroup(_ string) (JobHandle, error) {
	return &unixJobHandle{}, nil
}

func (j *unixJobHandle) Assign(pid int) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.pids = append(j.pids, pid)
	return nil
}

func (j *unixJobHandle) CloseAndKill() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, pid := range j.pids {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	}
	j.pids = nil
	return nil
}
