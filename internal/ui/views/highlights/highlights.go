// Package highlights implements View 4: Highlight Library and Montage Workbench.
package highlights

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/composer"
	"strikeye/internal/storage"
	"strikeye/internal/ui/components"
	"strikeye/internal/ui/theme"
)

// AgentCaller abstracts JSON-RPC calls to strikeye-agent.
type AgentCaller interface {
	Call(ctx context.Context, method string, params any, result any) error
}

// HighlightFilter filters HighlightClip items in the Highlight Library.
type HighlightFilter struct {
	PlayerQuery   string
	ClipType      string
	TagQuery      string
	RenderedOnly  bool
	FavoritesOnly bool
}

// MontageItem represents one ordered clip entry in the Montage Workbench timeline.
type MontageItem struct {
	Clip           storage.HighlightClip
	VideoPath      string
	TrimStartSec   float64
	TrimEndSec     float64
	InsertRadar    bool
	RadarMediaPath string
	Subtitle       string
}

// View manages Highlight Library and Montage Workbench state.
type View struct {
	mu             sync.RWMutex
	db             *storage.DB
	agent          AgentCaller
	ffmpegPath     string
	demoID         int64
	demo           *storage.Demo
	clips          []storage.HighlightClip
	videosByID     map[int64]storage.MediaVideo
	radarBySteam   map[string]storage.RadarCard
	filter         HighlightFilter
	selectedClipID string
	playerWidget   *components.VideoPlayerWidget
	activeTab      string // "library" or "montage"
	montageItems   []MontageItem
	title          string
	introPath      string
	outroPath      string
	bgmPath        string
	audioTracks    []audioTrackDraft
	clipVol        float64
	bgmVol         float64
	transition     string
	transitionDur  float64
	showNameCard   bool
	outputPath     string
	exportProgress float64
	exportStatus   string
	statusMsg      string
	onStartRender  func()
}

// NewView constructs a Highlights & Montage view.
func NewView(db *storage.DB, agent AgentCaller, ffmpegPath string, onStartRender func()) *View {
	return &View{
		db:           db,
		agent:        agent,
		ffmpegPath:   ffmpegPath,
		videosByID:   make(map[int64]storage.MediaVideo),
		radarBySteam: make(map[string]storage.RadarCard),
		playerWidget: components.NewVideoPlayerWidget(ffmpegPath),
		activeTab:    "library",
		filter: HighlightFilter{
			PlayerQuery: "全部选手",
			ClipType:    "全部类型",
			TagQuery:    "全部标签",
		},
		title:         "Strikeye Highlight Montage",
		audioTracks:   defaultAudioTracks(),
		clipVol:       1.0,
		bgmVol:        0.35,
		transition:    "fade",
		transitionDur: 0.5,
		showNameCard:  true,
		onStartRender: onStartRender,
	}
}

// Close releases active preview decoder resources.
func (v *View) Close() {
	if v.playerWidget != nil {
		v.playerWidget.Close()
	}
}

// LoadDemo refreshes highlights, verified videos, and radar cards for the selected demoID.
// When demoID <= 0, highlights remain unbound (empty) until a demo is selected.
func (v *View) LoadDemo(ctx context.Context, demoID int64) error {
	videosByID := make(map[int64]storage.MediaVideo)
	radarBySteam := make(map[string]storage.RadarCard)
	var clips []storage.HighlightClip
	var demo *storage.Demo

	if v.db != nil && demoID > 0 {
		if d, err := v.db.Demos().GetByID(ctx, demoID); err == nil {
			demo = &d
		}
		allMedia, _ := v.db.Media().ListByDemo(ctx, demoID)
		for _, mv := range allMedia {
			videosByID[mv.ID] = mv
		}
		clips, _ = v.db.Analysis().ListHighlightClips(ctx, demoID)
		cards, _ := v.db.Analysis().ListRadarCards(ctx, demoID)
		for _, c := range cards {
			radarBySteam[c.SteamID64] = c
		}
	}

	if v.playerWidget != nil {
		v.playerWidget.Close()
	}
	v.mu.Lock()
	v.demoID = demoID
	v.demo = demo
	v.videosByID = videosByID
	v.radarBySteam = radarBySteam
	v.clips = clips
	v.selectedClipID = ""
	v.mu.Unlock()
	return nil
}

// SetExportProgress updates the live montage export status from IPC events.
func (v *View) SetExportProgress(status string, progress float64, msg string) {
	v.mu.Lock()
	v.exportStatus = status
	v.exportProgress = progress
	if msg != "" {
		v.statusMsg = msg
	}
	v.mu.Unlock()
}

// FilterClips filters HighlightClip items purely in-memory.
func FilterClips(clips []storage.HighlightClip, f HighlightFilter) []storage.HighlightClip {
	pq := strings.ToLower(strings.TrimSpace(f.PlayerQuery))
	if pq == "全部选手" {
		pq = ""
	}
	tq := strings.ToLower(strings.TrimSpace(f.TagQuery))
	if tq == "全部标签" {
		tq = ""
	}
	out := make([]storage.HighlightClip, 0, len(clips))
	for _, c := range clips {
		if f.FavoritesOnly && !c.IsFavorite {
			continue
		}
		if f.RenderedOnly && (c.MediaVideoID == nil || *c.MediaVideoID == 0) {
			continue
		}
		if f.ClipType != "" && f.ClipType != "全部类型" && !strings.EqualFold(c.ClipType, f.ClipType) {
			continue
		}
		if pq != "" && !strings.Contains(strings.ToLower(c.PlayerName), pq) {
			continue
		}
		if tq != "" && !strings.Contains(strings.ToLower(c.TagsJSON), tq) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (v *View) playerOptions() []string {
	v.mu.RLock()
	clips := v.clips
	v.mu.RUnlock()
	seen := map[string]bool{}
	var items []string
	for _, c := range clips {
		name := strings.TrimSpace(c.PlayerName)
		if name != "" && !seen[name] {
			seen[name] = true
			items = append(items, name)
		}
	}
	sort.Strings(items)
	return append([]string{"全部选手"}, items...)
}

func (v *View) clipTypeOptions() []string {
	v.mu.RLock()
	clips := v.clips
	v.mu.RUnlock()
	seen := map[string]bool{}
	var items []string
	for _, c := range clips {
		ct := strings.TrimSpace(c.ClipType)
		if ct != "" && !seen[ct] {
			seen[ct] = true
			items = append(items, ct)
		}
	}
	for _, def := range []string{"ace", "4k", "3k", "2k", "clutch", "defuse", "one_tap", "knife", "long_range", "noscope", "airborne"} {
		if !seen[def] {
			seen[def] = true
			items = append(items, def)
		}
	}
	sort.Strings(items)
	return append([]string{"全部类型"}, items...)
}

func (v *View) tagOptions() []string {
	v.mu.RLock()
	clips := v.clips
	v.mu.RUnlock()
	seen := map[string]bool{}
	var items []string
	for _, c := range clips {
		for _, t := range theme.ParseTags(c.TagsJSON) {
			if t != "" && !seen[t] {
				seen[t] = true
				items = append(items, t)
			}
		}
	}
	sort.Strings(items)
	return append([]string{"全部标签"}, items...)
}

// BuildMontageSpec assembles a composer.MontageSpec from ordered MontageItems and workbench options.
func BuildMontageSpec(
	demoID int64,
	title, introPath, outroPath, bgmPath, outputPath string,
	clipVol, bgmVol float64,
	transition string,
	transitionDur float64,
	showNameCard bool,
	items []MontageItem,
) composer.MontageSpec {
	clips := make([]composer.MontageClip, 0, len(items))
	var radars []composer.RadarSegment
	for _, it := range items {
		mc := composer.MontageClip{
			ID:            it.Clip.ID,
			VideoPath:     it.VideoPath,
			TrimStartSec:  it.TrimStartSec,
			TrimEndSec:    it.TrimEndSec,
			Transition:    transition,
			TransitionDur: transitionDur,
			Subtitle:      it.Subtitle,
		}
		if showNameCard {
			mc.NameCard = &composer.NameCardInput{
				PlayerName:   it.Clip.PlayerName,
				RoundNumber:  it.Clip.RoundNumber,
				HighlightTag: fmt.Sprintf("%s (%dK)", it.Clip.ClipType, it.Clip.KillCount),
			}
		}
		clips = append(clips, mc)
		if it.InsertRadar && it.RadarMediaPath != "" {
			radars = append(radars, composer.RadarSegment{
				BeforeClipID: it.Clip.ID,
				MediaPath:    it.RadarMediaPath,
				DurationSec:  4.0,
			})
		}
	}
	return composer.MontageSpec{
		DemoID:        demoID,
		Title:         title,
		IntroPath:     introPath,
		OutroPath:     outroPath,
		Clips:         clips,
		RadarSegments: radars,
		BGMPath:       bgmPath,
		ClipVolume:    clipVol,
		BGMVolume:     bgmVol,
		BGMFadeSec:    1.5,
		Width:         1920,
		Height:        1080,
		FPS:           60,
		OutputPath:    outputPath,
	}
}

// Render draws the Highlight Library and Montage Workbench tabs.
func (v *View) Render(c *ui.Context) {
	v.mu.RLock()
	demoID := v.demoID
	demo := v.demo
	clipCount := len(v.clips)
	statusMsg := v.statusMsg
	v.mu.RUnlock()

	if demoID <= 0 {
		ui.Column(c).Fill().Center().Padding(32).Children(func() {
			ui.Box(c).Width(460).Children(func() {
				theme.EmptyStateCard(c, theme.IconHighlights, "尚未选择比赛", "请先在比赛资料库选择一场比赛，以查看该场次的精彩片段。")
			})
		})
		return
	}

	ui.Column(c).Fill().Padding(14).Gap(14).Children(func() {
		if demo != nil {
			mapName := "未知地图"
			if demo.MapName != nil {
				mapName = *demo.MapName
			}
			theme.RenderMapScoreBanner(c, mapName, demo.TeamAName, demo.TeamBName, demo.TeamAScore, demo.TeamBScore, func() {
				theme.PillBadge(c, fmt.Sprintf("精彩片段 %d  •  合辑 %d 段", clipCount, len(v.montageItems)), theme.AccentAmber)
			})
		}

		theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
			theme.SegmentedControl(c, []theme.SegmentItem{
				{Key: "library", Label: "1. 精彩片段库", Icon: theme.IconHighlights},
				{Key: "montage", Label: fmt.Sprintf("2. 合辑创作工作台 (%d 段)", len(v.montageItems)), Icon: theme.IconFilm},
			}, v.activeTab, func(k string) {
				v.activeTab = k
			})
			ui.Spacer(c)
			if statusMsg != "" {
				theme.PillBadge(c, statusMsg, theme.AccentCyan)
			}
		})

		ui.Column(c.Key("highlights-tab-" + v.activeTab)).FillWidth().Grow(1).MinHeight(0).
			Transition(ui.ElementTransition{
				Duration:     180 * time.Millisecond,
				Ease:         ui.EaseOut,
				ExitDuration: 120 * time.Millisecond,
				Position:     true,
				Enter:        &ui.Motion{Y: 8, Opacity: 1},
				Exit:         &ui.Motion{Y: -6, Opacity: 0},
			}).
			Children(func() {
				if v.activeTab == "library" {
					v.renderLibrary(c)
				} else {
					v.renderWorkbench(c)
				}
			})
	})
}

func (v *View) renderLibrary(c *ui.Context) {
	v.mu.RLock()
	clips := v.clips
	v.mu.RUnlock()

	theme.GlassCard(c, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.Select(c, &v.filter.PlayerQuery, v.playerOptions())
			ui.Select(c, &v.filter.ClipType, v.clipTypeOptions())
			ui.Select(c, &v.filter.TagQuery, v.tagOptions())
			ui.Checkbox(c, &v.filter.RenderedOnly, "仅看已生成视频")
			ui.Checkbox(c, &v.filter.FavoritesOnly, "仅看收藏")
		})
	})

	filtered := FilterClips(clips, v.filter)
	ui.Row(c).FillWidth().Grow(1).MinHeight(0).Gap(14).Children(func() {
		ui.Column(c).Width(420).FillHeight().MinHeight(0).Children(func() {
			ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).FillWidth().Gap(10).Children(func() {
					if len(filtered) == 0 {
						theme.EmptyStateCard(c, theme.IconHighlights, "暂无符合条件的高光片段", "调整上方筛选条件或等待后台解析完成。")
						return
					}
					for _, clip := range filtered {
						ui.Column(c.Key(clip.ID)).FillWidth().
							Transition(ui.ElementTransition{
								Duration:     200 * time.Millisecond,
								Ease:         ui.EaseOut,
								ExitDuration: 140 * time.Millisecond,
								Position:     true,
								Enter:        &ui.Motion{Y: 10, Opacity: 0},
								Exit:         &ui.Motion{Y: -6, Opacity: 0},
							}).
							Children(func() {
								v.renderClipCard(c, clip)
							})
					}
				})
			})
		})
		ui.Column(c).FillHeight().Grow(1).MinWidth(0).MinHeight(0).Children(func() {
			v.renderClipPreview(c)
		})
	})
}

func (v *View) renderClipCard(c *ui.Context, clip storage.HighlightClip) {
	ready := clip.MediaVideoID != nil && *clip.MediaVideoID > 0
	var accent ui.Color
	if clip.ID == v.selectedClipID {
		accent = theme.AccentAmber
	}
	card := theme.BentoCard(c, accent, 14, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.ScoreText(c, fmt.Sprintf("R%d", clip.RoundNumber)).FontSize(16).TextColor(theme.AccentAmber)
			ui.Text(c, clip.PlayerName).FontSize(14).Bold().TextColor(theme.TextPrimary)
			theme.MutedText(c, theme.ClipTypeLabel(clip.ClipType))
			ui.Spacer(c)
			if ready {
				theme.PillBadge(c, "已生成 MP4", theme.StatusGreen)
			} else {
				theme.PillBadge(c, "未渲染", theme.StatusGray)
			}
		})
		tags := theme.ParseTags(clip.TagsJSON)
		if len(tags) > 0 {
			ui.ScrollHorizontal(c).FillWidth().Children(func() {
				theme.RowCenter(c).Gap(6).Children(func() {
					for _, tag := range tags {
						ui.Box(c).Shrink(0).
							Background(ui.RGBA(34, 211, 238, 0.12)).
							Border(1, ui.RGBA(34, 211, 238, 0.28)).
							Radius(999).
							PaddingX(10).
							PaddingY(3).
							Children(func() {
								ui.Text(c, tag).Font("Rajdhani").FontSize(11).Bold().TextColor(theme.AccentCyan)
							})
					}
				})
			})
		}
		theme.RowCenter(c).Gap(6).Children(func() {
			if theme.IconActionButton(c, theme.IconPlay, "预览/裁剪").Clicked() {
				v.mu.Lock()
				v.selectedClipID = clip.ID
				mv, ok := storage.MediaVideo{}, false
				if ready {
					mv, ok = v.videosByID[*clip.MediaVideoID]
				}
				v.mu.Unlock()
				if ready && ok {
					v.playerWidget.Load(mv.FilePath, mv.DurationSec, clip.StartTick, clip.EndTick, 64, 0)
				} else {
					v.playerWidget.Close()
				}
			}
			if ready {
				if theme.IconAccentButton(c, theme.IconPlus, "加入合辑").Clicked() {
					v.mu.RLock()
					mv, ok := v.videosByID[*clip.MediaVideoID]
					rc := v.radarBySteam[clip.PlayerSteamID64]
					v.mu.RUnlock()
					if ok {
						radarPath := rc.StaticImagePath
						if rc.AnimVideoPath != nil && *rc.AnimVideoPath != "" {
							radarPath = *rc.AnimVideoPath
						}
						v.montageItems = append(v.montageItems, MontageItem{
							Clip:           clip,
							VideoPath:      mv.FilePath,
							TrimEndSec:     mv.DurationSec,
							RadarMediaPath: radarPath,
						})
						v.mu.Lock()
						v.statusMsg = "已加入合辑工作台: " + clip.PlayerName
						v.mu.Unlock()
					}
				}
			} else {
				if theme.IconActionButton(c, theme.IconPlus, "加入渲染队列").Clicked() {
					v.enqueueClip(clip)
				}
				if theme.IconAccentButton(c, theme.IconBolt, "立即渲染").Clicked() {
					v.enqueueClip(clip)
					if v.onStartRender != nil {
						v.onStartRender()
					}
				}
			}
		})
	})
	theme.HoverLift(card, 3)
}

func (v *View) renderClipPreview(c *ui.Context) {
	v.mu.RLock()
	clips := v.clips
	selectedClipID := v.selectedClipID
	v.mu.RUnlock()

	var selected *storage.HighlightClip
	for i := range clips {
		if clips[i].ID == selectedClipID {
			cp := clips[i]
			selected = &cp
			break
		}
	}
	if selected == nil {
		theme.Card(c, func() {
			theme.MutedText(c, "点击左侧高光片段进行预览或微调裁剪点")
		})
		return
	}
	if selected.MediaVideoID == nil || *selected.MediaVideoID == 0 {
		theme.Card(c, func() {
			theme.SectionTitle(c, fmt.Sprintf("%s — R%d %s", selected.PlayerName, selected.RoundNumber, selected.ClipType))
			theme.MutedText(c, "该高光片段尚未生成视频，请点击「加入渲染队列」或「立即渲染」。")
		})
		return
	}
	v.playerWidget.Render(c, nil, true, func(inTick, outTick int64) {
		cCopy := *selected
		cCopy.StartTick = inTick
		cCopy.EndTick = outTick
		v.enqueueClip(cCopy)
	})
}

func (v *View) enqueueClip(clip storage.HighlightClip) {
	if v.agent == nil {
		return
	}
	rnd := clip.RoundNumber
	var res map[string]any
	err := v.agent.Call(context.Background(), "render.enqueue", map[string]any{
		"demo_id":            clip.DemoID,
		"clip_id":            clip.ID,
		"round_number":       rnd,
		"target_steam_id64":  clip.PlayerSteamID64,
		"target_player_name": clip.PlayerName,
		"perspective":        "pov",
		"task_scope":         "highlight_clip",
		"start_tick":         clip.StartTick,
		"end_tick":           clip.EndTick,
		"priority":           100,
	}, &res)
	if err != nil {
		v.statusMsg = "入队失败: " + err.Error()
		return
	}
	v.statusMsg = fmt.Sprintf("已入队重新渲染: %s (%d~%d)", clip.PlayerName, clip.StartTick, clip.EndTick)
}

func (v *View) renderWorkbench(c *ui.Context) {
	ui.Row(c).FillWidth().Grow(1).MinHeight(0).Gap(12).Children(func() {
		ui.Column(c).FillHeight().Grow(1).MinWidth(0).MinHeight(0).Children(func() {
			theme.Card(c, func() {
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					theme.SectionTitle(c, "合辑片段序列")
					ui.Spacer(c)
					theme.StatText(c, fmt.Sprintf("共 %d 段", len(v.montageItems))).FontSize(12).TextColor(theme.AccentAmber)
				})
				ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						if len(v.montageItems) == 0 {
							ui.Column(c).FillWidth().Height(160).Center().Gap(6).Children(func() {
								theme.MutedText(c, "合辑序列为空")
								theme.MutedText(c, "请先在「精彩片段库」中选择已生成 MP4 的片段点击「加入合辑」。")
							})
							return
						}
						moveUpIdx := -1
						moveDownIdx := -1
						removeIdx := -1
						for idx := range v.montageItems {
							it := &v.montageItems[idx]
							ui.Column(c).FillWidth().Background(theme.BgInput).Border(1, theme.BorderSubtle).Radius(6).Padding(10).Gap(8).Children(func() {
								theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
									theme.ScoreText(c, fmt.Sprintf("#%d", idx+1)).FontSize(13).TextColor(theme.AccentAmber)
									theme.ScoreText(c, fmt.Sprintf("R%d", it.Clip.RoundNumber)).FontSize(13).TextColor(theme.TextMuted)
									ui.Text(c, fmt.Sprintf("%s (%s)", it.Clip.PlayerName, it.Clip.ClipType)).
										FontSize(13).Bold().TextColor(theme.TextPrimary)
									ui.Spacer(c)
									if idx > 0 && theme.ActionButton(c, "上移").Clicked() {
										moveUpIdx = idx
									}
									if idx < len(v.montageItems)-1 && theme.ActionButton(c, "下移").Clicked() {
										moveDownIdx = idx
									}
									if theme.DangerButton(c, "移除").Clicked() {
										removeIdx = idx
									}
								})
								theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
									ui.Checkbox(c, &it.InsertRadar, "在片段前插入选手六维战力雷达图动画")
									ui.Spacer(c)
									ui.TextInput(c, &it.Subtitle).Width(220).Placeholder("自定义字幕条...")
								})
							})
						}
						if removeIdx >= 0 && removeIdx < len(v.montageItems) {
							v.montageItems = append(v.montageItems[:removeIdx], v.montageItems[removeIdx+1:]...)
						} else if moveUpIdx > 0 && moveUpIdx < len(v.montageItems) {
							v.montageItems[moveUpIdx-1], v.montageItems[moveUpIdx] = v.montageItems[moveUpIdx], v.montageItems[moveUpIdx-1]
						} else if moveDownIdx >= 0 && moveDownIdx < len(v.montageItems)-1 {
							v.montageItems[moveDownIdx+1], v.montageItems[moveDownIdx] = v.montageItems[moveDownIdx], v.montageItems[moveDownIdx+1]
						}
					})
				})
			}).FillHeight().MinHeight(0)
		})

		ui.Column(c).Width(360).FillHeight().Children(func() {
			theme.Card(c, func() {
				theme.SectionTitle(c, "合辑全局包装与导出")
				ui.TextInput(c, &v.title).FillWidth().Placeholder("合辑标题")
				ui.TextInput(c, &v.introPath).FillWidth().Placeholder("片头 MP4 路径 (可选)")
				ui.TextInput(c, &v.outroPath).FillWidth().Placeholder("片尾 MP4 路径 (可选)")
				ui.TextInput(c, &v.bgmPath).FillWidth().Placeholder("BGM 音频路径 (可选)")
				renderAudioTrackEditor(c, &v.audioTracks)
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					theme.MutedText(c, "转场效果").Width(64)
					ui.Select(c, &v.transition, []string{"fade", "fadewhite", "wipeleft", "smoothleft", "cut"}).Width(180)
				})
				ui.Checkbox(c, &v.showNameCard, "开启左下角选手信息卡")
				ui.TextInput(c, &v.outputPath).FillWidth().Placeholder("导出路径 (留空自动生成)")
				v.mu.RLock()
				exportStatus := v.exportStatus
				exportProgress := v.exportProgress
				demoID := v.demoID
				v.mu.RUnlock()
				if exportStatus != "" {
					theme.Capsule(c, fmt.Sprintf("%s %.0f%%", exportStatus, exportProgress*100), theme.AccentCyan)
				}
				if theme.AccentButton(c, "导出合辑 MP4").Clicked() && len(v.montageItems) > 0 && v.agent != nil {
					spec := BuildMontageSpec(demoID, v.title, v.introPath, v.outroPath, v.bgmPath, v.outputPath,
						v.clipVol, v.bgmVol, v.transition, v.transitionDur, v.showNameCard, v.montageItems)
					spec.AudioTracks = toAudioTracks(v.audioTracks)
					var res map[string]any
					if err := v.agent.Call(context.Background(), "montage.export", map[string]any{"spec": spec}, &res); err != nil {
						v.mu.Lock()
						v.statusMsg = "导出启动失败: " + err.Error()
						v.mu.Unlock()
					} else {
						v.mu.Lock()
						v.exportStatus = "running"
						v.exportProgress = 0.05
						v.statusMsg = "后台 FFmpeg 正在合成合辑..."
						v.mu.Unlock()
					}
				}
			})
		})
	})
}
