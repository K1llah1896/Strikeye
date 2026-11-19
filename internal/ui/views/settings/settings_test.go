package settings

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
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

func TestSaveWatchDirsButtonPersistsConfig(t *testing.T) {
	tmp := t.TempDir()
	v := NewView(nil, nil, tmp)
	v.watchDirsTx = "\"D:\\CS2\\replays\"\r\nE:\\FaceitDemos\r\n"

	tt := ui.NewTester(v.Render, 1100, 800)
	tt.Frame()
	if err := tt.Click("保存监听目录"); err != nil {
		t.Fatalf("Click 保存监听目录 failed: %v", err)
	}
	tt.Frame()

	loaded, _, err := config.Load(tmp)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if len(loaded.DemoWatchDirs) != 2 || loaded.DemoWatchDirs[0] != `D:\CS2\replays` || loaded.DemoWatchDirs[1] != `E:\FaceitDemos` {
		t.Fatalf("unexpected persisted DemoWatchDirs: %+v", loaded.DemoWatchDirs)
	}
	_ = v.Reload(context.Background())
	if v.watchDirsTx == "" {
		t.Fatalf("expected watchDirsTx restored on Reload")
	}
}

func TestStatusBannerDismissesOnClick(t *testing.T) {
	v := NewView(nil, nil, t.TempDir())
	v.setStatus("配置已保存")

	tt := ui.NewTester(v.Render, 1100, 800)
	tt.Frame()
	if !tt.HasText("配置已保存") {
		t.Fatalf("expected status banner to be visible")
	}
	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("click 关闭 failed: %v", err)
	}
	tt.Frame()
	if tt.HasText("配置已保存") {
		t.Fatalf("expected status banner to disappear after clicking 关闭")
	}
}
