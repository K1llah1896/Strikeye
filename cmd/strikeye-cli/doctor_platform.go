package main

import "runtime"

// platformReport summarises what this build can do on the host OS.
type platformReport struct {
	OS             string   `json:"os"`
	Arch           string   `json:"arch"`
	IPCLocalSocket string   `json:"ipc_local_socket"`
	HLAERecording  bool     `json:"hlae_recording"`
	Notes          []string `json:"notes,omitempty"`
}

// describePlatform reports the capabilities that depend on the host OS, so a
// Linux/macOS user learns why recording is unavailable instead of hitting an
// obscure failure deep in the render pipeline.
func describePlatform() platformReport {
	rep := platformReport{
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		IPCLocalSocket: ipcSocketKind(),
		HLAERecording:  runtime.GOOS == "windows",
	}
	if !rep.HLAERecording {
		rep.Notes = append(rep.Notes,
			"HLAE/CS2 recording needs Windows; analysis, replay, export and CLI tools work here.")
	}
	if runtime.GOOS == "windows" {
		rep.Notes = append(rep.Notes, "IPC uses named pipes.")
	} else {
		rep.Notes = append(rep.Notes, "IPC falls back to a Unix domain socket.")
	}
	return rep
}
