// Package library implements View 1: Match Library for browsing, filtering,
// annotating, and manually importing CS2 demos.
package library

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/config"
	"strikeye/internal/gcpd"
	"strikeye/internal/gsi"
	"strikeye/internal/launcher"
	"strikeye/internal/series"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
	"strikeye/internal/watcher"
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
	GroupBySeries bool
	// ExtraDemoIDs extends the keyword match with full-text index hits (annotation
	// notes, team names, players of demos whose cards would not otherwise match).
	ExtraDemoIDs map[int64]bool
}

// MatchEntry pairs a Demo row with pre-fetched player names and video counts for fast filtering.
type MatchEntry struct {
	Demo        storage.Demo
	PlayerNames []string
	PlayerTags  []string
	// BannedPlayers counts accounts with a cached ban snapshot (VAC, community or
	// game ban). Zero also means "no snapshot yet", which is why the UI only
	// shows a badge for positive counts.
	BannedPlayers  int
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
	mu                sync.RWMutex
	db                *storage.DB
	agent             AgentCaller
	dataRoot          string
	live              gsi.State
	liveOK            bool
	liveChecked       time.Time
	entries           []MatchEntry
	parseProgress     map[int64]ParseProgressInfo
	filter            FilterCriteria
	manualPath        string
	showManualImport  bool
	showSteamDownload bool
	steamCookieDraft  string
	steamTab          string
	steamLimit        int
	steamBusy         bool
	sharecodeDraft    string
	apiKeyDraft       string
	showAPIKeyPrompt  bool
	editingID         int64
	remarkDraft       string
	statusMsg         string
	ftsIDs            map[int64]bool
	ftsQuery          string
	ftsPending        bool
	onNavigate        func(target NavTarget, demoID int64)
}

// NewView constructs a Match Library view.
func NewView(db *storage.DB, agent AgentCaller, dataRoot string, onNavigate func(NavTarget, int64)) *View {
	return &View{
		db:            db,
		agent:         agent,
		dataRoot:      dataRoot,
		parseProgress: make(map[int64]ParseProgressInfo),
		steamTab:      gcpd.Tabs[0],
		steamLimit:    3,
		onNavigate:    onNavigate,
		filter: FilterCriteria{
			Map:         "全部地图",
			Platform:    allPlatformsOption,
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
		ids := make([]string, 0, len(players))
		for _, p := range players {
			names = append(names, p.PlayerName)
			if p.SteamID64 != "" {
				ids = append(ids, p.SteamID64)
			}
		}
		tagMap, _ := v.db.PlayerProfiles().TagsForSteamIDs(ctx, ids)
		tags := make([]string, 0, 4)
		seenTag := make(map[string]bool, 4)
		for _, p := range players {
			for _, t := range tagMap[p.SteamID64] {
				key := strings.ToLower(t)
				if !seenTag[key] {
					seenTag[key] = true
					tags = append(tags, t)
				}
			}
		}
		bannedMap, _ := v.db.PlayerBans().BannedFor(ctx, ids)
		entries = append(entries, MatchEntry{
			Demo:           d,
			PlayerNames:    names,
			PlayerTags:     tags,
			BannedPlayers:  len(bannedMap),
			HighlightCount: hlCount,
			POVCount:       povCount,
		})
	}
	v.mu.Lock()
	v.entries = entries
	v.mu.Unlock()
	return nil
}

// FilteredEntries applies FilterCriteria to v.entries.
func (v *View) FilteredEntries() []MatchEntry {
	v.mu.RLock()
	entries := v.entries
	filter := v.filter
	kw := strings.TrimSpace(filter.Keyword)
	synced := kw != "" && v.ftsQuery == kw
	ids := v.ftsIDs
	v.mu.RUnlock()
	if kw != "" {
		v.ensureFullTextSearch(kw)
	}
	if synced {
		filter.ExtraDemoIDs = ids
	}
	return FilterMatches(entries, filter)
}

// ensureFullTextSearch refreshes the FTS hit set for a keyword in the
// background. It is a no-op while a query for the same keyword is in flight,
// so calling it from the render loop is cheap.
func (v *View) ensureFullTextSearch(keyword string) {
	v.mu.Lock()
	if v.db == nil || v.ftsPending || v.ftsQuery == keyword {
		v.mu.Unlock()
		return
	}
	v.ftsPending = true
	db := v.db
	v.mu.Unlock()

	go func() {
		ctx := context.Background()
		ids := make(map[int64]bool, 16)
		if _, err := db.Search().EnsureSearchIndex(ctx); err == nil {
			if hits, err := db.Search().Query(ctx, keyword, 500); err == nil {
				for _, hit := range hits {
					ids[hit.DemoID] = true
				}
			}
		}
		v.mu.Lock()
		v.ftsIDs = ids
		v.ftsQuery = keyword
		v.ftsPending = false
		v.mu.Unlock()
	}()
}

// groupFiltered reorders filtered entries so each multi-map series is
// contiguous, and returns the series lookup plus visible map counts per key.
func groupFiltered(filtered []MatchEntry) ([]MatchEntry, map[int64]series.Series, map[string]int) {
	demos := make([]storage.Demo, 0, len(filtered))
	for _, e := range filtered {
		demos = append(demos, e.Demo)
	}
	groups := series.Group(demos)
	byID := series.ByDemoID(groups)
	visible := make(map[string]int, len(groups))
	for _, e := range filtered {
		if s, ok := byID[e.Demo.ID]; ok {
			visible[s.Key]++
		}
	}
	ordered := make([]MatchEntry, 0, len(filtered))
	for _, g := range groups {
		for _, e := range filtered {
			if s, ok := byID[e.Demo.ID]; ok && s.Key == g.Key {
				ordered = append(ordered, e)
			}
		}
	}
	for _, e := range filtered {
		if _, ok := byID[e.Demo.ID]; !ok {
			ordered = append(ordered, e)
		}
	}
	return ordered, byID, visible
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
		if f.Platform != "" && f.Platform != allPlatformsOption && !platformMatches(d.SourcePlatform, f.Platform) {
			continue
		}
		if f.ParseStatus != "" && f.ParseStatus != "全部状态" && !strings.EqualFold(d.ParseStatus, f.ParseStatus) {
			continue
		}
		if kw != "" && !f.ExtraDemoIDs[d.ID] {
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
				if !matched {
					for _, tag := range e.PlayerTags {
						if strings.Contains(strings.ToLower(tag), kw) {
							matched = true
							break
						}
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
	v.mu.RLock()
	entries := v.entries
	v.mu.RUnlock()
	seen := map[string]bool{}
	opts := []string{"全部地图"}
	for _, e := range entries {
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
	v.refreshLiveState()
	ui.Column(c).Fill().Padding(14).Gap(14).Children(func() {
		if v.liveOK {
			theme.GlassCard(c, func() {
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					theme.StatusDot(c, theme.StatusGreen, 8)
					ui.Text(c, "GSI 直播").Font("Rajdhani").FontSize(12).Bold().TextColor(theme.StatusGreen)
					ui.Text(c, liveHeadline(v.live)).FontSize(12).TextColor(theme.TextPrimary)
					ui.Spacer(c)
					theme.MutedText(c, "来自 strikeye-cli gsi")
				})
			})
		}
		// Floating glass filter capsule bar (2-row layout so action buttons never clip at narrower widths)
		theme.GlassCard(c, func() {
			theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
				theme.RowCenter(c).Grow(1).MinWidth(180).Background(theme.BgInput).Border(1, theme.RimBorder).Radius(12).PaddingX(10).Gap(6).Children(func() {
					theme.RenderIcon(c, theme.IconSearch, 14, theme.TextMuted)
					ui.TextInput(c, &v.filter.Keyword).FillWidth().Placeholder("搜索地图 / 选手 / 备注 / 文件名")
				})
				if theme.IconActionButton(c, theme.IconPlus, "+ 手动导入 Demo").Shrink(0).Clicked() {
					v.showManualImport = !v.showManualImport
				}
				if theme.IconActionButton(c, theme.IconShield, "刷新封禁").Shrink(0).Clicked() {
					if loadSteamAPIKey(v.dataRoot) == "" {
						v.showAPIKeyPrompt = true
						v.statusMsg = "封禁查询需要 Steam Web API Key"
					} else {
						v.statusMsg = "正在查询封禁状态..."
						go v.runBanRefreshNow()
					}
				}
				if theme.IconActionButton(c, theme.IconDownload, "从 Steam 下载").Shrink(0).Clicked() {
					v.showSteamDownload = !v.showSteamDownload
					if v.showSteamDownload && v.steamCookieDraft == "" {
						v.steamCookieDraft = loadSteamCookie(v.dataRoot)
					}
				}
				if theme.IconActionButton(c, theme.IconRefresh, "刷新").Shrink(0).Clicked() {
					v.scanAndRefresh(context.Background())
				}
				if theme.IconActionButton(c, theme.IconFolder, "配置监听目录").Shrink(0).Clicked() && v.onNavigate != nil {
					v.onNavigate(NavSettings, 0)
				}
			})
			theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
				ui.Select(c, &v.filter.Map, v.mapOptions())
				ui.Select(c, &v.filter.Platform, platformFilterOptions())
				ui.Select(c, &v.filter.ParseStatus, []string{"全部状态", "parsed", "parsing", "pending", "failed"})
				ui.Checkbox(c, &v.filter.GroupBySeries, "按系列赛分组")
				ui.Checkbox(c, &v.filter.FavoritesOnly, "仅看收藏")
				ui.Spacer(c)
			})
			v.renderAPIKeyPanel(c)
			v.renderSteamDownload(c)
			if v.showManualImport {
				importRow := theme.RowCenter(c).FillWidth().Gap(8).
					Transition(ui.ElementTransition{
						Duration:     200 * time.Millisecond,
						Ease:         ui.EaseOut,
						ExitDuration: 140 * time.Millisecond,
						Position:     true,
						Enter:        &ui.Motion{Collapse: true, Opacity: 0},
						Exit:         &ui.Motion{Collapse: true, Opacity: 0},
					})
				importRow.Children(func() {
					ui.TextInput(c, &v.manualPath).FillWidth().Placeholder("输入本地 .dem 文件绝对路径以手动导入并解析")
					if theme.IconAccentButton(c, theme.IconPlus, "导入并解析").Clicked() && strings.TrimSpace(v.manualPath) != "" {
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
		grouped := make(map[int64]series.Series)
		visible := make(map[string]int)
		if v.filter.GroupBySeries {
			filtered, grouped, visible = groupFiltered(filtered)
		}
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				if len(filtered) == 0 {
					theme.EmptyStateCard(c, theme.IconLibrary, "暂无符合条件的比赛 Demo", "请将 .dem 文件放入监听目录，或在上方输入本地路径手动添加。")
					return
				}
				seenSeries := make(map[string]bool)
				for _, e := range filtered {
					if s, ok := grouped[e.Demo.ID]; ok && visible[s.Key] >= 2 && !seenSeries[s.Key] {
						seenSeries[s.Key] = true
						renderSeriesHeader(c, s)
					}
					// Keyed wrappers let newly filtered cards fade in and moving
					// cards glide to their new slots without rebuilding the view.
					ui.Column(c.Key(e.Demo.ID)).FillWidth().
						Transition(ui.ElementTransition{
							Duration:     200 * time.Millisecond,
							Ease:         ui.EaseOut,
							ExitDuration: 140 * time.Millisecond,
							Position:     true,
							Enter:        &ui.Motion{Y: 10, Opacity: 0},
							Exit:         &ui.Motion{Y: -6, Opacity: 0},
						}).
						Children(func() {
							v.renderMatchCard(c, e)
						})
				}
			})
		})
	})
}

func renderSeriesHeader(c *ui.Context, s series.Series) {
	theme.RowCenter(c).FillWidth().Gap(8).PaddingX(4).Children(func() {
		theme.PillBadge(c, "系列赛", theme.AccentCyan)
		ui.Text(c, s.Label()).FontSize(13).Bold().TextColor(theme.TextPrimary)
		theme.MutedText(c, fmt.Sprintf("%d 图 · %s", len(s.Maps), s.Date))
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

	card := theme.BentoCard(c, ui.Color{}, 16, func() {
		// Header metadata row (map badge removed; map identity lives in the scoreline hero banner)
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, title).FontSize(14).Bold().TextColor(theme.TextPrimary)
			theme.StatText(c, fmt.Sprintf("%s  •  %.0fs  •  %d 回合", strings.ToUpper(d.SourcePlatform), d.DurationSec, d.TotalRounds)).
				FontSize(12).TextColor(theme.TextMuted)
			ui.Spacer(c)
			if d.IsFavorite {
				theme.PillBadge(c, "已收藏", theme.AccentAmber)
			}
			for i, tag := range e.PlayerTags {
				if i >= 3 {
					break
				}
				theme.PillBadge(c, tag, theme.StatusYellow)
			}
			if e.BannedPlayers > 0 {
				theme.PillBadge(c, fmt.Sprintf("封禁 %d", e.BannedPlayers), theme.StatusRed)
			}
			theme.PillBadge(c, theme.ParseStatusLabel(d.ParseStatus), theme.ParseStatusColor(d.ParseStatus))
			videoBadge := "未生成视频"
			videoCol := theme.StatusGray
			if e.HighlightCount > 0 || e.POVCount > 0 {
				videoBadge = fmt.Sprintf("高光 %d / POV %d", e.HighlightCount, e.POVCount)
				videoCol = theme.StatusGreen
			} else if d.RenderStatus == "queued" || d.RenderStatus == "rendering" {
				videoBadge = "排队渲染中"
				videoCol = theme.StatusYellow
			}
			theme.PillBadge(c, videoBadge, videoCol)
		})

		// Bento Scoreline Hero Strip with official CS2 in-game map scene & official map logo
		var rightSlot func()
		if d.Remark != nil && *d.Remark != "" {
			remark := *d.Remark
			rightSlot = func() {
				theme.PillBadge(c, remark, theme.AccentCyan)
			}
		}
		theme.RenderMapScoreBanner(c, mapLabel, teamA, teamB, d.TeamAScore, d.TeamBScore, rightSlot)

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
			if theme.IconActionButton(c, theme.IconAnalysis, "查看比赛分析").Clicked() && v.onNavigate != nil {
				v.onNavigate(NavAnalysis, d.ID)
			}
			if theme.IconActionButton(c, theme.IconHighlights, "浏览精彩片段").Clicked() && v.onNavigate != nil {
				v.onNavigate(NavHighlights, d.ID)
			}
			favLabel := "收藏"
			if d.IsFavorite {
				favLabel = "取消收藏"
			}
			if theme.IconActionButton(c, theme.IconStar, favLabel).Clicked() && v.db != nil {
				_ = v.db.Demos().UpdateAnnotations(context.Background(), d.ID, d.DisplayName, d.Remark, !d.IsFavorite)
				_ = v.Refresh(context.Background())
			}
			if theme.IconActionButton(c, theme.IconSliders, "编辑备注").Clicked() {
				v.editingID = d.ID
				v.remarkDraft = derefStr(d.Remark)
			}
			if theme.IconActionButton(c, theme.IconDownload, "导出数据").Clicked() {
				dir, count, err := exportDemo(context.Background(), v.db, v.dataRoot, d.ID)
				if err != nil {
					v.statusMsg = "导出失败: " + err.Error()
				} else {
					v.statusMsg = fmt.Sprintf("已导出 %d 个文件到 %s", count, dir)
				}
			}
			if theme.IconActionButton(c, theme.IconPlay, "用 CS2 打开").Clicked() {
				if err := openDemoInCS2(d, launcher.Open); err != nil {
					v.statusMsg = "启动 CS2 失败: " + err.Error()
				} else {
					v.statusMsg = "已请求 CS2 载入该 demo（Steam 需在运行）"
				}
			}
			ui.Spacer(c)
			if theme.IconAccentButton(c, theme.IconSpectator, "进入独立观战").Clicked() && v.onNavigate != nil {
				v.onNavigate(NavSpectator, d.ID)
			}
		})
	})
	theme.HoverLift(card, 3)
}

func (v *View) scanAndRefresh(ctx context.Context) {
	var rep watcher.ScanReport
	scannedViaAgent := false
	if v.agent != nil {
		if err := v.agent.Call(ctx, "demo.scan", nil, &rep); err == nil {
			scannedViaAgent = true
		}
	}
	if !scannedViaAgent && v.db != nil && v.dataRoot != "" {
		cfg, _, err := config.Load(v.dataRoot)
		if err == nil {
			probe := config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths})
			targets := watcher.DiscoverWatchTargets(watcher.DiscoverOptions{
				Config: cfg,
				Probe:  probe,
			})
			w := watcher.New(v.db, watcher.Options{
				Targets:   targets,
				ImportDir: filepath.Join(v.dataRoot, "media", "imports"),
			})
			if localRep, scanErr := w.ScanNowWithReport(ctx); scanErr == nil {
				rep = localRep
			}
		}
	}
	_ = v.Refresh(ctx)
	v.mu.RLock()
	nEntries := len(v.entries)
	v.mu.RUnlock()
	v.statusMsg = fmt.Sprintf("已扫描 %d 个监听目录，发现 %d 个 .dem（新增 %d，当前共 %d 场）",
		rep.Targets, rep.Candidates, rep.Discovered, nEntries)
	if len(rep.Warnings) > 0 {
		v.statusMsg += " | 注意: " + rep.Warnings[0]
	}
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
