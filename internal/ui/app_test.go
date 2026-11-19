package ui

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"strikeye/internal/config"
	"strikeye/internal/ipc"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
)

func TestAppNavigationAndEvents(t *testing.T) {
	tmp := t.TempDir()
	db, err := storage.OpenDefault(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	exited := false
	app := NewApp(AppOptions{
		DataRoot:  tmp,
		DB:        db,
		OnExitNow: func() { exited = true },
	})
	defer app.Close()

	if app.ActiveView() != ViewLibrary {
		t.Fatalf("expected initial ViewLibrary, got %s", app.ActiveView())
	}
	app.NavigateTo(ViewAnalysis, 0)
	if app.ActiveView() != ViewAnalysis {
		t.Fatalf("expected ViewAnalysis after navigate, got %s", app.ActiveView())
	}

	raw, _ := json.Marshal(map[string]any{"task_count": 4})
	app.ApplyAgentEvent(ipc.EventNotification{Topic: "render.planned", Params: raw})
	if app.queuedCount != 4 || app.agentStatusText != "待渲染" {
		t.Fatalf("unexpected agent status after event: %s (%d)", app.agentStatusText, app.queuedCount)
	}

	if err := app.ConfirmStartRenderNow(); err != nil {
		t.Fatalf("ConfirmStartRenderNow: %v", err)
	}
	if !exited {
		t.Fatalf("expected OnExitNow callback to fire")
	}
}

func TestApplyAgentEventParseProgress(t *testing.T) {
	tmp := t.TempDir()
	db, err := storage.OpenDefault(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	app := NewApp(AppOptions{DataRoot: tmp, DB: db})
	defer app.Close()

	startRaw, _ := json.Marshal(map[string]any{"demo_id": 7})
	app.ApplyAgentEvent(ipc.EventNotification{Topic: "parse.started", Params: startRaw})
	if !app.parseActive || app.parseProgress >= 0 {
		t.Fatalf("expected indeterminate parseActive after parse.started, got active=%v prog=%v", app.parseActive, app.parseProgress)
	}

	progRaw, _ := json.Marshal(map[string]any{"demo_id": 7, "stage": "扫描 Demo 帧数据", "progress": 0.62})
	app.ApplyAgentEvent(ipc.EventNotification{Topic: "parse.progress", Params: progRaw})
	if !app.parseActive || app.parseProgress != 0.62 || app.parseStage != "扫描 Demo 帧数据" {
		t.Fatalf("unexpected parse progress state: active=%v prog=%v stage=%q", app.parseActive, app.parseProgress, app.parseStage)
	}

	doneRaw, _ := json.Marshal(map[string]any{"demo_id": 7})
	app.ApplyAgentEvent(ipc.EventNotification{Topic: "parse.completed", Params: doneRaw})
	if app.parseActive || app.agentStatusText != "解析完成" {
		t.Fatalf("expected parseActive cleared after parse.completed, got active=%v status=%q", app.parseActive, app.agentStatusText)
	}
}

func TestBackgroundImageRendersBehindUIContent(t *testing.T) {
	defer theme.Apply(config.DefaultConfig(t.TempDir()).Theme)

	tmp := t.TempDir()
	db, err := storage.OpenDefault(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	bgPath := filepath.Join(tmp, "bg.png")
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	f, err := os.Create(bgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	cfg := config.DefaultConfig(tmp)
	cfg.Theme.BackgroundImage = bgPath
	cfg.Theme.OverlayOpacity = 0.6
	cfg.Theme.CardOpacity = 0.9
	if err := config.Save(tmp, cfg); err != nil {
		t.Fatal(err)
	}

	app := NewApp(AppOptions{DataRoot: tmp, DB: db})
	defer app.Close()

	tt := mygoui.NewTester(app.Render, 1360, 840)
	tt.Frame()
	if !tt.HasText("STRIKEYE") {
		t.Fatalf("expected STRIKEYE header text to be rendered")
	}
	// Clicking the "任务与设置" item in the nav rail should hit-test above the background image.
	if err := tt.Click("任务与设置"); err != nil {
		t.Fatalf("Click 任务与设置 failed: %v", err)
	}
	tt.Frame()
	if app.ActiveView() != ViewSettings {
		t.Fatalf("expected ViewSettings after clicking 任务与设置 over background image, got %s", app.ActiveView())
	}
}

func TestTopRightNotificationsAndGitHub(t *testing.T) {
	tmp := t.TempDir()
	db, err := storage.OpenDefault(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	app := NewApp(AppOptions{DataRoot: tmp, DB: db})
	defer app.Close()
	app.AddNotification("Demo 解析完成", "de_mirage 已就绪", theme.StatusGreen)
	if app.UnreadNotificationCount() != 1 {
		t.Fatalf("expected 1 unread notification, got %d", app.UnreadNotificationCount())
	}

	tt := mygoui.NewTester(app.Render, 1360, 840)
	tt.Frame()

	// Click notification center button via accessibility label
	if err := tt.Click("通知中心"); err != nil {
		t.Fatalf("click 通知中心 failed: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Demo 解析完成") {
		t.Fatalf("expected notification item text inside popover")
	}
	if err := tt.Click("全部已读"); err != nil {
		t.Fatalf("click 全部已读 failed: %v", err)
	}
	tt.Frame()
	if app.UnreadNotificationCount() != 0 {
		t.Fatalf("expected 0 unread notifications after 全部已读, got %d", app.UnreadNotificationCount())
	}

	// Click GitHub icon button
	if err := tt.Click("GitHub 仓库"); err != nil {
		t.Fatalf("click GitHub 仓库 failed: %v", err)
	}
	tt.Frame()
	opened := tt.OpenedURLs()
	if len(opened) == 0 || opened[len(opened)-1] != strikeyeGitHubURL {
		t.Fatalf("expected %s in OpenedURLs, got %+v", strikeyeGitHubURL, opened)
	}

	// Clicking bottom-left render queue telemetry navigates to ViewSettings
	if err := tt.Click("查看队列 ›"); err != nil {
		t.Fatalf("click 查看队列 › failed: %v", err)
	}
	tt.Frame()
	if app.ActiveView() != ViewSettings {
		t.Fatalf("expected ViewSettings after clicking render queue widget, got %s", app.ActiveView())
	}
}
