// Package highlights implements View 4: Highlight Library and Montage Workbench.
package highlights

import (
	"context"
	"fmt"
	"strings"

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
	db             *storage.DB
	agent          AgentCaller
	ffmpegPath     string
	demoID         int64
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
		db:            db,
		agent:         agent,
		ffmpegPath:    ffmpegPath,
		videosByID:    make(map[int64]storage.MediaVideo),
		radarBySteam:  make(map[string]storage.RadarCard),
		playerWidget:  components.NewVideoPlayerWidget(ffmpegPath),
		activeTab:     "library",
		filter:        HighlightFilter{ClipType: "全部类型"},
		title:         "Strikeye Highlight Montage",
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

// LoadDemo refreshes highlights, verified videos, and radar cards for demoID (or all demos if 0).
func (v *View) LoadDemo(ctx context.Context, demoID int64) error {
	v.demoID = demoID
	if v.db == nil {
		return nil
	}
	v.videosByID = make(map[int64]storage.MediaVideo)
	v.radarBySteam = make(map[string]storage.RadarCard)

	if demoID > 0 {
		allMedia, _ := v.db.Media().ListByDemo(ctx, demoID)
		for _, mv := range allMedia {
			v.videosByID[mv.ID] = mv
		}
		v.clips, _ = v.db.Analysis().ListHighlightClips(ctx, demoID)
		cards, _ := v.db.Analysis().ListRadarCards(ctx, demoID)
		for _, c := range cards {
			v.radarBySteam[c.SteamID64] = c
		}
	} else {
		demos, _ := v.db.Demos().List(ctx)
		var all []storage.HighlightClip
		for _, d := range demos {
			allMedia, _ := v.db.Media().ListByDemo(ctx, d.ID)
			for _, mv := range allMedia {
				v.videosByID[mv.ID] = mv
			}
			cs, _ := v.db.Analysis().ListHighlightClips(ctx, d.ID)
			all = append(all, cs...)
			cards, _ := v.db.Analysis().ListRadarCards(ctx, d.ID)
			for _, c := range cards {
				v.radarBySteam[c.SteamID64] = c
			}
		}
		v.clips = all
	}
	return nil
}

// SetExportProgress updates the live montage export status from IPC events.
func (v *View) SetExportProgress(status string, progress float64, msg string) {
	v.exportStatus = status
	v.exportProgress = progress
	if msg != "" {
		v.statusMsg = msg
	}
}

// FilterClips filters HighlightClip items purely in-memory.
func FilterClips(clips []storage.HighlightClip, f HighlightFilter) []storage.HighlightClip {
	pq := strings.ToLower(strings.TrimSpace(f.PlayerQuery))
	tq := strings.ToLower(strings.TrimSpace(f.TagQuery))
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
	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			if theme.TabButton(c, "1. 精彩片段库 (Highlight Library)", v.activeTab == "library").Clicked() {
				v.activeTab = "library"
			}
			if theme.TabButton(c, fmt.Sprintf("2. 合辑创作工作台 (%d 段)", len(v.montageItems)), v.activeTab == "montage").Clicked() {
				v.activeTab = "montage"
			}
			ui.Spacer(c)
			if v.statusMsg != "" {
				theme.MutedText(c, v.statusMsg)
			}
		})

		if v.activeTab == "library" {
			v.renderLibrary(c)
		} else {
			v.renderWorkbench(c)
		}
	})
}

func (v *View) renderLibrary(c *ui.Context) {
	theme.Card(c, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.TextInput(c, &v.filter.PlayerQuery).Width(180).Placeholder("按选手筛选")
			ui.Select(c, &v.filter.ClipType, []string{"全部类型", "multi_kill", "ace", "clutch", "opening_kill", "utility"})
			ui.TextInput(c, &v.filter.TagQuery).Width(180).Placeholder("按标签筛选 (如 4K, 1v3)")
			ui.Checkbox(c, &v.filter.RenderedOnly, "仅看已生成视频")
			ui.Checkbox(c, &v.filter.FavoritesOnly, "仅看收藏")
		})
	})

	filtered := FilterClips(v.clips, v.filter)
	ui.Row(c).FillWidth().Grow(1).MinHeight(0).Gap(12).Children(func() {
		ui.Column(c).Width(420).FillHeight().MinHeight(0).Children(func() {
			ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					if len(filtered) == 0 {
						theme.Card(c, func() {
							theme.MutedText(c, "暂无符合条件的高光片段")
						})
						return
					}
					for _, clip := range filtered {
						v.renderClipCard(c, clip)
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
	card := theme.Card(c, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.ScoreText(c, fmt.Sprintf("R%d", clip.RoundNumber)).FontSize(14).TextColor(theme.AccentAmber)
			ui.Text(c, clip.PlayerName).FontSize(14).Bold().TextColor(theme.TextPrimary)
			theme.MutedText(c, theme.ClipTypeLabel(clip.ClipType))
			ui.Spacer(c)
			if ready {
				theme.Capsule(c, "已生成 MP4", theme.StatusGreen)
			} else {
				theme.Capsule(c, "未渲染", theme.StatusGray)
			}
		})
		tags := theme.ParseTags(clip.TagsJSON)
		if len(tags) > 0 {
			ui.ScrollHorizontal(c).FillWidth().Children(func() {
				theme.RowCenter(c).Gap(6).Children(func() {
					for _, tag := range tags {
						ui.Box(c).Shrink(0).Background(theme.BgInput).Radius(10).PaddingX(9).PaddingY(3).Children(func() {
							ui.Text(c, tag).Font("Rajdhani").FontSize(11).Bold().TextColor(theme.AccentCyan)
						})
					}
				})
			})
		}
		theme.RowCenter(c).Gap(6).Children(func() {
			if theme.ActionButton(c, "预览/裁剪").Clicked() {
				v.selectedClipID = clip.ID
				if ready {
					if mv, ok := v.videosByID[*clip.MediaVideoID]; ok {
						v.playerWidget.Load(mv.FilePath, mv.DurationSec, clip.StartTick, clip.EndTick, 64, 0)
					}
				} else {
					v.playerWidget.Close()
				}
			}
			if ready {
				if theme.AccentButton(c, "加入合辑").Clicked() {
					if mv, ok := v.videosByID[*clip.MediaVideoID]; ok {
						rc := v.radarBySteam[clip.PlayerSteamID64]
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
						v.statusMsg = "已加入合辑工作台: " + clip.PlayerName
					}
				}
			} else {
				if theme.ActionButton(c, "加入渲染队列").Clicked() {
					v.enqueueClip(clip)
				}
				if theme.AccentButton(c, "立即渲染").Clicked() {
					v.enqueueClip(clip)
					if v.onStartRender != nil {
						v.onStartRender()
					}
				}
			}
		})
	})
	if clip.ID == v.selectedClipID {
		_ = card.Border(1, theme.AccentAmber)
	}
}

func (v *View) renderClipPreview(c *ui.Context) {
	var selected *storage.HighlightClip
	for i := range v.clips {
		if v.clips[i].ID == v.selectedClipID {
			selected = &v.clips[i]
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
										v.montageItems[idx-1], v.montageItems[idx] = v.montageItems[idx], v.montageItems[idx-1]
									}
									if idx < len(v.montageItems)-1 && theme.ActionButton(c, "下移").Clicked() {
										v.montageItems[idx+1], v.montageItems[idx] = v.montageItems[idx], v.montageItems[idx+1]
									}
									if theme.DangerButton(c, "移除").Clicked() {
										v.montageItems = append(v.montageItems[:idx], v.montageItems[idx+1:]...)
									}
								})
								theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
									ui.Checkbox(c, &it.InsertRadar, "在片段前插入选手六维战力雷达图动画")
									ui.Spacer(c)
									ui.TextInput(c, &it.Subtitle).Width(220).Placeholder("自定义字幕条...")
								})
							})
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
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					theme.MutedText(c, "转场效果").Width(64)
					ui.Select(c, &v.transition, []string{"fade", "fadewhite", "wipeleft", "smoothleft", "cut"}).Width(180)
				})
				ui.Checkbox(c, &v.showNameCard, "开启左下角选手信息卡")
				ui.TextInput(c, &v.outputPath).FillWidth().Placeholder("导出路径 (留空自动生成)")
				if v.exportStatus != "" {
					theme.Capsule(c, fmt.Sprintf("%s %.0f%%", v.exportStatus, v.exportProgress*100), theme.AccentCyan)
				}
				if theme.AccentButton(c, "导出合辑 MP4").Clicked() && len(v.montageItems) > 0 && v.agent != nil {
					spec := BuildMontageSpec(v.demoID, v.title, v.introPath, v.outroPath, v.bgmPath, v.outputPath,
						v.clipVol, v.bgmVol, v.transition, v.transitionDur, v.showNameCard, v.montageItems)
					var res map[string]any
					if err := v.agent.Call(context.Background(), "montage.export", map[string]any{"spec": spec}, &res); err != nil {
						v.statusMsg = "导出启动失败: " + err.Error()
					} else {
						v.exportStatus = "running"
						v.exportProgress = 0.05
						v.statusMsg = "后台 FFmpeg 正在合成合辑..."
					}
				}
			})
		})
	})
}
