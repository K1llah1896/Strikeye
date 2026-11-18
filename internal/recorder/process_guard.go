package recorder

import (
	"context"
	"strings"
)

// ProcessIdentity uniquely identifies a Strikeye-managed process instance so
// PID reuse or user-launched cs2.exe instances are never confused or killed.
type ProcessIdentity struct {
	PID          int
	CreationTime uint64
	SessionToken string
	Name         string
}

// JobHandle represents an OS-level process containment group (Windows Job Object
// with KILL_ON_JOB_CLOSE on Windows, or tracked process group on other OSes).
type JobHandle interface {
	Assign(pid int) error
	CloseAndKill() error
}

// ProcessGuard enforces Strikeye process ownership and detects external cs2.exe instances.
type ProcessGuard interface {
	HasExternalCS2(ctx context.Context, owned []ProcessIdentity) (bool, error)
	NewJobGroup(sessionToken string) (JobHandle, error)
}

// NewProcessGuard returns the platform-native ProcessGuard implementation.
func NewProcessGuard() ProcessGuard {
	return newPlatformProcessGuard()
}

// MatchesOwnedIdentity verifies whether candidate matches any owned ProcessIdentity
// by PID, CreationTime (when non-zero on both), and SessionToken (when present).
func MatchesOwnedIdentity(candidate ProcessIdentity, owned []ProcessIdentity) bool {
	for _, o := range owned {
		if o.PID <= 0 || candidate.PID != o.PID {
			continue
		}
		if o.CreationTime != 0 && candidate.CreationTime != 0 && candidate.CreationTime != o.CreationTime {
			continue
		}
		if o.SessionToken != "" && candidate.SessionToken != "" && !strings.Contains(candidate.SessionToken, o.SessionToken) {
			continue
		}
		return true
	}
	return false
}

// HasExternalProcess reports whether any process in snapshot matching targetName
// (case-insensitive) is NOT owned by Strikeye.
func HasExternalProcess(snapshot []ProcessIdentity, targetName string, owned []ProcessIdentity) bool {
	for _, proc := range snapshot {
		if !strings.EqualFold(proc.Name, targetName) {
			continue
		}
		if !MatchesOwnedIdentity(proc, owned) {
			return true
		}
	}
	return false
}
