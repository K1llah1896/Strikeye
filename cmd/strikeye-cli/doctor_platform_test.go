package main

import (
	"runtime"
	"testing"
)

func TestDescribePlatform(t *testing.T) {
	rep := describePlatform()
	if rep.OS != runtime.GOOS || rep.Arch != runtime.GOARCH {
		t.Fatalf("platform = %+v", rep)
	}
	if rep.HLAERecording != (runtime.GOOS == "windows") {
		t.Fatalf("hlae flag = %v on %s", rep.HLAERecording, runtime.GOOS)
	}
	if rep.IPCLocalSocket == "" || len(rep.Notes) == 0 {
		t.Fatalf("incomplete report = %+v", rep)
	}
	switch runtime.GOOS {
	case "windows":
		if rep.IPCLocalSocket != "named_pipe" {
			t.Fatalf("socket = %q", rep.IPCLocalSocket)
		}
	default:
		if rep.IPCLocalSocket != "unix_socket" {
			t.Fatalf("socket = %q", rep.IPCLocalSocket)
		}
	}
}
