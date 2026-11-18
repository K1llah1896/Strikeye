// Package settings implements View 5: Settings & Tasks for environment probing,
// storage management, render policy configuration, and render_tasks queue control.
package settings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/config"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
	"strikeye/internal/updater"
	"strikeye/internal/version"
)

// AgentCaller abstracts JSON-RPC calls to strikeye-agent.
type AgentCaller interface {
	Call(ctx context.Context, method string, params any, result any) error
}

// View manages Settings & Tasks state and rendering.
type View struct {
	db          *storage.DB
	agent       AgentCaller
	dataRoot    string
	cfg         config.Config
	probe       config.ProbeReport
	resChoice   string
	fpsChoice   string
	aliasesText string
	watchDirsTx string
	tasks       []storage.RenderTask
	taskFilter  string
	agentLogs   string
	statusMsg   string

	updateMu       sync.RWMutex
	updateBusy     bool
	updateProgress float64
	updateStage    string
	updateResult   *updater.CheckResult
	updateApplied  bool
	updateError    string
	onInvalidate   func()
	onRestart      func()
}

// SetCallbacks registers UI invalidation and app restart callbacks for async update flows.
func (v *View) SetCallbacks(onInvalidate func(), onRestart func()) {
	v.updateMu.Lock()
	v.onInvalidate = onInvalidate
	v.onRestart = onRestart
	v.updateMu.Unlock()
}

func (v *View) invalidate() {
	v.updateMu.RLock()
	fn := v.onInvalidate
	v.updateMu.RUnlock()
	if fn != nil {
		fn()
	}
}

// NewView constructs a Settings & Tasks view.
func NewView(db *storage.DB, agent AgentCaller, dataRoot string) *View {
	v := &View{
		db:         db,
		agent:      agent,
		dataRoot:   dataRoot,
		taskFilter: "全部任务",
	}
	_ = v.Reload(context.Background())
	return v
}

// Reload loads config.json, runs environment probes, and refreshes render_tasks.
func (v *View) Reload(ctx context.Context) error {
	cfg, _, err := config.Load(v.dataRoot)
	if err != nil {
		return err
	}
	v.cfg = cfg
	v.probe = config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths})
	v.resChoice = fmt.Sprintf("%dx%d", cfg.Render.Width, cfg.Render.Height)
	v.fpsChoice = fmt.Sprintf("%d", cfg.Render.FPS)
	v.aliasesText = strings.Join(cfg.PrimaryPlayerAliases, ", ")
	v.watchDirsTx = strings.Join(cfg.DemoWatchDirs, "\n")
	return v.RefreshTasks(ctx)
}

// RefreshTasks reloads render_tasks from SQLite WAL.
func (v *View) RefreshTasks(ctx context.Context) error {
	if v.db == nil {
		return nil
	}
	statuses := []string{"running", "queued", "failed", "cancelled", "completed"}
	if v.taskFilter != "" && v.taskFilter != "全部任务" {
		statuses = []string{v.taskFilter}
	}
	var all []storage.RenderTask
	for _, st := range statuses {
		rows, err := v.db.RenderTasks().ListByStatus(ctx, st)
		if err == nil {
			all = append(all, rows...)
		}
	}
	v.tasks = all
	return nil
}

// ApplyChoicesToConfig applies UI string selections back onto v.cfg.
func ApplyChoicesToConfig(cfg *config.Config, resChoice, fpsChoice, aliasesText, watchDirsText string) {
	switch resChoice {
	case "2560x1440":
		cfg.Render.Width = 2560
		cfg.Render.Height = 1440
	default:
		cfg.Render.Width = 1920
		cfg.Render.Height = 1080
	}
	switch fpsChoice {
	case "120":
		cfg.Render.FPS = 120
	case "240":
		cfg.Render.FPS = 240
	default:
		cfg.Render.FPS = 60
	}
	var aliases []string
	for _, part := range strings.Split(aliasesText, ",") {
		if t := strings.TrimSpace(part); t != "" {
			aliases = append(aliases, t)
		}
	}
	cfg.PrimaryPlayerAliases = aliases

	var dirs []string
	for _, line := range strings.Split(watchDirsText, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			dirs = append(dirs, t)
		}
	}
	cfg.DemoWatchDirs = dirs
}

// BackupDatabase copies strikeye.db to <DataRoot>/backup/manual_<ts>.db.
func BackupDatabase(dataRoot string) (string, error) {
	layout, err := config.ResolveDataLayout(dataRoot)
	if err != nil {
		return "", err
	}
	src := filepath.Join(dataRoot, "strikeye.db")
	raw, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(layout.BackupDBSnapshots, fmt.Sprintf("manual_%d.db", time.Now().Unix()))
	if err := os.MkdirAll(layout.BackupDBSnapshots, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

// CleanInvalidCache removes temporary _strikeye_playback_*.dem and stale temp capture dirs.
func CleanInvalidCache(dataRoot string) (int, error) {
	layout, err := config.ResolveDataLayout(dataRoot)
	if err != nil {
		return 0, err
	}
	cacheDir := filepath.Join(layout.Root, "cache")
	removed := 0
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	for _, ent := range entries {
		name := ent.Name()
		if strings.HasPrefix(name, "_strikeye_playback_") || strings.HasPrefix(name, "tmp_") {
			if os.RemoveAll(filepath.Join(cacheDir, name)) == nil {
				removed++
			}
		}
	}
	return removed, nil
}

// Render draws the Settings & Tasks workspace.
func (v *View) Render(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(14).Gap(8).Children(func() {
			if v.statusMsg != "" {
				theme.Capsule(c, v.statusMsg, theme.AccentCyan)
			}

			// 1. Environment probe & path overrides
			theme.Card(c, func() {
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					theme.SectionTitle(c, "1. 环境检测与工具路径")
					ui.Spacer(c)
					if theme.ActionButton(c, "重新探测").Clicked() {
						v.probe = config.ProbeToolchain(config.ProbeOptions{Configured: v.cfg.Paths})
					}
				})
				ffmpegLabel := "ffmpeg"
				ffprobeLabel := "ffprobe"
				if runtime.GOOS == "windows" {
					ffmpegLabel = "ffmpeg.exe"
					ffprobeLabel = "ffprobe.exe"
				}
				toolRow(c, "cs2.exe", v.probe.CS2, &v.cfg.Paths.CS2Exe)
				toolRow(c, "HLAE.exe", v.probe.HLAE, &v.cfg.Paths.HLAEExe)
				toolRow(c, ffmpegLabel, v.probe.FFmpeg, &v.cfg.Paths.FFmpegExe)
				toolRow(c, ffprobeLabel, v.probe.FFprobe, &v.cfg.Paths.FFprobeExe)
			})

			// 2. Directories & Storage maintenance
			theme.Card(c, func() {
				theme.SectionTitle(c, "2. 目录与存储管理")
				theme.MutedText(c, "Demo 监听目录（每行一个路径）：")
				ui.TextArea(c, &v.watchDirsTx).FillWidth().Height(38)
				theme.RowCenter(c).Gap(10).Children(func() {
					if theme.ActionButton(c, "一键备份数据库与媒体索引").Clicked() {
						if dst, err := BackupDatabase(v.dataRoot); err != nil {
							v.statusMsg = "备份失败: " + err.Error()
						} else {
							v.statusMsg = "数据库已备份至: " + dst
						}
					}
					if theme.ActionButton(c, "清理无效缓存").Clicked() {
						n, _ := CleanInvalidCache(v.dataRoot)
						v.statusMsg = fmt.Sprintf("已清理 %d 项临时缓存", n)
					}
				})
			})

			// 3. Render Policy & Quality
			theme.Card(c, func() {
				theme.SectionTitle(c, "3. 后台渲染策略与画质配置")
				theme.RowCenter(c).FillWidth().Gap(12).Children(func() {
					ui.Column(c).Gap(4).Children(func() {
						theme.MutedText(c, "自动渲染策略")
						ui.Select(c, &v.cfg.Render.Policy, []string{"smart_highlights", "full_pov", "manual_only"})
					})
					ui.Column(c).Gap(4).Children(func() {
						theme.MutedText(c, "输出分辨率")
						ui.Select(c, &v.resChoice, []string{"1920x1080", "2560x1440"})
					})
					ui.Column(c).Gap(4).Children(func() {
						theme.MutedText(c, "录制帧率 (FPS)")
						ui.Select(c, &v.fpsChoice, []string{"60", "120", "240"})
					})
					ui.Column(c).Gap(4).Children(func() {
						theme.MutedText(c, "视频编码器")
						ui.Select(c, &v.cfg.Render.Encoder, []string{"auto", "h264_nvenc", "h264_qsv", "h264_amf", "libx264"})
					})
				})
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					ui.TextInput(c, &v.cfg.PrimarySteamID64).Width(220).Placeholder("主关注玩家 SteamID64")
					ui.TextInput(c, &v.aliasesText).Width(240).Placeholder("主关注玩家昵称列表 (逗号分隔)")
					ui.Checkbox(c, &v.cfg.Render.KeyOverlayEnabled, "按键 HUD 后期叠加")
					ui.Checkbox(c, &v.cfg.Render.FrameMeldEnabled, "FrameMeld 补帧")
				})
				theme.RowCenter(c).FillWidth().Children(func() {
					if theme.AccentButton(c, "保存配置并通知 Agent 重载").Clicked() {
						ApplyChoicesToConfig(&v.cfg, v.resChoice, v.fpsChoice, v.aliasesText, v.watchDirsTx)
						if err := config.Save(v.dataRoot, v.cfg); err != nil {
							v.statusMsg = "保存失败: " + err.Error()
						} else {
							v.statusMsg = "配置已保存"
							if v.agent != nil {
								_ = v.agent.Call(context.Background(), "config.reload", nil, nil)
							}
						}
					}
				})
			})

			// 4. Live Render Task Queue
			theme.Card(c, func() {
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					theme.SectionTitle(c, "4. 后台渲染任务队列")
					ui.Spacer(c)
					if ui.Select(c, &v.taskFilter, []string{"全部任务", "queued", "running", "completed", "failed", "cancelled"}).Changed() {
						_ = v.RefreshTasks(context.Background())
					}
					if theme.ActionButton(c, "刷新队列").Clicked() {
						_ = v.RefreshTasks(context.Background())
					}
				})
				if len(v.tasks) == 0 {
					theme.MutedText(c, "当前筛选条件下暂无后台渲染任务。")
				}
				for _, t := range v.tasks {
					v.renderTaskRow(c, t)
				}
			})

			// 5. Appearance & Theme Customization
			theme.Card(c, func() {
				theme.SectionTitle(c, "5. 外观与主题配置")
				presetIDs := make([]string, 0, len(theme.Presets()))
				for _, p := range theme.Presets() {
					presetIDs = append(presetIDs, p.ID)
				}
				theme.RowCenter(c).FillWidth().Gap(12).Children(func() {
					ui.Column(c).Gap(4).Children(func() {
						theme.MutedText(c, "主题预设")
						if ui.Select(c, &v.cfg.Theme.Preset, presetIDs).Changed() {
							for _, p := range theme.Presets() {
								if p.ID == v.cfg.Theme.Preset {
									v.cfg.Theme.AccentColor = p.Accent
									break
								}
							}
							theme.Apply(v.cfg.Theme)
						}
					})
					ui.Column(c).Gap(4).Children(func() {
						theme.MutedText(c, "强调色 (Hex)")
						if ui.TextInput(c, &v.cfg.Theme.AccentColor).Width(120).Placeholder("#F59E0B").Changed() {
							theme.Apply(v.cfg.Theme)
						}
					})
					ui.Column(c).Gap(4).Children(func() {
						theme.MutedText(c, "快捷强调色板")
						theme.RowCenter(c).Gap(6).Children(func() {
							for _, sw := range theme.AccentSwatches() {
								btn := ui.ButtonBase(c).
									Width(22).Height(22).Radius(11).
									Background(ui.Hex(sw.Hex)).
									Border(1, theme.BorderSubtle)
								if strings.EqualFold(v.cfg.Theme.AccentColor, sw.Hex) {
									btn = btn.Border(2, theme.TextPrimary)
								}
								if btn.Clicked() {
									v.cfg.Theme.AccentColor = sw.Hex
									theme.Apply(v.cfg.Theme)
								}
							}
						})
					})
				})
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					theme.MutedText(c, "背景图片:").Width(64)
					ui.TextInput(c, &v.cfg.Theme.BackgroundImage).FillWidth().Placeholder("输入本地 PNG/JPG 图片路径（留空则使用纯色背景）")
					if theme.ActionButton(c, "清除").Clicked() {
						v.cfg.Theme.BackgroundImage = ""
						theme.Apply(v.cfg.Theme)
					}
				})
				theme.RowCenter(c).FillWidth().Gap(16).Children(func() {
					theme.RowCenter(c).Gap(8).Children(func() {
						theme.MutedText(c, fmt.Sprintf("背景遮罩: %.0f%%", v.cfg.Theme.OverlayOpacity*100))
						if ui.Slider(c, &v.cfg.Theme.OverlayOpacity, 0.0, 0.95).Width(140).Changed() {
							theme.Apply(v.cfg.Theme)
						}
					})
					theme.RowCenter(c).Gap(8).Children(func() {
						theme.MutedText(c, fmt.Sprintf("卡片不透明度: %.0f%%", v.cfg.Theme.CardOpacity*100))
						if ui.Slider(c, &v.cfg.Theme.CardOpacity, 0.4, 1.0).Width(140).Changed() {
							theme.Apply(v.cfg.Theme)
						}
					})
					ui.Spacer(c)
					if theme.AccentButton(c, "应用并保存主题").Clicked() {
						theme.Apply(v.cfg.Theme)
						if err := config.Save(v.dataRoot, v.cfg); err != nil {
							v.statusMsg = "主题保存失败: " + err.Error()
						} else {
							v.statusMsg = "主题配置已保存并生效"
						}
					}
				})
			})

			// 6. Version & In-App Hot Update
			v.renderUpdateCard(c)
		})
	})
}

func (v *View) renderUpdateCard(c *ui.Context) {
	v.updateMu.RLock()
	busy := v.updateBusy
	prog := v.updateProgress
	stage := v.updateStage
	res := v.updateResult
	applied := v.updateApplied
	errMsg := v.updateError
	v.updateMu.RUnlock()

	theme.Card(c, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.SectionTitle(c, "6. 版本信息与在线热更新")
			ui.Spacer(c)
			theme.Capsule(c, "当前版本: "+version.AppVersion, theme.AccentCyan)
		})
		theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
			if ui.Checkbox(c, &v.cfg.Update.AutoCheck, "启动时自动检查更新").Changed() {
				_ = config.Save(v.dataRoot, v.cfg)
			}
			ui.Spacer(c)
			if !busy && theme.AccentButton(c, "检查更新").Clicked() {
				v.updateMu.Lock()
				v.updateBusy = true
				v.updateProgress = -1
				v.updateStage = "正在检查 GitHub 最新版本..."
				v.updateError = ""
				v.updateApplied = false
				v.updateMu.Unlock()
				v.invalidate()

				go func() {
					checkRes, err := updater.CheckUpdate(context.Background(), nil, updater.DefaultGitHubReleasesURL, version.AppVersion)
					v.updateMu.Lock()
					v.updateBusy = false
					v.updateProgress = 0
					v.updateStage = ""
					if err != nil {
						v.updateError = err.Error()
						v.updateResult = nil
					} else {
						v.updateError = ""
						v.updateResult = &checkRes
					}
					v.updateMu.Unlock()
					v.invalidate()
				}()
			}
			if !busy && res != nil && res.HasUpdate && !applied {
				if theme.AccentButton(c, "立即下载并热更新 (v"+res.LatestVersion+")").Clicked() {
					asset := res.Asset
					targetDir := v.dataRoot
					if exe, err := os.Executable(); err == nil {
						targetDir = filepath.Dir(exe)
					}
					v.updateMu.Lock()
					v.updateBusy = true
					v.updateProgress = 0.02
					v.updateStage = "准备下载更新..."
					v.updateError = ""
					v.updateMu.Unlock()
					v.invalidate()

					go func() {
						err := updater.DownloadAndApply(context.Background(), nil, asset, targetDir, func(st string, p float64) {
							v.updateMu.Lock()
							v.updateStage = st
							v.updateProgress = p
							v.updateMu.Unlock()
							v.invalidate()
						})
						v.updateMu.Lock()
						v.updateBusy = false
						if err != nil {
							v.updateError = err.Error()
						} else {
							v.updateApplied = true
							v.updateProgress = 1.0
							v.updateStage = "热更新已就绪，请重启应用"
						}
						v.updateMu.Unlock()
						v.invalidate()
					}()
				}
			}
			if applied {
				if theme.AccentButton(c, "立即重启应用以生效").Clicked() {
					v.updateMu.RLock()
					restartFn := v.onRestart
					v.updateMu.RUnlock()
					if restartFn != nil {
						restartFn()
					}
				}
			}
		})

		if busy {
			theme.ProgressBar(c, prog, stage)
		} else if errMsg != "" {
			theme.Capsule(c, "更新异常: "+errMsg, theme.StatusRed)
		} else if applied {
			theme.Capsule(c, "热更新替换成功，点击「立即重启应用以生效」完成切换", theme.StatusGreen)
		} else if res != nil {
			if res.HasUpdate {
				note := res.Notes
				if note == "" {
					note = "包含功能优化与问题修复"
				}
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					theme.Capsule(c, "发现新版本: "+res.LatestVersion, theme.AccentAmber)
					theme.MutedText(c, note)
				})
			} else {
				theme.Capsule(c, "当前已是最新版本 ("+res.CurrentVersion+")", theme.StatusGreen)
			}
		}
	})
}

func toolRow(c *ui.Context, name string, st config.ToolStatus, pathPtr *string) {
	theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, name).Width(100).Font("Rajdhani").FontSize(14).Bold().TextColor(theme.TextPrimary)
		if st.Available {
			theme.Capsule(c, "可用", theme.StatusGreen)
		} else {
			theme.Capsule(c, "缺失", theme.StatusRed)
		}
		ui.TextInput(c, pathPtr).FillWidth().Placeholder(st.Path)
	})
}

func (v *View) renderTaskRow(c *ui.Context, t storage.RenderTask) {
	theme.RowCenter(c).FillWidth().Background(theme.BgInput).Border(1, theme.BorderSubtle).Radius(10).Padding(8).Gap(10).Children(func() {
		theme.ScoreText(c, fmt.Sprintf("#%d", t.ID)).FontSize(12).TextColor(theme.AccentAmber)
		theme.Capsule(c, t.Status, theme.MediaStatusColor(t.Status))
		ui.Text(c, t.TargetPlayerName).FontSize(12).Bold().TextColor(theme.TextPrimary)
		theme.StatText(c, fmt.Sprintf("%s  •  P=%d", t.TaskScope, t.Priority)).FontSize(11).TextColor(theme.TextMuted)
		if t.ErrorMessage != nil && *t.ErrorMessage != "" {
			ui.Text(c, *t.ErrorMessage).FontSize(11).TextColor(theme.StatusRed)
		}
		ui.Spacer(c)
		if t.Status == "queued" && v.agent != nil {
			if theme.ActionButton(c, "置顶").Clicked() {
				_ = v.agent.Call(context.Background(), "render.priority", map[string]any{"task_id": t.ID, "priority": t.Priority + 50}, nil)
				_ = v.RefreshTasks(context.Background())
			}
			if theme.DangerButton(c, "取消").Clicked() {
				_ = v.agent.Call(context.Background(), "render.cancel", map[string]any{"task_id": t.ID}, nil)
				_ = v.RefreshTasks(context.Background())
			}
		}
		if (t.Status == "failed" || t.Status == "cancelled") && v.agent != nil {
			if theme.AccentButton(c, "重试").Clicked() {
				_ = v.agent.Call(context.Background(), "render.retry", map[string]any{"task_id": t.ID}, nil)
				_ = v.RefreshTasks(context.Background())
			}
		}
	})
}
