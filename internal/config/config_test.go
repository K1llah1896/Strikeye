package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDataLayoutAndLoadDefaults(t *testing.T) {
	root := filepath.Join(t.TempDir(), "strikeye-data")

	cfg, status, err := Load(root)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !status.CreatedDefault {
		t.Fatalf("expected CreatedDefault=true on clean root")
	}
	if status.RecoveredFromCorrupt {
		t.Fatalf("unexpected RecoveredFromCorrupt=true")
	}

	layout, err := ResolveDataLayout(root)
	if err != nil {
		t.Fatalf("ResolveDataLayout failed: %v", err)
	}
	requiredDirs := []string{
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
	for _, dir := range requiredDirs {
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			t.Fatalf("expected directory %q to exist, err=%v", dir, statErr)
		}
	}

	if cfg.Render.Policy != PolicySmartHighlights || cfg.Render.Width != 1920 || cfg.Render.FPS != 60 {
		t.Fatalf("unexpected default render config: %+v", cfg.Render)
	}
	if _, err := os.Stat(ConfigPath(root)); err != nil {
		t.Fatalf("expected config.json to be written: %v", err)
	}
}

func TestSaveAndLoadAtomicRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root)
	cfg.PrimarySteamID64 = "76561198000000001"
	cfg.DemoWatchDirs = []string{filepath.Join(root, "demos")}
	cfg.Render.Policy = PolicyFullPOV
	cfg.Render.FPS = 120

	if err := Save(root, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Ensure no leftover temporary files remain.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".config-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("found leftover temp config file: %s", e.Name())
		}
	}

	loaded, status, err := Load(root)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if status.CreatedDefault || status.RecoveredFromCorrupt {
		t.Fatalf("unexpected load status: %+v", status)
	}
	if loaded.PrimarySteamID64 != cfg.PrimarySteamID64 || loaded.Render.Policy != PolicyFullPOV || loaded.Render.FPS != 120 {
		t.Fatalf("loaded config mismatch: got %+v, want %+v", loaded, cfg)
	}
}

func TestLoadQuarantinesCorruptConfig(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureDataLayout(root); err != nil {
		t.Fatalf("EnsureDataLayout failed: %v", err)
	}
	badJSON := []byte("{broken json")
	if err := os.WriteFile(ConfigPath(root), badJSON, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, status, err := Load(root)
	if err != nil {
		t.Fatalf("Load failed on corrupt config: %v", err)
	}
	if !status.RecoveredFromCorrupt || !status.CreatedDefault {
		t.Fatalf("expected corrupt recovery flags, got %+v", status)
	}
	if status.QuarantinePath == "" {
		t.Fatalf("expected non-empty QuarantinePath")
	}
	qBytes, err := os.ReadFile(status.QuarantinePath)
	if err != nil {
		t.Fatalf("failed reading quarantined file: %v", err)
	}
	if string(qBytes) != string(badJSON) {
		t.Fatalf("quarantined bytes mismatch: got %q", string(qBytes))
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("expected regenerated default config, got %+v", cfg)
	}
}

func TestProbeToolchainAvailableAndVDFDiscovery(t *testing.T) {
	tmp := t.TempDir()
	steamRoot := filepath.Join(tmp, "Steam")
	libRoot := filepath.Join(tmp, "SteamLibrary")
	cs2Exe := filepath.Join(libRoot, "steamapps", "common", "Counter-Strike Global Offensive", "game", "bin", "win64", "cs2.exe")
	hlaeExe := filepath.Join(tmp, "tools", "HLAE.exe")
	afxDLL := filepath.Join(tmp, "tools", "x64", "AfxHookSource2.dll")
	ffmpegExe := filepath.Join(tmp, "tools", "bin", "ffmpeg.exe")
	ffprobeExe := filepath.Join(tmp, "tools", "bin", "ffprobe.exe")

	for _, dir := range []string{
		filepath.Join(steamRoot, "config"),
		filepath.Dir(cs2Exe),
		filepath.Dir(afxDLL),
		filepath.Dir(ffmpegExe),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"` + strings.ReplaceAll(libRoot, `\`, `\\`) + `"
	}
}`
	if err := os.WriteFile(filepath.Join(steamRoot, "config", "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatalf("write vdf: %v", err)
	}
	for _, f := range []string{cs2Exe, hlaeExe, afxDLL, ffmpegExe, ffprobeExe} {
		if err := os.WriteFile(f, []byte("stub"), 0o755); err != nil {
			t.Fatalf("write stub %q: %v", f, err)
		}
	}

	report := ProbeToolchain(ProbeOptions{
		ExtraSteamRoots: []string{steamRoot},
		ExtraSearchDirs: []string{filepath.Join(tmp, "tools")},
		EnvLookup:       func(string) string { return "" },
		SkipSystemPATH:  true,
	})

	if !report.Steam.Available || report.Steam.Path != steamRoot {
		t.Fatalf("unexpected steam probe: %+v", report.Steam)
	}
	if !report.CS2.Available || report.CS2.Path != cs2Exe {
		t.Fatalf("unexpected cs2 probe: %+v", report.CS2)
	}
	if !report.HLAE.Available || report.HLAE.Path != hlaeExe {
		t.Fatalf("unexpected hlae probe: %+v", report.HLAE)
	}
	if !report.AfxHookSource2.Available || report.AfxHookSource2.Path != afxDLL {
		t.Fatalf("unexpected afxhook probe: %+v", report.AfxHookSource2)
	}
	if !report.FFmpeg.Available || report.FFmpeg.Path != ffmpegExe {
		t.Fatalf("unexpected ffmpeg probe: %+v", report.FFmpeg)
	}
	if !report.FFprobe.Available || report.FFprobe.Path != ffprobeExe {
		t.Fatalf("unexpected ffprobe probe: %+v", report.FFprobe)
	}
}

func TestProbeToolchainMissingAndOverrides(t *testing.T) {
	tmp := t.TempDir()
	customFFmpeg := filepath.Join(tmp, "custom-ffmpeg")
	if err := os.WriteFile(customFFmpeg, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write custom ffmpeg: %v", err)
	}

	report := ProbeToolchain(ProbeOptions{
		Configured: ToolPaths{
			FFmpegExe: customFFmpeg,
			HLAEExe:   filepath.Join(tmp, "does-not-exist", "HLAE.exe"),
		},
		EnvLookup:      func(string) string { return "" },
		SkipSystemPATH: true,
	})

	if !report.FFmpeg.Available || report.FFmpeg.Path != customFFmpeg {
		t.Fatalf("expected configured ffmpeg override to succeed, got %+v", report.FFmpeg)
	}
	if report.HLAE.Available || report.HLAE.Reason == "" {
		t.Fatalf("expected missing configured HLAE to report unavailable with reason, got %+v", report.HLAE)
	}
	if report.AfxHookSource2.Available {
		t.Fatalf("expected afx_hook_source2 unavailable when HLAE is missing, got %+v", report.AfxHookSource2)
	}
	if report.FFprobe.Available {
		t.Fatalf("expected ffprobe unavailable, got %+v", report.FFprobe)
	}
}

func TestThemeConfigNormalizationAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root)
	cfg.Theme = ThemeConfig{
		Preset:          "unknown_preset",
		AccentColor:     "not-a-hex",
		BackgroundImage: "  /tmp/bg.png  ",
		OverlayOpacity:  2.5,
		CardOpacity:     0.1,
	}
	if err := Save(root, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, _, err := Load(root)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Theme.Preset != ThemePresetPureBlack {
		t.Fatalf("expected default preset pure_black, got %q", loaded.Theme.Preset)
	}
	if loaded.Theme.AccentColor != "#22C55E" {
		t.Fatalf("expected default accent color #22C55E, got %q", loaded.Theme.AccentColor)
	}
	if loaded.Theme.BackgroundImage != "/tmp/bg.png" {
		t.Fatalf("expected trimmed background image, got %q", loaded.Theme.BackgroundImage)
	}
	if loaded.Theme.OverlayOpacity != 0.95 {
		t.Fatalf("expected clamped overlay opacity 0.95, got %v", loaded.Theme.OverlayOpacity)
	}
	if loaded.Theme.CardOpacity != 0.4 {
		t.Fatalf("expected clamped card opacity 0.4, got %v", loaded.Theme.CardOpacity)
	}

	loaded.Theme.Preset = ThemePresetMidnightBlue
	loaded.Theme.AccentColor = "#38bdf8"
	if err := Save(root, loaded); err != nil {
		t.Fatalf("Save custom theme failed: %v", err)
	}
	reloaded, _, err := Load(root)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if reloaded.Theme.Preset != ThemePresetMidnightBlue || reloaded.Theme.AccentColor != "#38BDF8" {
		t.Fatalf("unexpected reloaded theme: %+v", reloaded.Theme)
	}
}

func TestDefaultDataRootUsesExecutableDir(t *testing.T) {
	root, err := DefaultDataRoot()
	if err != nil {
		t.Fatalf("DefaultDataRoot failed: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable failed: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		exe = resolved
	}
	want := filepath.Clean(filepath.Dir(exe))
	if root != want {
		t.Fatalf("expected DefaultDataRoot %q, got %q", want, root)
	}
}
