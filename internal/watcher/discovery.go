package watcher

import (
	"os"
	"path/filepath"
	"strings"

	"strikeye/internal/config"
)

// WatchTarget represents a resolved directory to monitor for CS2 .dem files.
type WatchTarget struct {
	Dir        string
	Platform   string
	AutoProbed bool
}

// DiscoverOptions customizes watch-directory discovery for tests and overrides.
type DiscoverOptions struct {
	Config       config.Config
	Probe        config.ProbeReport
	ExtraTargets []WatchTarget
	EnvLookup    func(string) string
}

// DiscoverWatchTargets collects configured DemoWatchDirs together with existing
// auto-probed Valve CS2, Perfect World, and 5E demo directories.
func DiscoverWatchTargets(opts DiscoverOptions) []WatchTarget {
	if opts.EnvLookup == nil {
		opts.EnvLookup = os.Getenv
	}

	seen := make(map[string]struct{})
	var targets []WatchTarget

	add := func(dir, platform string, autoProbed bool) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		clean := filepath.Clean(dir)
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			return
		}
		key := normPathKey(clean)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		if platform == "" {
			platform = ClassifyPlatform(clean)
		}
		targets = append(targets, WatchTarget{
			Dir:        clean,
			Platform:   platform,
			AutoProbed: autoProbed,
		})
	}

	for _, dir := range opts.Config.DemoWatchDirs {
		add(dir, ClassifyPlatform(dir), false)
	}
	for _, extra := range opts.ExtraTargets {
		add(extra.Dir, extra.Platform, extra.AutoProbed)
	}

	// Auto-probe CS2 replays directory from configured or probed cs2.exe path.
	for _, cs2Path := range []string{opts.Config.Paths.CS2Exe, opts.Probe.CS2.Path} {
		if replays := cs2ReplaysDirFromExe(cs2Path); replays != "" {
			add(replays, PlatformValve, true)
		}
	}

	for _, cand := range platformAutoTargets(opts.EnvLookup) {
		add(cand.Dir, cand.Platform, true)
	}

	return targets
}

func cs2ReplaysDirFromExe(cs2Exe string) string {
	cs2Exe = strings.TrimSpace(cs2Exe)
	if cs2Exe == "" {
		return ""
	}
	// Typical layout: .../Counter-Strike Global Offensive/game/bin/win64/cs2.exe
	// -> .../Counter-Strike Global Offensive/game/csgo/replays
	clean := filepath.Clean(cs2Exe)
	lower := strings.ToLower(filepath.ToSlash(clean))
	if idx := strings.Index(lower, "/game/bin/"); idx >= 0 {
		root := clean[:idx]
		return filepath.Join(root, "game", "csgo", "replays")
	}
	return ""
}

func normPathKey(p string) string {
	if os.PathSeparator == '\\' {
		return strings.ToLower(p)
	}
	return p
}
