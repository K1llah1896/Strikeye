// Package ipc provides cross-process singleton/render mutexes and a bidirectional
// JSON-RPC 2.0 + event-streaming IPC channel between strikeye.exe and strikeye-agent.exe.
package ipc

const (
	MutexAgentSingleton  = `Local\StrikeyeAgentSingletonMutex`
	MutexUISingleton     = `Local\StrikeyeUISingletonMutex`
	MutexCS2RenderActive = `Local\StrikeyeCS2RenderActiveMutex`

	DefaultPipePath = `\\.\pipe\strikeye-ipc`
)

// Lock represents a cross-process named mutual-exclusion lock.
type Lock interface {
	Name() string
	TryAcquire() (bool, error)
	IsHeld() (bool, error)
	Release() error
}

// NewLock constructs a named cross-process lock using the OS default namespace/directory.
func NewLock(name string) Lock {
	return newPlatformLock("", name)
}

// NewLockInDir constructs a named cross-process lock scoped to dir on file-backed
// platforms (ignored on Windows where kernel Local\ namespace is used).
func NewLockInDir(dir, name string) Lock {
	return newPlatformLock(dir, name)
}
