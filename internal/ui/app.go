// Package ui implements Strikeye's MyGo Native UI application shell,
// left navigation rail, live agent status bar, and render-now handoff modal.
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"strikeye/internal/config"
	"strikeye/internal/ipc"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
	"strikeye/internal/ui/views/analysis"
	"strikeye/internal/ui/views/highlights"
	"strikeye/internal/ui/views/library"
	"strikeye/internal/ui/views/settings"
	"strikeye/internal/ui/views/spectator"
	"strikeye/internal/updater"
	"strikeye/internal/version"
)

// ViewID identifies one of Strikeye's 5 primary workspace views.
type ViewID string

const (
	ViewLibrary    ViewID = "library"
	ViewSpectator  ViewID = "spectator"
	ViewAnalysis   ViewID = "analysis"
	ViewHighlights ViewID = "highlights"
	ViewSettings   ViewID = "settings"
)

// AppOptions configures the main Strikeye MyGo Native UI shell.
type AppOptions struct {
	DataRoot   string
	DB         *storage.DB
	Agent      *ipc.Client
	OnExitNow  func()
}

// App manages the top-level MyGo Native UI window, navigation, and live agent events.
type App struct {
	mu               sync.RWMutex
	opts             AppOptions
	activeView       ViewID
	agentStatusText  string
	parseActive      bool
	parseProgress    float64
	parseStage       string
	navLoading       bool
	navProgress      float64
	navStage         string
	navSeq           uint64
	queuedCount      int
	availUpdateVer   string
	showRenderModal  bool
	libraryView      *library.View
	spectatorView    *spectator.View
	analysisView     *analysis.View
	highlightsView   *highlights.View
	settingsView     *settings.View
	win              *mygo.Window
}

// NewApp constructs the MyGo Native UI application shell and wires all 5 views.
func NewApp(opts AppOptions) *App {
	cfg, _, _ := config.Load(opts.DataRoot)
	theme.Apply(cfg.Theme)
	probe := config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths})

	var caller library.AgentCaller
	if opts.Agent != nil {
		caller = opts.Agent
	}

	app := &App{
		opts:            opts,
		activeView:      ViewLibrary,
		agentStatusText: "空闲",
	}

	triggerRenderModal := func() {
		app.mu.Lock()
		app.showRenderModal = true
		app.mu.Unlock()
	}

	app.libraryView = library.NewView(opts.DB, caller, opts.DataRoot, func(target library.NavTarget, demoID int64) {
		app.NavigateTo(ViewID(target), demoID)
	})
	app.spectatorView = spectator.NewView(opts.DB, caller, probe.FFmpeg.Path, triggerRenderModal)
	app.analysisView = analysis.NewView(opts.DB, opts.DataRoot, probe.FFmpeg.Path)
	app.analysisView.SetInvalidate(app.invalidateWindow)
	app.highlightsView = highlights.NewView(opts.DB, caller, probe.FFmpeg.Path, triggerRenderModal)
	app.settingsView = settings.NewView(opts.DB, caller, opts.DataRoot)
	app.settingsView.SetCallbacks(app.invalidateWindow, app.RestartForUpdate)

	if cfg.Update.AutoCheck {
		go func() {
			if res, err := updater.CheckUpdate(context.Background(), nil, updater.DefaultGitHubReleasesURL, version.AppVersion); err == nil && res.HasUpdate {
				app.mu.Lock()
				app.availUpdateVer = res.LatestVersion
				app.mu.Unlock()
				app.invalidateWindow()
			}
		}()
	}

	_ = app.libraryView.Refresh(context.Background())
	return app
}

func (a *App) invalidateWindow() {
	a.mu.RLock()
	win := a.win
	a.mu.RUnlock()
	if win != nil {
		win.Invalidate()
	}
}

// NavigateTo switches the active workspace view and loads demoID asynchronously when applicable.
func (a *App) NavigateTo(view ViewID, demoID int64) {
	a.mu.Lock()
	a.activeView = view
	a.navSeq++
	seq := a.navSeq
	needsAsync := (view == ViewSpectator && demoID > 0) || (view == ViewAnalysis && demoID > 0) || view == ViewHighlights
	if needsAsync {
		a.navLoading = true
		a.navProgress = -1
		switch view {
		case ViewAnalysis:
			a.navStage = "正在加载比赛分析数据与 2D 雷达索引..."
		case ViewSpectator:
			a.navStage = "正在加载观战视角与事件时间轴..."
		case ViewHighlights:
			a.navStage = "正在加载精彩片段库..."
		}
	} else {
		a.navLoading = false
		a.navProgress = 0
		a.navStage = ""
	}
	a.mu.Unlock()
	a.invalidateWindow()

	ctx := context.Background()
	if !needsAsync {
		switch view {
		case ViewLibrary:
			_ = a.libraryView.Refresh(ctx)
		case ViewSettings:
			_ = a.settingsView.RefreshTasks(ctx)
		}
		a.invalidateWindow()
		return
	}

	go func(target ViewID, id int64, mySeq uint64) {
		switch target {
		case ViewSpectator:
			_ = a.spectatorView.SelectDemo(ctx, id)
		case ViewAnalysis:
			_ = a.analysisView.SelectDemo(ctx, id)
		case ViewHighlights:
			_ = a.highlightsView.LoadDemo(ctx, id)
		}
		a.mu.Lock()
		if a.navSeq == mySeq {
			a.navLoading = false
			a.navProgress = 1
			a.navStage = ""
		}
		a.mu.Unlock()
		a.invalidateWindow()
	}(view, demoID, seq)
}

// ActiveView returns the currently selected ViewID.
func (a *App) ActiveView() ViewID {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activeView
}

// ApplyAgentEvent updates the top status bar and view progress from an IPC event.
func (a *App) ApplyAgentEvent(ev ipc.EventNotification) {
	var payload map[string]any
	_ = json.Unmarshal(ev.Params, &payload)

	a.mu.Lock()
	demoID := int64(0)
	if id, ok := payload["demo_id"].(float64); ok {
		demoID = int64(id)
	}
	switch ev.Topic {
	case "parse.started":
		a.agentStatusText = "解析中"
		a.parseActive = true
		a.parseProgress = -1
		a.parseStage = "准备解析 Demo..."
		if a.libraryView != nil {
			a.libraryView.SetParseProgress(demoID, a.parseStage, -1)
		}
	case "parse.progress":
		p, _ := payload["progress"].(float64)
		stage, _ := payload["stage"].(string)
		if stage == "" {
			stage = "正在解析 Demo..."
		}
		a.parseActive = true
		a.parseProgress = p
		a.parseStage = stage
		a.agentStatusText = fmt.Sprintf("解析中 %.0f%%", p*100)
		if a.libraryView != nil {
			a.libraryView.SetParseProgress(demoID, stage, p)
		}
	case "parse.completed":
		a.agentStatusText = "解析完成"
		a.parseActive = false
		a.parseProgress = 0
		a.parseStage = ""
		if a.libraryView != nil {
			a.libraryView.ClearParseProgress(demoID)
		}
	case "parse.failed":
		a.agentStatusText = "解析异常"
		a.parseActive = false
		a.parseProgress = 0
		a.parseStage = ""
		if a.libraryView != nil {
			a.libraryView.ClearParseProgress(demoID)
		}
	case "render.planned":
		if n, ok := payload["task_count"].(float64); ok {
			a.queuedCount += int(n)
		}
		a.agentStatusText = "待渲染"
	case "render.cancelled", "render.retried":
		a.agentStatusText = "队列已更新"
	case "montage.progress":
		p, _ := payload["progress"].(float64)
		a.highlightsView.SetExportProgress("running", p, "")
	case "montage.completed":
		a.highlightsView.SetExportProgress("completed", 1.0, "合辑导出完成")
	case "montage.failed":
		errMsg, _ := payload["error"].(string)
		a.highlightsView.SetExportProgress("failed", 0, "合辑导出失败: "+errMsg)
	}
	a.mu.Unlock()

	if ev.Topic == "parse.completed" || ev.Topic == "parse.failed" || ev.Topic == "demo.discovered" {
		_ = a.libraryView.Refresh(context.Background())
	}
	a.mu.RLock()
	win := a.win
	a.mu.RUnlock()
	if win != nil {
		win.Invalidate()
	}
}

// ConfirmStartRenderNow triggers render.start_now via IPC and exits the UI process cleanly.
func (a *App) ConfirmStartRenderNow() error {
	if a.opts.Agent != nil {
		var res map[string]any
		if err := a.opts.Agent.Call(context.Background(), "render.start_now", nil, &res); err != nil {
			return err
		}
	}
	a.Close()
	if a.opts.OnExitNow != nil {
		a.opts.OnExitNow()
	} else {
		os.Exit(0)
	}
	return nil
}

// RestartForUpdate closes the current UI session, launches the newly replaced
// executable, and exits the current process.
func (a *App) RestartForUpdate() {
	a.Close()
	_ = updater.RelaunchCurrentExecutable()
	if a.opts.OnExitNow != nil {
		a.opts.OnExitNow()
	} else {
		os.Exit(0)
	}
}

// Close releases view resources (such as active video decoders).
func (a *App) Close() {
	if a.spectatorView != nil {
		a.spectatorView.Close()
	}
	if a.highlightsView != nil {
		a.highlightsView.Close()
	}
}

// Run starts the MyGo Native UI event loop and blocks until the window closes.
func (a *App) Run(ctx context.Context) error {
	theme.RegisterFonts()

	if a.opts.Agent != nil {
		_ = a.opts.Agent.Subscribe(ctx)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case ev, ok := <-a.opts.Agent.Events():
					if !ok {
						return
					}
					a.ApplyAgentEvent(ev)
				}
			}
		}()
	}

	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:          "Strikeye — CS2 Demo 观战与分析工作台",
			Width:          1360,
			Height:         840,
			MinWidth:       1080,
			MinHeight:      700,
			TitleBarStyle:  mygo.TitleBarHidden,
			TitleBarHeight: 54,
			Vibrancy:       mygo.VibrancySidebar,
			Content:        mygoui.View(a.Render),
		})
	})

	err := mygo.App.Run()
	a.Close()
	return err
}

// Render builds the top-level MyGo Native UI frame for the main window.
func (a *App) Render(c *mygoui.Context) {
	a.mu.RLock()
	active := a.activeView
	statusText := a.agentStatusText
	parseActive := a.parseActive
	parseProg := a.parseProgress
	parseStage := a.parseStage
	navLoading := a.navLoading
	navProg := a.navProgress
	navStage := a.navStage
	queued := a.queuedCount
	availUpdate := a.availUpdateVer
	showModal := a.showRenderModal
	a.mu.RUnlock()

	bmp, overlayCol := theme.BackgroundBitmap()
	grad, hasGrad := theme.BackgroundGradient()
	hasBackdrop := bmp != nil || hasGrad
	glassActive := hasBackdrop || c.Vibrancy()
	shellBg := theme.BgBase
	panelBg := theme.BgSurface
	if hasBackdrop {
		shellBg = mygoui.Transparent
	} else if c.Vibrancy() {
		shellBg = mygoui.Transparent
		panelBg = theme.GlassSurface(theme.CardOpacity())
	}

	mygoui.Themed(c, theme.ActiveTheme()).Fill().Children(func() {
		if bmp != nil {
			mygoui.Box(c).Fill().IgnoreInput(true).Children(func() {
				mygoui.Image(c, bmp).Fill().Fit(mygoui.Cover)
				mygoui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Background(overlayCol)
			})
		} else if hasGrad {
			mygoui.Box(c).Fill().IgnoreInput(true).LinearGradient(grad)
		}
		shellCol := mygoui.Column(c).Fill().Background(shellBg)
		if hasBackdrop {
			shellCol = mygoui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Background(shellBg)
		}
		shellCol.Children(func() {
			// Top Header Bar (flush with top edge so OS window controls align cleanly at y=0)
			tb := c.TitleBar()
			headerH := float32(54)
			if tb.Height > headerH {
				headerH = tb.Height
			}
			headerBar := theme.RowCenter(c).FillWidth().Height(headerH).
				Background(panelBg).Border(1, theme.BorderSubtle).
				Padding(0, tb.Right+16, 0, tb.Left+16).Gap(12).
				DragWindow()
			if glassActive {
				headerBar = headerBar.Effect(theme.GlassEffect())
			}
			headerBar.Children(func() {
				mygoui.Row(c).AlignItems(mygoui.Center).Gap(8).Children(func() {
					mygoui.Box(c).Width(4).Height(18).Radius(2).Background(theme.AccentAmber)
					mygoui.Text(c, "STRIKEYE").Font("Rajdhani").FontSize(20).Bold().LetterSpacing(1.4).TextColor(theme.AccentAmber)
				})
				mygoui.Text(c, "SEE THE ROUND. OWN THE MOMENT.").Font("Rajdhani").FontSize(11).LetterSpacing(1.2).TextColor(theme.TextMuted)
				mygoui.Spacer(c)
				if parseActive {
					mygoui.Box(c).Width(200).Children(func() {
						theme.ProgressBar(c, parseProg, parseStage)
					})
				}
				theme.Capsule(c, fmt.Sprintf("后台状态: %s | 待渲染: %d", statusText, queued), theme.AccentCyan)
				if queued > 0 {
					if theme.AccentButton(c, "立即启动渲染").Clicked() {
						a.mu.Lock()
						a.showRenderModal = true
						a.mu.Unlock()
					}
				}
				if availUpdate != "" {
					if theme.AccentButton(c, "发现新版本 "+availUpdate).Clicked() {
						a.NavigateTo(ViewSettings, 0)
					}
				}
				if theme.ActionButton(c, "设置").Clicked() {
					a.NavigateTo(ViewSettings, 0)
				}
			})

			// Main Split: Left Nav Rail + Workspace
			mygoui.Row(c).FillWidth().Grow(1).MinHeight(0).Padding(10).Gap(10).Children(func() {
				navRail := mygoui.Column(c).Width(196).FillHeight().Background(panelBg).Border(1, theme.BorderSubtle).Radius(14).Padding(12).Gap(6)
				if glassActive {
					navRail = navRail.Effect(theme.GlassEffect())
				}
				navRail.Children(func() {
					navItems := []struct {
						id    ViewID
						title string
						sub   string
					}{
						{ViewLibrary, "比赛资料库", "MATCH LIBRARY"},
						{ViewSpectator, "独立观战", "SPECTATOR"},
						{ViewAnalysis, "比赛分析", "ANALYSIS"},
						{ViewHighlights, "精彩片段", "HIGHLIGHTS"},
						{ViewSettings, "任务与设置", "SYSTEM & TASKS"},
					}
					for _, item := range navItems {
						if theme.NavButton(c, item.title, item.sub, active == item.id).Clicked() {
							a.NavigateTo(item.id, 0)
						}
					}
				})

				mygoui.Column(c).FillHeight().Grow(1).MinWidth(0).MinHeight(0).Children(func() {
					if navLoading {
						mygoui.Column(c).Fill().Center().Padding(32).Children(func() {
							mygoui.Box(c).Width(460).Children(func() {
								theme.Card(c, func() {
									theme.SectionTitle(c, "正在准备视图")
									theme.ProgressBar(c, navProg, navStage)
								})
							})
						})
						return
					}
					switch active {
					case ViewLibrary:
						a.libraryView.Render(c)
					case ViewSpectator:
						a.spectatorView.Render(c)
					case ViewAnalysis:
						a.analysisView.Render(c)
					case ViewHighlights:
						a.highlightsView.Render(c)
					case ViewSettings:
						a.settingsView.Render(c)
					}
				})
			})
		})

		modalOpen := showModal
		if mygoui.Modal(c, &modalOpen, func() {
			a.renderConfirmModal(c)
		}).Dismissed() {
			a.mu.Lock()
			a.showRenderModal = false
			a.mu.Unlock()
		}
	})
}

func (a *App) renderConfirmModal(c *mygoui.Context) {
	modal := mygoui.Column(c).Width(460).Background(theme.BgSurface).Border(1, theme.BorderSubtle).Radius(16).Padding(20).Gap(14)
	bmp, _ := theme.BackgroundBitmap()
	_, hasGrad := theme.BackgroundGradient()
	if bmp != nil || hasGrad || c.Vibrancy() {
		modal = modal.Effect(theme.GlassEffect())
	}
	modal.Children(func() {
		theme.RowCenter(c).Gap(8).Children(func() {
			theme.StatusDot(c, theme.AccentAmber, 8)
			theme.SectionTitle(c, "确认启动 CS2 后台渲染")
		})
		mygoui.Text(c, "即将启动 CS2 原生引擎进行视频渲染。为保证渲染稳定性，Strikeye 主界面将自动关闭，渲染完成后将通过系统通知提醒您。").
			FontSize(13).LineHeight(1.45).TextColor(theme.TextPrimary)
		theme.RowCenter(c).Gap(10).Children(func() {
			mygoui.Spacer(c)
			if theme.ActionButton(c, "取消").Clicked() {
				a.mu.Lock()
				a.showRenderModal = false
				a.mu.Unlock()
			}
			if theme.AccentButton(c, "确认并退出主界面").Clicked() {
				_ = a.ConfirmStartRenderNow()
			}
		})
	})
}
