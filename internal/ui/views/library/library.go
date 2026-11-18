// Package library implements View 1: Match Library for browsing, filtering,
// annotating, and manually importing CS2 demos.
package library

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
)

// NavTarget identifies a destination view when clicking a match card quick action.
type NavTarget string

const (
	NavSpectator  NavTarget = "spectator"
	NavAnalysis   NavTarget = "analysis"
	NavHighlights NavTarget = "highlights"
	NavSettings   NavTarget = "settings"
)

// AgentCaller abstracts JSON-RPC calls to strikeye-agent.
type AgentCaller interface {
	Call(ctx context.Context, method string, params any, result any) error
}

// FilterCriteria holds all Match Library toolbar filter values.
type FilterCriteria struct {
	Keyword       string
	Map           string
	Platform      string
	ParseStatus   string
	FavoritesOnly bool
}

// MatchEntry pairs a Demo row with pre-fetched player names and video counts for fast filtering.
type MatchEntry struct {
	Demo           storage.Demo
	PlayerNames    []string
	HighlightCount int
	POVCount       int
}

// ParseProgressInfo holds live parse progress for a demo card.
type ParseProgressInfo struct {
	Stage    string
	Progress float64
}

// View manages Match Library state and rendering.
type View struct {
	mu               sync.RWMutex
	db               *storage.DB
	agent            AgentCaller
	dataRoot         string
	entries          []MatchEntry
	parseProgress    map[int64]ParseProgressInfo
	filter           FilterCriteria
	manualPath       string
	showManualImport bool
	editingID        int64
	remarkDraft      string
	statusMsg        string
	onNavigate       func(target NavTarget, demoID int64)
}

// NewView constructs a Match Library view.
func NewView(db *storage.DB, agent AgentCaller, dataRoot string, onNavigate func(NavTarget, int64)) *View {
	return &View{
		db:            db,
		agent:         agent,
		dataRoot:      dataRoot,
		parseProgress: make(map[int64]ParseProgressInfo),
		onNavigate:    onNavigate,
		filter: FilterCriteria{
			Map:         "全部地图",
			Platform:    "全部平台",
			ParseStatus: "全部状态",
		},
	}
}

// SetParseProgress updates live parse progress for demoID.
func (v *View) SetParseProgress(demoID int64, stage string, progress float64) {
	if demoID <= 0 {
		return
	}
	v.mu.Lock()
	if v.parseProgress == nil {
		v.parseProgress = make(map[int64]ParseProgressInfo)
	}
	v.parseProgress[demoID] = ParseProgressInfo{Stage: stage, Progress: progress}
	v.mu.Unlock()
}

// ClearParseProgress removes live parse progress state for demoID.
func (v *View) ClearParseProgress(demoID int64) {
	v.mu.Lock()
	delete(v.parseProgress, demoID)
	v.mu.Unlock()
}

// Refresh reloads demos, player names, and media counts from SQLite WAL.
func (v *View) Refresh(ctx context.Context) error {
	if v.db == nil {
		return nil
	}
	demos, err := v.db.Demos().List(ctx)
	if err != nil {
		return err
	}
	entries := make([]MatchEntry, 0, len(demos))
	for _, d := range demos {
		videos, _ := v.db.Media().ListByDemo(ctx, d.ID)
		hlCount, povCount := 0, 0
		for _, mv := range videos {
			if mv.VideoScope == "highlight_clip" || mv.VideoScope == "highlight" {
				hlCount++
			} else if mv.VideoScope == "round_pov" {
				povCount++
			}
		}
		players, _ := v.db.Analysis().ListMatchPlayers(ctx, d.ID)
		names := make([]string, 0, len(players))
		for _, p := range players {
			names = append(names, p.PlayerName)
		}
		entries = append(entries, MatchEntry{
			Demo:           d,
			PlayerNames:    names,
			HighlightCount: hlCount,
			POVCount:       povCount,
		})
	}
	v.entries = entries
	return nil
}

// FilteredEntries applies FilterCriteria to v.entries.
func (v *View) FilteredEntries() []MatchEntry {
	return FilterMatches(v.entries, v.filter)
}

// FilterMatches filters MatchEntry items purely in-memory for deterministic UI updates & testing.
func FilterMatches(entries []MatchEntry, f FilterCriteria) []MatchEntry {
	kw := strings.ToLower(strings.TrimSpace(f.Keyword))
	out := make([]MatchEntry, 0, len(entries))
	for _, e := range entries {
		d := e.Demo
		if f.FavoritesOnly && !d.IsFavorite {
			continue
		}
		mapName := derefStr(d.MapName)
		if f.Map != "" && f.Map != "全部地图" && !strings.EqualFold(mapName, f.Map) {
			continue
		}
		if f.Platform != "" && f.Platform != "全部平台" && !strings.EqualFold(d.SourcePlatform, f.Platform) {
			continue
		}
		if f.ParseStatus != "" && f.ParseStatus != "全部状态" && !strings.EqualFold(d.ParseStatus, f.ParseStatus) {
			continue
		}
		if kw != "" {
			matched := strings.Contains(strings.ToLower(d.Filename), kw) ||
				strings.Contains(strings.ToLower(mapName), kw) ||
				strings.Contains(strings.ToLower(derefStr(d.Remark)), kw) ||
				strings.Contains(strings.ToLower(derefStr(d.DisplayName)), kw) ||
				strings.Contains(strings.ToLower(d.TeamAName+" "+d.TeamBName), kw)
			if !matched {
				for _, pn := range e.PlayerNames {
					if strings.Contains(strings.ToLower(pn), kw) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func (v *View) mapOptions() []string {
	seen := map[string]bool{}
	opts := []string{"全部地图"}
	for _, e := range v.entries {
		m := derefStr(e.Demo.MapName)
		if m != "" && !seen[m] {
			seen[m] = true
			opts = append(opts, m)
		}
	}
	return opts
}

// Render draws the Match Library toolbar and scrollable match cards.
func (v *View) Render(c *ui.Context) {
	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		// Top toolbar
		theme.Card(c, func() {
			theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
				ui.TextInput(c, &v.filter.Keyword).Width(240).Placeholder("搜索地图 / 选手 / 备注 / 文件名")
				ui.Select(c, &v.filter.Map, v.mapOptions())
				ui.Select(c, &v.filter.Platform, []string{"全部平台", "valve", "faceit", "5e", "perfectworld", "custom"})
				ui.Select(c, &v.filter.ParseStatus, []string{"全部状态", "ready", "parsing", "pending", "failed"})
				ui.Checkbox(c, &v.filter.FavoritesOnly, "仅看收藏")
				ui.Spacer(c)
				if theme.ActionButton(c, "+ 手动导入 Demo").Clicked() {
					v.showManualImport = !v.showManualImport
				}
				if theme.ActionButton(c, "刷新").Clicked() {
					if v.agent != nil {
						var res map[string]any
						_ = v.agent.Call(context.Background(), "demo.scan", nil, &res)
					}
					_ = v.Refresh(context.Background())
				}
				if theme.ActionButton(c, "配置监听目录").Clicked() && v.onNavigate != nil {
					v.onNavigate(NavSettings, 0)
				}
			})
			if v.showManualImport {
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					ui.TextInput(c, &v.manualPath).FillWidth().Placeholder("输入本地 .dem 文件绝对路径以手动导入并解析")
					if theme.AccentButton(c, "导入并解析").Clicked() && strings.TrimSpace(v.manualPath) != "" {
						if v.agent != nil {
							var res map[string]any
							if err := v.agent.Call(context.Background(), "demo.parse", map[string]any{
								"file_path": strings.TrimSpace(v.manualPath),
							}, &res); err != nil {
								v.statusMsg = "导入失败: " + err.Error()
							} else {
								v.statusMsg = "已提交解析: " + strings.TrimSpace(v.manualPath)
								v.manualPath = ""
								v.showManualImport = false
								_ = v.Refresh(context.Background())
							}
						}
					}
				})
			}
			if v.statusMsg != "" {
				theme.MutedText(c, v.statusMsg)
			}
		})

		// Match card list
		filtered := v.FilteredEntries()
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				if len(filtered) == 0 {
					theme.Card(c, func() {
						ui.Column(c).FillWidth().Height(140).Center().Gap(6).Children(func() {
							theme.SectionTitle(c, "暂无符合条件的比赛 Demo")
							theme.MutedText(c, "请将 .dem 文件放入监听目录，或在上方输入本地路径手动添加。")
						})
					})
					return
				}
				for _, e := range filtered {
					v.renderMatchCard(c, e)
				}
			})
		})
	})
}

func (v *View) renderMatchCard(c *ui.Context, e MatchEntry) {
	d := e.Demo
	mapLabel := derefStr(d.MapName)
	if mapLabel == "" {
		mapLabel = "未知地图"
	}
	title := d.Filename
	if d.DisplayName != nil && *d.DisplayName != "" {
		title = *d.DisplayName
	}
	teamA := d.TeamAName
	if teamA == "" {
		teamA = "TEAM CT"
	}
	teamB := d.TeamBName
	if teamB == "" {
		teamB = "TEAM T"
	}

	theme.Card(c, func() {
		// Header metadata row
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.Capsule(c, strings.ToUpper(mapLabel), theme.AccentAmber)
			ui.Text(c, title).FontSize(14).Bold().TextColor(theme.TextPrimary)
			theme.StatText(c, fmt.Sprintf("%s  •  %.0fs  •  %d 回合", strings.ToUpper(d.SourcePlatform), d.DurationSec, d.TotalRounds)).
				FontSize(12).TextColor(theme.TextMuted)
			ui.Spacer(c)
			if d.IsFavorite {
				theme.Capsule(c, "已收藏", theme.AccentAmber)
			}
			theme.Capsule(c, theme.ParseStatusLabel(d.ParseStatus), theme.ParseStatusColor(d.ParseStatus))
			videoBadge := "未生成视频"
			videoCol := theme.StatusGray
			if e.HighlightCount > 0 || e.POVCount > 0 {
				videoBadge = fmt.Sprintf("高光 %d / POV %d", e.HighlightCount, e.POVCount)
				videoCol = theme.StatusGreen
			} else if d.RenderStatus == "queued" || d.RenderStatus == "rendering" {
				videoBadge = "排队渲染中"
				videoCol = theme.StatusYellow
			}
			theme.Capsule(c, videoBadge, videoCol)
		})

		// Scoreline hero strip
		theme.RowCenter(c).FillWidth().Background(theme.BgInput).Radius(10).PaddingX(16).PaddingY(12).Gap(14).Children(func() {
			theme.RowCenter(c).Gap(12).Children(func() {
				ui.Text(c, teamA).FontSize(14).Bold().TextColor(theme.TeamCT)
				theme.RowCenter(c).Background(theme.BgSurface).Radius(10).PaddingX(12).PaddingY(4).Gap(8).Children(func() {
					theme.ScoreText(c, fmt.Sprintf("%d", d.TeamAScore)).FontSize(24).TextColor(theme.TextPrimary)
					theme.ScoreText(c, ":").FontSize(18).TextColor(theme.TextMuted)
					theme.ScoreText(c, fmt.Sprintf("%d", d.TeamBScore)).FontSize(24).TextColor(theme.TextPrimary)
				})
				ui.Text(c, teamB).FontSize(14).Bold().TextColor(theme.TeamT)
			})
			ui.Spacer(c)
			if d.Remark != nil && *d.Remark != "" {
				theme.RowCenter(c).Background(theme.BgSurface).Radius(4).PaddingX(10).PaddingY(4).Gap(6).Children(func() {
					theme.StatusDot(c, theme.AccentCyan, 5)
					theme.MutedText(c, *d.Remark)
				})
			}
		})

		if v.editingID == d.ID {
			theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
				ui.TextInput(c, &v.remarkDraft).FillWidth().Placeholder("输入比赛备注...")
				if theme.AccentButton(c, "保存备注").Clicked() {
					if v.db != nil {
						rm := v.remarkDraft
						_ = v.db.Demos().UpdateAnnotations(context.Background(), d.ID, d.DisplayName, &rm, d.IsFavorite)
						_ = v.Refresh(context.Background())
					}
					v.editingID = 0
				}
				if theme.ActionButton(c, "取消").Clicked() {
					v.editingID = 0
				}
			})
		}

		if d.ParseStatus == "parsing" {
			v.mu.RLock()
			progInfo, hasProg := v.parseProgress[d.ID]
			v.mu.RUnlock()
			progVal := -1.0
			stage := "正在解析 Demo..."
			if hasProg {
				progVal = progInfo.Progress
				if progInfo.Stage != "" {
					stage = progInfo.Stage
				}
			}
			theme.ProgressBar(c, progVal, stage)
		}

		// Action hierarchy row
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			if theme.ActionButton(c, "查看比赛分析").Clicked() && v.onNavigate != nil {
				v.onNavigate(NavAnalysis, d.ID)
			}
			if theme.ActionButton(c, "浏览精彩片段").Clicked() && v.onNavigate != nil {
				v.onNavigate(NavHighlights, d.ID)
			}
			favLabel := "收藏"
			if d.IsFavorite {
				favLabel = "取消收藏"
			}
			if theme.ActionButton(c, favLabel).Clicked() && v.db != nil {
				_ = v.db.Demos().UpdateAnnotations(context.Background(), d.ID, d.DisplayName, d.Remark, !d.IsFavorite)
				_ = v.Refresh(context.Background())
			}
			if theme.ActionButton(c, "编辑备注").Clicked() {
				v.editingID = d.ID
				v.remarkDraft = derefStr(d.Remark)
			}
			ui.Spacer(c)
			if theme.AccentButton(c, "进入独立观战").Clicked() && v.onNavigate != nil {
				v.onNavigate(NavSpectator, d.ID)
			}
		})
	})
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
