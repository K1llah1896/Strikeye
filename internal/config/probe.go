package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// ToolStatus describes the probe result for one external tool or asset.
type ToolStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// ProbeReport holds availability results for Strikeye's external dependencies.
// OBS is intentionally absent.
type ProbeReport struct {
	Steam          ToolStatus `json:"steam"`
	CS2            ToolStatus `json:"cs2"`
	HLAE           ToolStatus `json:"hlae"`
	AfxHookSource2 ToolStatus `json:"afx_hook_source2"`
	FFmpeg         ToolStatus `json:"ffmpeg"`
	FFprobe        ToolStatus `json:"ffprobe"`
}

// ProbeOptions customizes path probing for explicit overrides and unit tests.
type ProbeOptions struct {
	Configured      ToolPaths
	ExtraSteamRoots []string
	ExtraSearchDirs []string
	EnvLookup       func(string) string
	SkipSystemPATH  bool
}

var vdfPathRegexp = regexp.MustCompile(`"path"\s+"([^"]*)"`)

// ParseSteamLibraryFoldersVDF parses Steam's libraryfolders.vdf and returns
// normalized library root paths referenced inside it.
func ParseSteamLibraryFoldersVDF(vdfPath string) []string {
	data, err := os.ReadFile(vdfPath)
	if err != nil {
		return nil
	}
	matches := vdfPathRegexp.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		raw := strings.TrimSpace(strings.ReplaceAll(m[1], `\\`, `\`))
		if raw == "" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out
}

// ProbeToolchain checks configured paths, environment overrides, Steam library manifests,
// and platform candidate locations for Steam, CS2, HLAE, AfxHookSource2, FFmpeg, and FFprobe.
func ProbeToolchain(opts ProbeOptions) ProbeReport {
	if opts.EnvLookup == nil {
		opts.EnvLookup = os.Getenv
	}

	steamRoots := collectSteamRoots(opts)
	report := ProbeReport{
		Steam:          probeSteam(opts, steamRoots),
		CS2:            probeCS2(opts, steamRoots),
		HLAE:           probeHLAE(opts),
		FFmpeg:         probeBinary("ffmpeg", opts.Configured.FFmpegExe, opts.EnvLookup("STRIKEYE_FFMPEG_EXE"), ffmpegCandidates(opts)),
		FFprobe:        probeBinary("ffprobe", opts.Configured.FFprobeExe, opts.EnvLookup("STRIKEYE_FFPROBE_EXE"), ffprobeCandidates(opts)),
		AfxHookSource2: ToolStatus{Name: "afx_hook_source2", Available: false, Reason: "not found"},
	}

	report.AfxHookSource2 = probeAfxHook(opts, report.HLAE.Path)
	return report
}

func collectSteamRoots(opts ProbeOptions) []string {
	seen := make(map[string]struct{})
	var roots []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || !isDirSafe(p) {
			return
		}
		clean := filepath.Clean(p)
		key := normCaseKey(clean)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		roots = append(roots, clean)
	}

	add(opts.Configured.SteamDir)
	if envSteam := opts.EnvLookup("STRIKEYE_STEAM_DIR"); envSteam != "" {
		add(envSteam)
	}
	for _, extra := range opts.ExtraSteamRoots {
		add(extra)
	}
	for _, p := range platformSteamRoots(opts.EnvLookup) {
		add(p)
	}

	// Expand libraryfolders.vdf from discovered Steam installations.
	baseCount := len(roots)
	for i := 0; i < baseCount; i++ {
		vdf := filepath.Join(roots[i], "config", "libraryfolders.vdf")
		for _, lib := range ParseSteamLibraryFoldersVDF(vdf) {
			add(lib)
		}
	}
	return roots
}

func probeSteam(opts ProbeOptions, steamRoots []string) ToolStatus {
	if opts.Configured.SteamDir != "" {
		if isDirSafe(opts.Configured.SteamDir) {
			return ToolStatus{Name: "steam", Available: true, Path: filepath.Clean(opts.Configured.SteamDir)}
		}
		return ToolStatus{Name: "steam", Available: false, Reason: "configured steam_dir does not exist"}
	}
	if len(steamRoots) > 0 {
		return ToolStatus{Name: "steam", Available: true, Path: steamRoots[0]}
	}
	return ToolStatus{Name: "steam", Available: false, Reason: "steam installation not found"}
}

func probeCS2(opts ProbeOptions, steamRoots []string) ToolStatus {
	if configured := strings.TrimSpace(opts.Configured.CS2Exe); configured != "" {
		if isFileSafe(configured) {
			return ToolStatus{Name: "cs2", Available: true, Path: filepath.Clean(configured)}
		}
		return ToolStatus{Name: "cs2", Available: false, Reason: "configured cs2_exe not found"}
	}
	if envPath := strings.TrimSpace(opts.EnvLookup("STRIKEYE_CS2_EXE")); envPath != "" && isFileSafe(envPath) {
		return ToolStatus{Name: "cs2", Available: true, Path: filepath.Clean(envPath)}
	}
	rel := filepath.Join("steamapps", "common", "Counter-Strike Global Offensive", "game", "bin", "win64", "cs2.exe")
	for _, root := range steamRoots {
		candidate := filepath.Join(root, rel)
		if isFileSafe(candidate) {
			return ToolStatus{Name: "cs2", Available: true, Path: candidate}
		}
	}
	return ToolStatus{Name: "cs2", Available: false, Reason: "cs2.exe not found in steam libraries"}
}

func probeHLAE(opts ProbeOptions) ToolStatus {
	return probeBinary("hlae", opts.Configured.HLAEExe, opts.EnvLookup("STRIKEYE_HLAE_EXE"), hlaeCandidates(opts))
}

func probeAfxHook(opts ProbeOptions, resolvedHLAE string) ToolStatus {
	if configured := strings.TrimSpace(opts.Configured.AfxHookSource2DLL); configured != "" {
		if isFileSafe(configured) {
			return ToolStatus{Name: "afx_hook_source2", Available: true, Path: filepath.Clean(configured)}
		}
		return ToolStatus{Name: "afx_hook_source2", Available: false, Reason: "configured afx_hook_source2_dll not found"}
	}
	if resolvedHLAE != "" {
		hlaeDir := filepath.Dir(resolvedHLAE)
		for _, rel := range []string{filepath.Join("x64", "AfxHookSource2.dll"), "AfxHookSource2.dll"} {
			candidate := filepath.Join(hlaeDir, rel)
			if isFileSafe(candidate) {
				return ToolStatus{Name: "afx_hook_source2", Available: true, Path: candidate}
			}
		}
	}
	return ToolStatus{Name: "afx_hook_source2", Available: false, Reason: "AfxHookSource2.dll not found alongside HLAE"}
}

func probeBinary(name, configured, envOverride string, candidates []string) ToolStatus {
	if configured = strings.TrimSpace(configured); configured != "" {
		if isFileSafe(configured) {
			return ToolStatus{Name: name, Available: true, Path: filepath.Clean(configured)}
		}
		return ToolStatus{Name: name, Available: false, Reason: "configured path does not exist"}
	}
	if envOverride = strings.TrimSpace(envOverride); envOverride != "" {
		if isFileSafe(envOverride) {
			return ToolStatus{Name: name, Available: true, Path: filepath.Clean(envOverride)}
		}
	}
	for _, c := range candidates {
		if isFileSafe(c) {
			return ToolStatus{Name: name, Available: true, Path: filepath.Clean(c)}
		}
	}
	return ToolStatus{Name: name, Available: false, Reason: name + " executable not found"}
}

func hlaeCandidates(opts ProbeOptions) []string {
	var out []string
	for _, d := range opts.ExtraSearchDirs {
		out = append(out, filepath.Join(d, "HLAE.exe"), filepath.Join(d, "hlae", "HLAE.exe"))
	}
	out = append(out, platformHLAECandidates(opts.EnvLookup)...)
	return out
}

func ffmpegCandidates(opts ProbeOptions) []string {
	return toolCandidates("ffmpeg", opts)
}

func ffprobeCandidates(opts ProbeOptions) []string {
	return toolCandidates("ffprobe", opts)
}

func toolCandidates(base string, opts ProbeOptions) []string {
	exeName := base
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	var out []string
	for _, d := range opts.ExtraSearchDirs {
		out = append(out, filepath.Join(d, exeName), filepath.Join(d, "bin", exeName))
		if runtime.GOOS != "windows" {
			out = append(out, filepath.Join(d, base+".exe"), filepath.Join(d, "bin", base+".exe"))
		}
	}
	if !opts.SkipSystemPATH {
		if found, err := exec.LookPath(base); err == nil {
			out = append(out, found)
		}
	}
	out = append(out, platformMediaToolCandidates(base, opts.EnvLookup)...)
	return out
}

func isDirSafe(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFileSafe(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func normCaseKey(p string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(p)
	}
	return p
}
