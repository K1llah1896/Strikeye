package main

import (
	"path/filepath"
	"testing"

	"strikeye/internal/ipc"
)

func TestStartupMutexGuardDetection(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), "locks")
	renderLock := ipc.NewLockInDir(lockDir, ipc.MutexCS2RenderActive)

	held, err := shouldShowGuardDialog(renderLock)
	if err != nil || held {
		t.Fatalf("expected render lock not held initially, got held=%v err=%v", held, err)
	}

	ok, err := renderLock.TryAcquire()
	if err != nil || !ok {
		t.Fatalf("expected acquire render lock, got ok=%v err=%v", ok, err)
	}
	defer func() { _ = renderLock.Release() }()

	held, err = shouldShowGuardDialog(renderLock)
	if err != nil || !held {
		t.Fatalf("expected render lock held to trigger guard dialog, got held=%v err=%v", held, err)
	}
}
