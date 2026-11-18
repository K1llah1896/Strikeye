//go:build windows

package recorder

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsProcessGuard struct{}

func newPlatformProcessGuard() ProcessGuard {
	return &windowsProcessGuard{}
}

func (g *windowsProcessGuard) HasExternalCS2(_ context.Context, owned []ProcessIdentity) (bool, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return false, fmt.Errorf("Process32First: %w", err)
	}

	var procs []ProcessIdentity
	for {
		exe := windows.UTF16ToString(entry.ExeFile[:])
		if exe == "cs2.exe" || exe == "CS2.EXE" {
			pid := int(entry.ProcessID)
			ct := queryProcessCreationTime(uint32(pid))
			procs = append(procs, ProcessIdentity{
				PID:          pid,
				CreationTime: ct,
				Name:         exe,
			})
		}
		err := windows.Process32Next(snap, &entry)
		if err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return false, fmt.Errorf("Process32Next: %w", err)
		}
	}
	return HasExternalProcess(procs, "cs2.exe", owned), nil
}

func queryProcessCreationTime(pid uint32) uint64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return 0
	}
	return uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime)
}

type windowsJobHandle struct {
	job          windows.Handle
	sessionToken string
	pids         []int
}

func (g *windowsProcessGuard) NewJobGroup(sessionToken string) (JobHandle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("CreateJobObject: %w", err)
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("SetInformationJobObject: %w", err)
	}
	return &windowsJobHandle{job: job, sessionToken: sessionToken}, nil
}

func (j *windowsJobHandle) Assign(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	defer windows.CloseHandle(h)

	if err := windows.AssignProcessToJobObject(j.job, h); err != nil {
		return fmt.Errorf("AssignProcessToJobObject(%d): %w", pid, err)
	}
	j.pids = append(j.pids, pid)
	return nil
}

func (j *windowsJobHandle) CloseAndKill() error {
	if j.job == 0 {
		return nil
	}
	_ = windows.TerminateJobObject(j.job, 1)
	err := windows.CloseHandle(j.job)
	j.job = 0
	if err != nil && err != windows.ERROR_INVALID_HANDLE {
		return err
	}
	return nil
}
