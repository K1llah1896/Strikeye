// Package guard implements the lightweight Render Mutex Guard Dialog shown
// when the user launches strikeye.exe while CS2 background rendering is active.
package guard

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"strikeye/internal/ipc"
	"strikeye/internal/ui/theme"
)

// Decision is the user outcome of the Guard Dialog.
type Decision int

const (
	DecisionContinueBackground Decision = iota
	DecisionOpenUI
	DecisionCancelled
)

// RenderStatusSnapshot mirrors the subset of agent render.status displayed in the guard dialog.
type RenderStatusSnapshot struct {
	Rendering        bool   `json:"rendering"`
	QueuedCount      int    `json:"queued_count"`
	CurrentTaskLabel string `json:"current_task_label"`
	CurrentProgress  string `json:"current_progress"`
}

// IPCClient abstracts the JSON-RPC calls needed by the Guard Dialog for testability.
type IPCClient interface {
	Call(ctx context.Context, method string, params any, result any) error
}

// Controller manages Guard Dialog state, IPC polling, and safe abort handoff.
type Controller struct {
	mu         sync.RWMutex
	client     IPCClient
	renderLock ipc.Lock
	status     RenderStatusSnapshot
	aborting   bool
	errText    string
	decision   Decision
}

// NewController constructs a Guard Dialog controller.
func NewController(client IPCClient, renderLock ipc.Lock) *Controller {
	return &Controller{
		client:     client,
		renderLock: renderLock,
		decision:   DecisionContinueBackground,
	}
}

// PollOnce queries render.status from strikeye-agent and updates controller state.
func (g *Controller) PollOnce(ctx context.Context) error {
	if g.client == nil {
		return nil
	}
	var raw map[string]any
	if err := g.client.Call(ctx, "render.status", nil, &raw); err != nil {
		g.mu.Lock()
		g.errText = fmt.Sprintf("无法连接后台 Agent: %v", err)
		g.mu.Unlock()
		return err
	}
	snap := RenderStatusSnapshot{}
	if v, ok := raw["rendering"].(bool); ok {
		snap.Rendering = v
	}
	if v, ok := raw["queued_count"].(float64); ok {
		snap.QueuedCount = int(v)
	}
	if v, ok := raw["current_task_label"].(string); ok {
		snap.CurrentTaskLabel = v
	}
	if v, ok := raw["current_progress"].(string); ok {
		snap.CurrentProgress = v
	}
	g.mu.Lock()
	g.status = snap
	g.errText = ""
	g.mu.Unlock()
	return nil
}

// AbortAndWait sends render.abort_and_cleanup and waits for renderLock to be released.
func (g *Controller) AbortAndWait(ctx context.Context, pollInterval time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 150 * time.Millisecond
	}
	g.mu.Lock()
	g.aborting = true
	g.errText = ""
	g.mu.Unlock()

	if g.client != nil {
		var resp map[string]any
		if err := g.client.Call(ctx, "render.abort_and_cleanup", nil, &resp); err != nil {
			g.mu.Lock()
			g.aborting = false
			g.errText = fmt.Sprintf("中止失败: %v", err)
			g.mu.Unlock()
			return err
		}
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if g.renderLock == nil {
			g.mu.Lock()
			g.decision = DecisionOpenUI
			g.aborting = false
			g.mu.Unlock()
			return nil
		}
		held, err := g.renderLock.IsHeld()
		if err == nil && !held {
			g.mu.Lock()
			g.decision = DecisionOpenUI
			g.aborting = false
			g.mu.Unlock()
			return nil
		}
		select {
		case <-ctx.Done():
			g.mu.Lock()
			g.aborting = false
			g.errText = "等待渲染锁释放超时"
			g.mu.Unlock()
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Snapshot returns the current status, aborting flag, and error text safely.
func (g *Controller) Snapshot() (RenderStatusSnapshot, bool, string) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.status, g.aborting, g.errText
}

// Decision returns the final user decision.
func (g *Controller) Decision() Decision {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.decision
}

// RunModal displays the lightweight MyGo Native UI Guard Dialog and blocks until
// the user chooses to continue background rendering or safely abort and open the UI.
func (g *Controller) RunModal(ctx context.Context) (Decision, error) {
	theme.RegisterFonts()
	pollCtx, cancelPoll := context.WithCancel(ctx)
	defer cancelPoll()

	_ = g.PollOnce(pollCtx)
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-pollCtx.Done():
				return
			case <-t.C:
				_ = g.PollOnce(pollCtx)
			}
		}
	}()

	var win *mygo.Window
	mygo.App.WhenReady(func() {
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Strikeye — 后台渲染守卫",
			Width:     520,
			Height:    280,
			MinWidth:  480,
			MinHeight: 260,
			Content: ui.View(func(c *ui.Context) {
				snap, aborting, errText := g.Snapshot()
				ui.Themed(c, theme.ActiveTheme()).Fill().Children(func() {
					theme.AmbientBackdrop(c)
					ui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Padding(20).Gap(14).Children(func() {
						theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
							ui.Box(c).Width(30).Height(30).Radius(10).
								Background(ui.RGBA(56, 189, 248, 0.14)).
								Center().
								Children(func() {
									theme.RenderIcon(c, theme.IconShield, 16, theme.AccentAmber)
								})
							theme.SectionTitle(c, "Strikeye 正在后台使用 CS2 引擎渲染视频")
							ui.Spacer(c)
							theme.PillBadge(c, fmt.Sprintf("待渲染: %d", snap.QueuedCount), theme.AccentAmber)
						})
						theme.GlassCard(c, func() {
							label := snap.CurrentTaskLabel
							if label == "" {
								label = "正在准备或执行 HLAE + CS2 录制任务..."
							}
							ui.Text(c, label).FontSize(13).Bold().TextColor(theme.TextPrimary)
							if snap.CurrentProgress != "" {
								theme.StatText(c, "进度: "+snap.CurrentProgress).FontSize(12).TextColor(theme.AccentCyan)
							} else {
								theme.MutedText(c, "为避免显卡上下文冲突，主界面与 CS2 渲染严格互斥。")
							}
							if errText != "" {
								ui.Text(c, errText).FontSize(12).TextColor(theme.StatusRed)
							}
						})
						ui.Spacer(c)
						theme.RowCenter(c).FillWidth().Gap(12).Children(func() {
							ui.Spacer(c)
							abortLabel := "安全中止渲染并打开 Strikeye"
							if aborting {
								abortLabel = "正在安全中止并恢复 CS2 配置..."
							}
							if theme.DangerButton(c, abortLabel).Clicked() && !aborting {
								go func() {
									abortCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
									defer cancel()
									if err := g.AbortAndWait(abortCtx, 150*time.Millisecond); err == nil && win != nil {
										win.Close()
									}
								}()
							}
							if theme.AccentButton(c, "继续后台渲染（稍后查看）").Clicked() && !aborting {
								g.mu.Lock()
								g.decision = DecisionContinueBackground
								g.mu.Unlock()
								if win != nil {
									win.Close()
								}
							}
						})
					})
				})
			}),
		})
	})

	if err := mygo.App.Run(); err != nil {
		return DecisionCancelled, err
	}
	return g.Decision(), nil
}
