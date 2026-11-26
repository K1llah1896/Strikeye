//go:build windows

package main

import (
	"os"
	"syscall"
)

const (
	// attachParentProcess is ATTACH_PARENT_PROCESS: the console of the parent.
	attachParentProcess = ^uintptr(0)
	// swHide is SW_HIDE for ShowWindow.
	swHide = 0
)

var (
	kernel32DLL       = syscall.NewLazyDLL("kernel32.dll")
	user32DLL         = syscall.NewLazyDLL("user32.dll")
	procAttachCon     = kernel32DLL.NewProc("AttachConsole")
	procFreeCon       = kernel32DLL.NewProc("FreeConsole")
	procGetConWindow  = kernel32DLL.NewProc("GetConsoleWindow")
	procGetConProcLis = kernel32DLL.NewProc("GetConsoleProcessList")
	procShowWindow    = user32DLL.NewProc("ShowWindow")
)

// attachConsoleForCLI makes sure a console-subsystem process launched without a
// terminal still has somewhere to write.
//
// The shipped binary links as a console application so that `strikeye cli …`
// from cmd.exe or PowerShell blocks, streams and returns its exit code like any
// other command-line tool. Launched from Explorer that means Windows created a
// console for us; for the CLI entry points it is the right window to print into,
// so nothing to do. The attach path covers a binary linked with `-H windowsgui`
// (as some build scripts do) and a fully detached parent: there the parent
// console is borrowed, and writes are dropped when there is none.
func attachConsoleForCLI() {
	if consoleWindow() != 0 {
		return
	}
	ok, _, _ := procAttachCon.Call(attachParentProcess)
	if ok == 0 {
		// Launched from Explorer with no parent console: writes go nowhere.
		// File outputs still work, which is what the export commands produce.
		return
	}
	rebind("CONOUT$", &os.Stdout)
	rebind("CONOUT$", &os.Stderr)
	rebind("CONIN$", &os.Stdin)
}

// detachConsoleForDesktop hides and releases the console of the desktop shell so
// double-clicking the executable does not leave a terminal window behind.
//
// A console shared with a real terminal (the user typed `strikeye` in cmd) is
// left alone: hiding it would hide their terminal, and the log output in it is
// useful. Only a console created solely for this process — process count 1 — is
// hidden and freed.
func detachConsoleForDesktop() {
	hwnd := consoleWindow()
	if hwnd == 0 {
		return
	}
	var pids [2]uint32
	count, _, _ := procGetConProcLis.Call(uintptr(unsafePointer(&pids[0])), uintptr(len(pids)))
	if count > 1 {
		return
	}
	_, _, _ = procShowWindow.Call(hwnd, swHide)
	_, _, _ = procFreeCon.Call()
}

// consoleWindow returns the HWND of the console this process is attached to, or
// 0 when it has none.
func consoleWindow() uintptr {
	hwnd, _, _ := procGetConWindow.Call()
	return hwnd
}

// rebind replaces a standard stream with a handle on the attached console.
func rebind(name string, target **os.File) {
	f, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		if f, err = os.OpenFile(name, os.O_WRONLY, 0); err != nil {
			return
		}
	}
	*target = f
}
