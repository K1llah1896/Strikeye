// Package config manages Strikeye's DataRoot directory layout, atomic config.json
// persistence, and external toolchain path probing.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataLayout holds resolved paths for every required DataRoot subdirectory.
type DataLayout struct {
	Root                string
	BackupDBSnapshots   string
	BackupCS2Configs    string
	CacheReplay32Hz     string
	CacheRadarCards     string
	CacheRadarAnims     string
	CacheRadarPortraits string
	CacheRadarTeamLogos string
	MediaPOVRounds      string
	MediaHighlights     string
	MediaExports        string
}

// DefaultDataRoot returns the directory containing the running executable
// so all user data (config.json, strikeye.db, cache, media, backup) stays portable
// alongside the binary rather than writing to the system drive (e.g. AppData).
func DefaultDataRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	if resolved, evalErr := filepath.EvalSymlinks(exe); evalErr == nil && resolved != "" {
		exe = resolved
	}
	return filepath.Clean(filepath.Dir(exe)), nil
}

// ResolveDataLayout computes all DataRoot subdirectory paths without creating them.
func ResolveDataLayout(root string) (DataLayout, error) {
	if root == "" {
		return DataLayout{}, fmt.Errorf("data root cannot be empty")
	}
	clean := filepath.Clean(root)
	return DataLayout{
		Root:                clean,
		BackupDBSnapshots:   filepath.Join(clean, "backup", "db_snapshots"),
		BackupCS2Configs:    filepath.Join(clean, "backup", "cs2_user_configs"),
		CacheReplay32Hz:     filepath.Join(clean, "cache", "replay_32hz"),
		CacheRadarCards:     filepath.Join(clean, "cache", "radar_cards", "cards"),
		CacheRadarAnims:     filepath.Join(clean, "cache", "radar_cards", "animations"),
		CacheRadarPortraits: filepath.Join(clean, "cache", "radar_cards", "portraits"),
		CacheRadarTeamLogos: filepath.Join(clean, "cache", "radar_cards", "team_logos"),
		MediaPOVRounds:      filepath.Join(clean, "media", "pov_rounds"),
		MediaHighlights:     filepath.Join(clean, "media", "highlights"),
		MediaExports:        filepath.Join(clean, "media", "exports"),
	}, nil
}

// EnsureDataLayout creates the DataRoot directory tree and verifies writability.
func EnsureDataLayout(root string) (DataLayout, error) {
	layout, err := ResolveDataLayout(root)
	if err != nil {
		return DataLayout{}, err
	}
	dirs := []string{
		layout.Root,
		layout.BackupDBSnapshots,
		layout.BackupCS2Configs,
		layout.CacheReplay32Hz,
		layout.CacheRadarCards,
		layout.CacheRadarAnims,
		layout.CacheRadarPortraits,
		layout.CacheRadarTeamLogos,
		layout.MediaPOVRounds,
		layout.MediaHighlights,
		layout.MediaExports,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return DataLayout{}, fmt.Errorf("create data directory %q: %w", d, err)
		}
	}
	return layout, nil
}
