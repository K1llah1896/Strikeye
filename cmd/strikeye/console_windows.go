//go:build windows

package main

import (
	"os"
	"syscall"
)

// attachParentProcess is ATTACH_PARENT_PROCESS: the console of the parent.
const attachParentProcess = ^uintptr(0)

var (
	kernel32DLL   = syscall.NewLazyDLL("kernel32.dll")
	procAttachCon = kernel32DLL.NewProc("AttachConsole")
)

// attachConsoleForCLI points this process at the console of whoever launched it.
//
// The single binary links with the GUI subsystem so opening the desktop shell
// never flashes a console window. That leaves `strikeye cli …` without a console
// to write to, so the CLI entry points borrow the parent's: attach to it and
// rebind the standard streams, which is the same trick `python.exe` uses.
func attachConsoleForCLI() {
	if ok, _, _ := procAttachCon.Call(attachParentProcess); ok == 0 {
		// Launched from Explorer or a detached parent: there is no console to
		// borrow, and writes simply go nowhere. Output files still work.
		return
	}
	rebind("CONOUT$", &os.Stdout)
	rebind("CONOUT$", &os.Stderr)
	rebind("CONIN$", &os.Stdin)
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
