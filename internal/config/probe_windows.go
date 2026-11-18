//go:build windows

package config

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func platformSteamRoots(getenv func(string) string) []string {
	var roots []string

	type regKey struct {
		root registry.Key
		path string
	}
	keys := []regKey{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`},
		{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`},
		{registry.CURRENT_USER, `Software\Valve\Steam`},
	}
	for _, k := range keys {
		rk, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		val, _, err := rk.GetStringValue("InstallPath")
		_ = rk.Close()
		if err == nil && strings.TrimSpace(val) != "" {
			roots = append(roots, val)
		}
	}

	if pf86 := getenv("ProgramFiles(x86)"); pf86 != "" {
		roots = append(roots, filepath.Join(pf86, "Steam"))
	}
	if pf := getenv("ProgramFiles"); pf != "" {
		roots = append(roots, filepath.Join(pf, "Steam"))
	}
	roots = append(roots,
		`C:\Program Files (x86)\Steam`,
		`C:\Steam`,
		`D:\Steam`,
		`D:\SteamLibrary`,
		`E:\SteamLibrary`,
	)
	return roots
}

func platformHLAECandidates(getenv func(string) string) []string {
	var out []string
	if local := getenv("LOCALAPPDATA"); local != "" {
		out = append(out, filepath.Join(local, "Programs", "HLAE", "HLAE.exe"))
	}
	if pf86 := getenv("ProgramFiles(x86)"); pf86 != "" {
		out = append(out, filepath.Join(pf86, "HLAE", "HLAE.exe"))
	}
	if pf := getenv("ProgramFiles"); pf != "" {
		out = append(out, filepath.Join(pf, "HLAE", "HLAE.exe"))
	}
	out = append(out,
		`C:\HLAE\HLAE.exe`,
		`C:\tools\HLAE\HLAE.exe`,
		`D:\HLAE\HLAE.exe`,
	)
	return out
}

func platformMediaToolCandidates(base string, getenv func(string) string) []string {
	exe := base + ".exe"
	var out []string
	if local := getenv("LOCALAPPDATA"); local != "" {
		out = append(out, filepath.Join(local, "Strikeye", "third_party", "ffmpeg", exe))
	}
	out = append(out,
		filepath.Join(`C:\ffmpeg\bin`, exe),
		filepath.Join(`C:\Program Files\ffmpeg\bin`, exe),
		filepath.Join(`C:\Program Files (x86)\ffmpeg\bin`, exe),
		filepath.Join(`D:\ffmpeg\bin`, exe),
		filepath.Join(`E:\ffmpeg\bin`, exe),
		filepath.Join(`C:\tools\ffmpeg\bin`, exe),
		filepath.Join(`C:\ProgramData\chocolatey\bin`, exe),
	)
	return out
}
