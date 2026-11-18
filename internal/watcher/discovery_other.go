//go:build !windows

package watcher

func platformAutoTargets(getenv func(string) string) []WatchTarget {
	var out []WatchTarget
	if envPW := getenv("STRIKEYE_PW_DEMO_DIR"); envPW != "" {
		out = append(out, WatchTarget{Dir: envPW, Platform: PlatformPerfectWorld, AutoProbed: true})
	}
	if env5E := getenv("STRIKEYE_5E_DEMO_DIR"); env5E != "" {
		out = append(out, WatchTarget{Dir: env5E, Platform: Platform5E, AutoProbed: true})
	}
	return out
}
