//go:build windows

package watcher

import "path/filepath"

func platformAutoTargets(getenv func(string) string) []WatchTarget {
	var out []WatchTarget
	if envPW := getenv("STRIKEYE_PW_DEMO_DIR"); envPW != "" {
		out = append(out, WatchTarget{Dir: envPW, Platform: PlatformPerfectWorld, AutoProbed: true})
	}
	if env5E := getenv("STRIKEYE_5E_DEMO_DIR"); env5E != "" {
		out = append(out, WatchTarget{Dir: env5E, Platform: Platform5E, AutoProbed: true})
	}

	if local := getenv("LOCALAPPDATA"); local != "" {
		out = append(out,
			WatchTarget{Dir: filepath.Join(local, "PerfectWorldArena", "demos"), Platform: PlatformPerfectWorld, AutoProbed: true},
			WatchTarget{Dir: filepath.Join(local, "5EPlay", "demos"), Platform: Platform5E, AutoProbed: true},
		)
	}
	if appdata := getenv("APPDATA"); appdata != "" {
		out = append(out,
			WatchTarget{Dir: filepath.Join(appdata, "5EPlay", "demos"), Platform: Platform5E, AutoProbed: true},
		)
	}
	if pf86 := getenv("ProgramFiles(x86)"); pf86 != "" {
		out = append(out,
			WatchTarget{Dir: filepath.Join(pf86, "PerfectWorldArena", "demos"), Platform: PlatformPerfectWorld, AutoProbed: true},
		)
	}
	return out
}
