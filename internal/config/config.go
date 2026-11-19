package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CurrentSchemaVersion = 1

	PolicySmartHighlights = "smart_highlights"
	PolicyFullPOV         = "full_pov"
	PolicyManualOnly      = "manual_only"

	ThemePresetEsportsDark    = "esports_dark"
	ThemePresetMidnightBlue   = "midnight_blue"
	ThemePresetTacticalRed    = "tactical_red"
	ThemePresetLightClean     = "light_clean"
	ThemePresetPureBlack      = "pure_black"
	ThemePresetGradientAurora = "gradient_aurora"
	ThemePresetGradientSunset = "gradient_sunset"
)

// ToolPaths stores explicit user-configured paths to Strikeye's external tools.
type ToolPaths struct {
	SteamDir          string `json:"steam_dir,omitempty"`
	CS2Exe            string `json:"cs2_exe,omitempty"`
	HLAEExe           string `json:"hlae_exe,omitempty"`
	AfxHookSource2DLL string `json:"afx_hook_source2_dll,omitempty"`
	FFmpegExe         string `json:"ffmpeg_exe,omitempty"`
	FFprobeExe        string `json:"ffprobe_exe,omitempty"`
	FrameMeldExe      string `json:"framemeld_exe,omitempty"`
}

// RenderConfig configures background HLAE/CS2 capture and FFmpeg encoding defaults.
type RenderConfig struct {
	Policy                 string  `json:"policy"`
	Width                  int     `json:"width"`
	Height                 int     `json:"height"`
	FPS                    int     `json:"fps"`
	Encoder                string  `json:"encoder"`
	PreBufferSec           float64 `json:"pre_buffer_sec"`
	PostBufferSec          float64 `json:"post_buffer_sec"`
	KeyOverlayEnabled      bool    `json:"key_overlay_enabled"`
	FrameMeldEnabled       bool    `json:"framemeld_enabled"`
	HighlightImportanceMin float64 `json:"highlight_importance_min"`
}

// ThemeConfig configures UI theme preset, custom accent color, background image, and opacity.
type ThemeConfig struct {
	Preset          string  `json:"preset"`
	AccentColor     string  `json:"accent_color"`
	BackgroundImage string  `json:"background_image,omitempty"`
	OverlayOpacity  float64 `json:"overlay_opacity"`
	CardOpacity     float64 `json:"card_opacity"`
}

// UpdateConfig configures in-app GitHub Releases update checking.
type UpdateConfig struct {
	AutoCheck bool `json:"auto_check"`
}

// Config is the root persisted Strikeye configuration stored in <DataRoot>/config.json.
type Config struct {
	SchemaVersion        int          `json:"schema_version"`
	DataRoot             string       `json:"data_root"`
	DemoWatchDirs        []string     `json:"demo_watch_dirs"`
	PrimarySteamID64     string       `json:"primary_steam_id64,omitempty"`
	PrimaryPlayerAliases []string     `json:"primary_player_aliases,omitempty"`
	Paths                ToolPaths    `json:"paths"`
	Render               RenderConfig `json:"render"`
	Theme                ThemeConfig  `json:"theme"`
	Update               UpdateConfig `json:"update"`
}

// LoadStatus reports how Load obtained the returned Config.
type LoadStatus struct {
	CreatedDefault       bool
	RecoveredFromCorrupt bool
	QuarantinePath       string
}

// DefaultConfig builds a Config populated with Strikeye defaults for dataRoot.
func DefaultConfig(dataRoot string) Config {
	return Config{
		SchemaVersion: CurrentSchemaVersion,
		DataRoot:      filepath.Clean(dataRoot),
		DemoWatchDirs: []string{},
		Render: RenderConfig{
			Policy:                 PolicyManualOnly,
			Width:                  1920,
			Height:                 1080,
			FPS:                    60,
			Encoder:                "auto",
			PreBufferSec:           3.0,
			PostBufferSec:          2.0,
			KeyOverlayEnabled:      true,
			FrameMeldEnabled:       false,
			HighlightImportanceMin: 50.0,
		},
		Theme: ThemeConfig{
			Preset:         ThemePresetPureBlack,
			AccentColor:    "#22C55E",
			OverlayOpacity: 0.72,
			CardOpacity:    0.90,
		},
	}
}

// Normalize fills missing or invalid configuration values with safe defaults.
func (c *Config) Normalize(dataRoot string) {
	def := DefaultConfig(dataRoot)
	if c.SchemaVersion <= 0 {
		c.SchemaVersion = def.SchemaVersion
	}
	if c.DataRoot == "" {
		c.DataRoot = def.DataRoot
	} else {
		c.DataRoot = filepath.Clean(c.DataRoot)
	}
	if c.DemoWatchDirs == nil {
		c.DemoWatchDirs = []string{}
	}
	switch c.Render.Policy {
	case PolicySmartHighlights, PolicyFullPOV, PolicyManualOnly:
	default:
		c.Render.Policy = def.Render.Policy
	}
	if c.Render.Width <= 0 {
		c.Render.Width = def.Render.Width
	}
	if c.Render.Height <= 0 {
		c.Render.Height = def.Render.Height
	}
	if c.Render.FPS <= 0 {
		c.Render.FPS = def.Render.FPS
	}
	if c.Render.Encoder == "" {
		c.Render.Encoder = def.Render.Encoder
	}
	if c.Render.PreBufferSec < 0 {
		c.Render.PreBufferSec = def.Render.PreBufferSec
	}
	if c.Render.PostBufferSec < 0 {
		c.Render.PostBufferSec = def.Render.PostBufferSec
	}
	if c.Render.HighlightImportanceMin <= 0 {
		c.Render.HighlightImportanceMin = def.Render.HighlightImportanceMin
	}
	if strings.TrimSpace(c.Theme.Preset) == "" && strings.TrimSpace(c.Theme.AccentColor) == "" &&
		c.Theme.OverlayOpacity == 0 && c.Theme.CardOpacity == 0 && strings.TrimSpace(c.Theme.BackgroundImage) == "" {
		c.Theme = def.Theme
	} else {
		switch c.Theme.Preset {
		case ThemePresetEsportsDark, ThemePresetMidnightBlue, ThemePresetTacticalRed, ThemePresetLightClean,
			ThemePresetPureBlack, ThemePresetGradientAurora, ThemePresetGradientSunset:
		default:
			c.Theme.Preset = def.Theme.Preset
		}
		if !isValidHexColor(c.Theme.AccentColor) {
			c.Theme.AccentColor = def.Theme.AccentColor
		} else {
			c.Theme.AccentColor = strings.ToUpper(strings.TrimSpace(c.Theme.AccentColor))
		}
		c.Theme.BackgroundImage = strings.TrimSpace(c.Theme.BackgroundImage)
		if c.Theme.OverlayOpacity < 0 {
			c.Theme.OverlayOpacity = 0
		} else if c.Theme.OverlayOpacity > 0.95 {
			c.Theme.OverlayOpacity = 0.95
		}
		if c.Theme.CardOpacity <= 0 {
			c.Theme.CardOpacity = def.Theme.CardOpacity
		} else if c.Theme.CardOpacity < 0.4 {
			c.Theme.CardOpacity = 0.4
		} else if c.Theme.CardOpacity > 1.0 {
			c.Theme.CardOpacity = 1.0
		}
	}
}

func isValidHexColor(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return false
		}
	}
	return true
}

// ConfigPath returns the path to config.json inside dataRoot.
func ConfigPath(dataRoot string) string {
	return filepath.Join(filepath.Clean(dataRoot), "config.json")
}

// Load ensures DataRoot layout exists, reads config.json, quarantines corrupt JSON
// if encountered, and applies safe defaults for missing fields.
func Load(dataRoot string) (Config, LoadStatus, error) {
	if _, err := EnsureDataLayout(dataRoot); err != nil {
		return Config{}, LoadStatus{}, err
	}
	path := ConfigPath(dataRoot)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := DefaultConfig(dataRoot)
		if saveErr := Save(dataRoot, cfg); saveErr != nil {
			return Config{}, LoadStatus{}, saveErr
		}
		return cfg, LoadStatus{CreatedDefault: true}, nil
	}
	if err != nil {
		return Config{}, LoadStatus{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		quarantinePath, qErr := quarantineCorruptFile(path)
		if qErr != nil {
			return Config{}, LoadStatus{}, fmt.Errorf("quarantine corrupt config %q: %w (parse err: %v)", path, qErr, err)
		}
		cfg = DefaultConfig(dataRoot)
		if saveErr := Save(dataRoot, cfg); saveErr != nil {
			return Config{}, LoadStatus{}, saveErr
		}
		return cfg, LoadStatus{
			CreatedDefault:       true,
			RecoveredFromCorrupt: true,
			QuarantinePath:       quarantinePath,
		}, nil
	}

	cfg.Normalize(dataRoot)
	return cfg, LoadStatus{}, nil
}

// Save normalizes cfg and writes it atomically to <dataRoot>/config.json.
func Save(dataRoot string, cfg Config) error {
	if _, err := EnsureDataLayout(dataRoot); err != nil {
		return err
	}
	cfg.Normalize(dataRoot)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	target := ConfigPath(dataRoot)
	return writeFileAtomically(target, data)
}

func quarantineCorruptFile(path string) (string, error) {
	stamp := time.Now().UTC().Format("20060102T150405Z")
	base := fmt.Sprintf("%s.corrupt-%s", path, stamp)
	candidate := base + ".bak"
	for i := 1; ; i++ {
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			break
		}
		candidate = fmt.Sprintf("%s-%d.bak", base, i)
	}
	if err := os.Rename(path, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func writeFileAtomically(target string, data []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("replace config %q: %w", target, err)
	}
	cleanup = false
	return nil
}
