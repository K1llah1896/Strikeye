package settings

import (
	"os"
	"path/filepath"
	"testing"

	"strikeye/internal/config"
)

func TestApplyChoicesAndStorageMaintenance(t *testing.T) {
	cfg := config.DefaultConfig(t.TempDir())
	ApplyChoicesToConfig(&cfg, "2560x1440", "120", "donk, sh1ro", "/demos/a\n/demos/b")
	if cfg.Render.Width != 2560 || cfg.Render.Height != 1440 || cfg.Render.FPS != 120 {
		t.Fatalf("unexpected resolution/fps: %+v", cfg.Render)
	}
	if len(cfg.PrimaryPlayerAliases) != 2 || len(cfg.DemoWatchDirs) != 2 {
		t.Fatalf("unexpected aliases/watchdirs: %+v %+v", cfg.PrimaryPlayerAliases, cfg.DemoWatchDirs)
	}

	tmp := t.TempDir()
	layout, _ := config.ResolveDataLayout(tmp)
	cacheDir := filepath.Join(layout.Root, "cache")
	_ = os.MkdirAll(cacheDir, 0o755)
	_ = os.WriteFile(filepath.Join(cacheDir, "_strikeye_playback_1.dem"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(cacheDir, "keep.rpl"), []byte("y"), 0o644)

	removed, err := CleanInvalidCache(tmp)
	if err != nil || removed != 1 {
		t.Fatalf("expected 1 removed temp cache file, got removed=%d err=%v", removed, err)
	}
}
