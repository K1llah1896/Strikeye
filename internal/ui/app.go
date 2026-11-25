// Package ui implements Strikeye's MyGo Native UI application shell,
// left navigation rail, live agent status bar, and render-now handoff modal.
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

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

const strikeyeGitHubURL = "https://github.com/K1llah1896/Strikeye"

// NotificationItem represents an in-app notification shown in the top-right Notification Center.
type NotificationItem struct {
	Title  string
	Detail string
	Time   string
	Accent mygoui.Color
	Unread bool
}

// App manages the top-level MyGo Native UI window, navigation, and live agent events.
type App struct {
	mu                sync.RWMutex
	opts              AppOptions
	activeView        ViewID
	agentStatusText   string
	parseActive       bool
	parseProgress     float64
	parseStage        string
	navLoading        bool
	navProgress       float64
	navStage          string
	navSeq            uint64
	selectedDemoID    int64
	queuedCount       int
	availUpdateVer    string
	showRenderModal   bool
	renderNowExit     bool
	showNotifications bool
	notifications     []NotificationItem
	libraryView       *library.View
	spectatorView     *spectator.View
	analysisView      *analysis.View
	highlightsView    *highlights.View
	settingsView      *settings.View
	win               *mygo.Window
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
	app.settingsView.SetTasksChanged(func() {
		app.refreshQueuedCount()
		app.invalidateWindow()
	})
	app.refreshQueuedCount()

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

// AddNotification records a notification item for the in-app Notification Center.
func (a *App) AddNotification(title, detail string, accent mygoui.Color) {
	a.mu.Lock()
	a.appendNotificationLocked(title, detail, accent)
	a.mu.Unlock()
}

func (a *App) appendNotificationLocked(title, detail string, accent mygoui.Color) {
	item := NotificationItem{
		Title:  title,
		Detail: detail,
		Time:   time.Now().Format("15:04"),
		Accent: accent,
		Unread: true,
	}
	a.notifications = append([]NotificationItem{item}, a.notifications...)
	if len(a.notifications) > 30 {
		a.notifications = a.notifications[:30]
	}
}

// UnreadNotificationCount returns the number of unread notifications.
func (a *App) UnreadNotificationCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	n := 0
	for _, it := range a.notifications {
		if it.Unread {
			n++
		}
	}
	return n
}

func (a *App) refreshQueuedCount() {
	if a.opts.DB == nil {
		return
	}
	ctx := context.Background()
	queued, err1 := a.opts.DB.RenderTasks().ListByStatus(ctx, "queued")
	running, err2 := a.opts.DB.RenderTasks().ListByStatus(ctx, "running")
	if err1 != nil && err2 != nil {
		return
	}
	a.mu.Lock()
	a.queuedCount = len(queued) + len(running)
	a.mu.Unlock()
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
	prevView := a.activeView
	a.activeView = view
	if demoID > 0 {
		a.selectedDemoID = demoID
	} else if view == ViewHighlights && a.selectedDemoID > 0 {
		demoID = a.selectedDemoID
	}
	a.navSeq++
	seq := a.navSeq
	needsAsync := (view == ViewSpectator && demoID > 0) || (view == ViewAnalysis && demoID > 0) || (view == ViewHighlights && demoID > 0)
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
	if prevView == ViewAnalysis && view != ViewAnalysis && a.analysisView != nil {
		a.analysisView.Close()
	}
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
		a.appendNotificationLocked("Demo 解析完成", fmt.Sprintf("Demo #%d 已就绪，可进入比赛分析与观战。", demoID), theme.StatusGreen)
		if a.libraryView != nil {
			a.libraryView.ClearParseProgress(demoID)
		}
	case "parse.failed":
		a.agentStatusText = "解析异常"
		a.parseActive = false
		a.parseProgress = 0
		a.parseStage = ""
		a.appendNotificationLocked("Demo 解析失败", fmt.Sprintf("Demo #%d 解析异常，请检查文件完整性。", demoID), theme.StatusRed)
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
		a.appendNotificationLocked("合辑导出完成", "视频已合成并注册至本地媒体索引。", theme.StatusGreen)
	case "montage.failed":
		errMsg, _ := payload["error"].(string)
		a.highlightsView.SetExportProgress("failed", 0, "合辑导出失败: "+errMsg)
		a.appendNotificationLocked("合辑导出失败", errMsg, theme.StatusRed)
	}
	a.mu.Unlock()

	if ev.Topic == "parse.completed" || ev.Topic == "parse.failed" || ev.Topic == "demo.discovered" {
		_ = a.libraryView.Refresh(context.Background())
	}
	if ev.Topic == "render.cancelled" || ev.Topic == "render.retried" || ev.Topic == "render.completed" || ev.Topic == "render.failed" {
		a.refreshQueuedCount()
	}
	if ev.Topic == "render.planned" && a.opts.DB != nil {
		ctx := context.Background()
		queued, err1 := a.opts.DB.RenderTasks().ListByStatus(ctx, "queued")
		running, err2 := a.opts.DB.RenderTasks().ListByStatus(ctx, "running")
		if err1 == nil && err2 == nil && len(queued)+len(running) > 0 {
			a.mu.Lock()
			a.queuedCount = len(queued) + len(running)
			a.mu.Unlock()
		}
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
	a.mu.Lock()
	a.renderNowExit = true
	a.mu.Unlock()
	a.Close()
	if a.opts.OnExitNow != nil {
		a.opts.OnExitNow()
	} else {
		os.Exit(0)
	}
	return nil
}

// ShouldKeepAgentOnExit reports whether the UI exited intentionally for background rendering.
func (a *App) ShouldKeepAgentOnExit() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.renderNowExit
}

// RestartForUpdate closes the current UI session, stops the background agent,
// launches the newly replaced executable, and exits the current process.
func (a *App) RestartForUpdate() {
	if a.opts.Agent != nil {
		sctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_ = a.opts.Agent.Call(sctx, "agent.shutdown", nil, nil)
		cancel()
	}
	a.Close()
	_ = updater.RelaunchCurrentExecutable()
	if a.opts.OnExitNow != nil {
		a.opts.OnExitNow()
	} else {
		os.Exit(0)
	}
}

// Close releases view resources (such as active video decoders and replay tickers).
func (a *App) Close() {
	if a.analysisView != nil {
		a.analysisView.Close()
	}
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
		win := mygo.NewWindow(mygo.WindowOptions{
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
		a.mu.Lock()
		a.win = win
		a.mu.Unlock()
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
	showNotif := a.showNotifications
	notifs := append([]NotificationItem(nil), a.notifications...)
	unreadNotifs := 0
	for _, it := range notifs {
		if it.Unread {
			unreadNotifs++
		}
	}
	a.mu.RUnlock()

	bmp, _ := theme.BackgroundBitmap()
	_, hasGrad := theme.BackgroundGradient()
	hasBackdrop := bmp != nil || hasGrad
	shellBg := theme.BgBase
	panelBg := theme.BgSurface
	if hasBackdrop {
		shellBg = mygoui.Transparent
	} else if c.Vibrancy() {
		shellBg = mygoui.Transparent
		panelBg = theme.GlassSurface(theme.CardOpacity())
	}

	mygoui.Themed(c, theme.ActiveTheme()).Fill().Children(func() {
		theme.AmbientBackdrop(c)
		shellCol := mygoui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Background(shellBg)
		shellCol.Children(func() {
			// Seamless borderless top header bar (drag-enabled, flush with OS window controls)
			tb := c.TitleBar()
			headerH := float32(60)
			if tb.Height > headerH {
				headerH = tb.Height
			}
			headerBar := theme.RowCenter(c).FillWidth().Height(headerH).
				Background(mygoui.RGBA(5, 7, 11, 0.42)).
				Padding(0, tb.Right+18, 0, tb.Left+18).Gap(12).
				DragWindow()
			headerBar.Children(func() {
				mygoui.Row(c).AlignItems(mygoui.Center).Gap(10).Children(func() {
					mygoui.Box(c).Width(32).Height(32).Radius(10).
						Background(mygoui.RGBA(56, 189, 248, 0.16)).
						Border(1, mygoui.RGBA(56, 189, 248, 0.35)).
						Center().
						Children(func() {
							theme.RenderIcon(c, theme.IconRadar, 17, theme.AccentAmber)
						})
					mygoui.Text(c, "STRIKEYE").Font("Rajdhani").FontSize(26).Bold().LetterSpacing(1.6).TextColor(theme.TextPrimary)
				})
				mygoui.Text(c, "SEE THE ROUND. OWN THE MOMENT.").Font("Rajdhani").FontSize(12).LetterSpacing(1.4).TextColor(theme.TextMuted)
				mygoui.Spacer(c)
				if parseActive {
					mygoui.Box(c).Width(210).
						Transition(mygoui.ElementTransition{
							Duration:     220 * time.Millisecond,
							Ease:         mygoui.EaseOut,
							ExitDuration: 140 * time.Millisecond,
							Enter:        &mygoui.Motion{Y: -8, Opacity: 0},
							Exit:         &mygoui.Motion{Y: -8, Opacity: 0},
						}).
						Children(func() {
							theme.ProgressBar(c, parseProg, parseStage)
						})
				}
				if queued > 0 {
					if theme.IconAccentButton(c, theme.IconBolt, "立即启动渲染").Clicked() {
						a.mu.Lock()
						a.showRenderModal = true
						a.mu.Unlock()
					}
				}
				if availUpdate != "" {
					if theme.IconAccentButton(c, theme.IconRefresh, "新版本 "+availUpdate).Clicked() {
						a.NavigateTo(ViewSettings, 0)
					}
				}
				bellBtn := theme.IconButton(c, theme.IconBell, unreadNotifs > 0).Tooltip("通知中心").Label("通知中心")
				if bellBtn.Clicked() {
					a.mu.Lock()
					a.showNotifications = !a.showNotifications
					a.mu.Unlock()
					showNotif = !showNotif
				}
				if theme.IconButton(c, theme.IconGitHub, false).Tooltip("GitHub 仓库").Label("GitHub 仓库").Clicked() {
					c.OpenURL(strikeyeGitHubURL)
				}
				notifOpen := showNotif
				if mygoui.Popover(c, bellBtn, &notifOpen, func() {
					a.renderNotificationsPopover(c, notifs, unreadNotifs)
				}).Dismissed() {
					a.mu.Lock()
					a.showNotifications = false
					a.mu.Unlock()
				}
			})

			// Main Split: Floating Glass Nav Rail + Workspace
			mygoui.Row(c).FillWidth().Grow(1).MinHeight(0).Padding(12).Gap(14).Children(func() {
				navRail := mygoui.Column(c).Width(208).FillHeight().
					Background(panelBg).
					Border(1, theme.RimBorder).
					Radius(20).
					Padding(12).
					Gap(6).
					Shadow(0, 12, 32, -6, mygoui.RGBA(0, 0, 0, 0.45))
				navRail.Children(func() {
					mygoui.Text(c, "WORKSPACE").Font("Rajdhani").FontSize(10).Bold().LetterSpacing(1.4).TextColor(theme.TextMuted).PaddingX(8).PaddingY(4)
					navItems := []struct {
						id    ViewID
						icon  *mygoui.SVG
						title string
						sub   string
					}{
						{ViewLibrary, theme.IconLibrary, "比赛资料库", "MATCH LIBRARY"},
						{ViewSpectator, theme.IconSpectator, "独立观战", "SPECTATOR"},
						{ViewAnalysis, theme.IconAnalysis, "比赛分析", "ANALYSIS"},
						{ViewHighlights, theme.IconHighlights, "精彩片段", "HIGHLIGHTS"},
						{ViewSettings, theme.IconSettings, "任务与设置", "SYSTEM & TASKS"},
					}
					for _, item := range navItems {
						if theme.NavButtonWithIcon(c, item.icon, item.title, item.sub, active == item.id).Clicked() {
							a.NavigateTo(item.id, 0)
						}
					}
					mygoui.Spacer(c)
					queueBtn := mygoui.ButtonBase(c).
						FillWidth().
						Radius(14).
						Background(theme.BgInput).
						Border(1, theme.RimBorder).
						Padding(10).
						Cursor(mygoui.CursorPointer).
						Children(func() {
							mygoui.Column(c).FillWidth().Gap(4).Children(func() {
								theme.RowCenter(c).FillWidth().Gap(6).Children(func() {
									theme.StatusDot(c, theme.StatusGreen, 6)
									mygoui.Text(c, "AGENT TELEMETRY").Font("Rajdhani").FontSize(10).Bold().LetterSpacing(1.1).TextColor(theme.TextMuted)
									mygoui.Spacer(c)
									mygoui.Text(c, "查看队列 ›").FontSize(10).TextColor(theme.AccentAmber)
								})
								mygoui.Text(c, fmt.Sprintf("%s · 队列 %d", statusText, queued)).FontSize(11).TextColor(theme.TextPrimary)
							})
						})
					if queueBtn.Hovered() {
						queueBtn = queueBtn.Background(theme.BgElevated).Border(1, theme.AccentAmber)
						queueBtn = queueBtn.AnimatePose(mygoui.Pose{Y: -1}, mygoui.Snappy)
					} else {
						queueBtn = queueBtn.AnimatePose(mygoui.Pose{}, mygoui.Snappy)
					}
					queueBtn = queueBtn.Transition(mygoui.ElementTransition{Colors: true, Duration: 160 * time.Millisecond, Ease: mygoui.EaseOut})
					if queueBtn.Clicked() {
						a.NavigateTo(ViewSettings, 0)
					}
				})

				mygoui.Column(c).FillHeight().Grow(1).MinWidth(0).MinHeight(0).Children(func() {
					if navLoading {
						mygoui.Column(c).Fill().Center().Padding(32).Children(func() {
							mygoui.Box(c).Width(460).Children(func() {
								theme.GlassCard(c, func() {
									theme.SectionTitle(c, "正在准备视图")
									theme.ProgressBar(c, navProg, navStage)
								})
							})
						})
						return
					}
					// Each workspace enters with a short fade and slide; the copy of
					// the view leaving fades out without taking layout or input.
					mygoui.Column(c.Key("workspace-" + string(active))).
						FillWidth().FillHeight().
						Transition(mygoui.ElementTransition{
							Duration:     190 * time.Millisecond,
							Ease:         mygoui.EaseOut,
							ExitDuration: 120 * time.Millisecond,
							Position:     true,
							Enter:        &mygoui.Motion{Y: 10, Opacity: 1},
							Exit:         &mygoui.Motion{Y: -6, Opacity: 0},
						}).
						Children(func() {
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
	modal := mygoui.Column(c).Width(460).
		Background(theme.BgSurface).
		Border(1, theme.RimBorder).
		Radius(22).
		Padding(22).
		Gap(14).
		Shadow(0, 18, 40, -6, mygoui.RGBA(0, 0, 0, 0.55))
	modal.Children(func() {
		theme.RowCenter(c).Gap(10).Children(func() {
			mygoui.Box(c).Width(32).Height(32).Radius(10).
				Background(mygoui.RGBA(56, 189, 248, 0.14)).
				Center().
				Children(func() {
					theme.RenderIcon(c, theme.IconBolt, 16, theme.AccentAmber)
				})
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
			if theme.IconAccentButton(c, theme.IconBolt, "确认并退出主界面").Clicked() {
				_ = a.ConfirmStartRenderNow()
			}
		})
	})
}

func (a *App) renderNotificationsPopover(c *mygoui.Context, items []NotificationItem, unread int) {
	panel := mygoui.Column(c).Width(340).
		Background(theme.BgSurface).
		Border(1, theme.RimBorder).
		Radius(18).
		Padding(14).
		Gap(10).
		Shadow(0, 14, 34, -4, mygoui.RGBA(0, 0, 0, 0.52))
	panel.Children(func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.RenderIcon(c, theme.IconBell, 15, theme.AccentAmber)
			theme.SectionTitle(c, "通知中心")
			if unread > 0 {
				theme.PillBadge(c, fmt.Sprintf("%d 条未读", unread), theme.AccentAmber)
			}
			mygoui.Spacer(c)
			if len(items) > 0 {
				if theme.ActionButton(c, "全部已读").Clicked() {
					a.mu.Lock()
					for i := range a.notifications {
						a.notifications[i].Unread = false
					}
					a.mu.Unlock()
				}
				if theme.ActionButton(c, "清空").Clicked() {
					a.mu.Lock()
					a.notifications = nil
					a.mu.Unlock()
				}
			}
		})
		if len(items) == 0 {
			mygoui.Column(c).FillWidth().PaddingY(20).Center().Gap(6).Children(func() {
				theme.RenderIcon(c, theme.IconCheck, 20, theme.TextMuted)
				theme.MutedText(c, "暂无新通知")
			})
			return
		}
		mygoui.Scroll(c).FillWidth().MaxHeight(260).Children(func() {
			mygoui.Column(c).FillWidth().Gap(8).Children(func() {
				for _, it := range items {
					bg := theme.BgInput
					if it.Unread {
						bg = theme.BgElevated
					}
					mygoui.Column(c).FillWidth().Background(bg).Border(1, theme.RimBorder).Radius(12).Padding(10).Gap(4).Children(func() {
						theme.RowCenter(c).FillWidth().Gap(6).Children(func() {
							theme.StatusDot(c, it.Accent, 6)
							mygoui.Text(c, it.Title).FontSize(12).Bold().TextColor(theme.TextPrimary)
							mygoui.Spacer(c)
							theme.StatText(c, it.Time).FontSize(10).TextColor(theme.TextMuted)
						})
						if it.Detail != "" {
							theme.MutedText(c, it.Detail)
						}
					})
				}
			})
		})
	})
}
