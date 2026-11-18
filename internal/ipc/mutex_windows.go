//go:build windows

package ipc

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sys/windows"
)

type windowsLock struct {
	name   string
	mu     sync.Mutex
	handle windows.Handle
}

func newPlatformLock(_ string, name string) Lock {
	return &windowsLock{name: name}
}

func (l *windowsLock) Name() string {
	return l.name
}

func (l *windowsLock) TryAcquire() (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.handle != 0 {
		return true, nil
	}

	namePtr, err := windows.UTF16PtrFromString(l.name)
	if err != nil {
		return false, fmt.Errorf("encode mutex name %q: %w", l.name, err)
	}
	h, err := windows.CreateMutex(nil, false, namePtr)
	if h == 0 {
		return false, fmt.Errorf("CreateMutex(%q): %w", l.name, err)
	}

	waitRes, waitErr := windows.WaitForSingleObject(h, 0)
	switch waitRes {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		l.handle = h
		return true, nil
	case uint32(windows.WAIT_TIMEOUT):
		_ = windows.CloseHandle(h)
		return false, nil
	default:
		_ = windows.CloseHandle(h)
		return false, fmt.Errorf("WaitForSingleObject(%q)=%d: %w", l.name, waitRes, waitErr)
	}
}

func (l *windowsLock) IsHeld() (bool, error) {
	l.mu.Lock()
	if l.handle != 0 {
		l.mu.Unlock()
		return true, nil
	}
	l.mu.Unlock()

	namePtr, err := windows.UTF16PtrFromString(l.name)
	if err != nil {
		return false, err
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, namePtr)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return false, nil
		}
		return false, fmt.Errorf("OpenMutex(%q): %w", l.name, err)
	}
	defer func() {
		_ = windows.CloseHandle(h)
	}()

	waitRes, waitErr := windows.WaitForSingleObject(h, 0)
	switch waitRes {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		_ = windows.ReleaseMutex(h)
		return false, nil
	case uint32(windows.WAIT_TIMEOUT):
		return true, nil
	default:
		return false, fmt.Errorf("WaitForSingleObject probe(%q)=%d: %w", l.name, waitRes, waitErr)
	}
}

func (l *windowsLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.handle == 0 {
		return nil
	}
	h := l.handle
	l.handle = 0
	relErr := windows.ReleaseMutex(h)
	closeErr := windows.CloseHandle(h)
	if relErr != nil {
		return relErr
	}
	return closeErr
}
