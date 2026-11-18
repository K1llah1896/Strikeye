//go:build !windows

package ipc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

type unixLock struct {
	name string
	path string
	mu   sync.Mutex
	file *os.File
}

func newPlatformLock(dir, name string) Lock {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "strikeye-locks")
	}
	sum := sha256.Sum256([]byte(name))
	filename := hex.EncodeToString(sum[:8]) + ".lock"
	return &unixLock{
		name: name,
		path: filepath.Join(dir, filename),
	}
}

func (l *unixLock) Name() string {
	return l.name
}

func (l *unixLock) TryAcquire() (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return false, fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, fmt.Errorf("open lock file %q: %w", l.path, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return false, nil
		}
		return false, fmt.Errorf("flock(%q): %w", l.path, err)
	}
	l.file = f
	return true, nil
}

func (l *unixLock) IsHeld() (bool, error) {
	l.mu.Lock()
	if l.file != nil {
		l.mu.Unlock()
		return true, nil
	}
	l.mu.Unlock()

	f, err := os.OpenFile(l.path, os.O_RDONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer func() {
		_ = f.Close()
	}()

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return true, nil
		}
		return false, err
	}
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return false, nil
}

func (l *unixLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	unErr := unix.Flock(int(f.Fd()), unix.LOCK_UN)
	closeErr := f.Close()
	if unErr != nil {
		return unErr
	}
	return closeErr
}
