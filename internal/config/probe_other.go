//go:build !windows

package config

import "path/filepath"

func platformSteamRoots(getenv func(string) string) []string {
	var roots []string
	if home := getenv("HOME"); home != "" {
		roots = append(roots,
			filepath.Join(home, ".steam", "steam"),
			filepath.Join(home, ".local", "share", "Steam"),
		)
	}
	return roots
}

func platformHLAECandidates(getenv func(string) string) []string {
	if home := getenv("HOME"); home != "" {
		return []string{filepath.Join(home, "HLAE", "HLAE.exe")}
	}
	return nil
}

func platformMediaToolCandidates(_ string, _ func(string) string) []string {
	return nil
}
