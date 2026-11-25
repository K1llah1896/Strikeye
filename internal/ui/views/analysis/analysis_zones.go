package analysis

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/callouts"
	"strikeye/internal/killctx"
	"strikeye/internal/mapcontrol"
	"strikeye/internal/mapgeom"
	"strikeye/internal/nadebook"
	"strikeye/internal/navmesh"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
	"strikeye/internal/ui/components"
	"strikeye/internal/ui/theme"
	"strikeye/internal/zones"
)

// calloutsTablePath is where the bundled callout table (or
// `strikeye-cli callouts-fetch`) lands.
func calloutsTablePath(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "map_data", "callouts.csv")
}

// ensureZonesReadyAsync computes callout-zone summaries off the UI goroutine.
func (v *View) ensureZonesReadyAsync() {
	v.mu.Lock()
	if v.zoneReady || v.zoneLoading || v.demo == nil {
		v.mu.Unlock()
		return
	}
	v.zoneLoading = true
	v.zoneSeq++
	seq := v.zoneSeq
	demo := *v.demo
	players := v.players
	rounds := append([]storage.MatchRound(nil), v.rounds...)
	replayIndex := append([]parser.RoundIndexEntry(nil), v.replayIndex...)
	cached := v.heatmapRounds
	rd := v.replayRound
	tickRate := demo.TickRate
	mapName := ""
	if demo.MapName != nil {
		mapName = *demo.MapName
	}
	cachePath := ""
	if demo.ReplayCachePath != nil {
		cachePath = *demo.ReplayCachePath
	}
	dataRoot := v.dataRoot
	db := v.db
	v.mu.Unlock()
	v.invalidate()

	go func() {
		// The datasets ship inside the package and are staged at startup by the
		// app shell; missing files here mean a source build without blobs.
		zoneErr := ""
		var set *callouts.Set
		table, err := callouts.LoadCSV(calloutsTablePath(dataRoot))
		if err != nil {
			zoneErr = "内置 callout 数据集缺失（源码构建）：运行 `strikeye-cli assets-pack` 后重新构建，或 `strikeye-cli callouts-fetch` 联网补齐。"
		} else if s, err := table.For(mapName); err != nil {
			zoneErr = fmt.Sprintf("callout 数据集暂不含地图 %s（已覆盖 %s）。", mapName, strings.Join(table.Maps(), ", "))
		} else {
			set = s
		}

		replays := cached
		if len(replays) == 0 && cachePath != "" {
			if _, err := os.Stat(cachePath); err != nil {
				name := filepath.Base(strings.ReplaceAll(cachePath, "\\", "/"))
				alt := filepath.Join(dataRoot, "cache", "replay_32hz", name)
				if _, err := os.Stat(alt); err == nil {
					cachePath = alt
				}
			}
		}
		if len(replays) == 0 && cachePath != "" && len(replayIndex) > 0 {
			loaded := make(map[int]parser.ReplayRoundData, len(replayIndex))
			for _, ent := range replayIndex {
				if _, decoded, err := parser.ReadReplayRound(cachePath, ent.RoundNumber); err == nil {
					loaded[int(ent.RoundNumber)] = decoded
				}
			}
			if len(loaded) > 0 {
				replays = loaded
			}
		}
		if len(replays) == 0 && len(rd.Frames) > 0 {
			rn := int(rd.RoundNumber)
			if rn <= 0 {
				rn = 1
			}
			replays = map[int]parser.ReplayRoundData{rn: rd}
		}

		slotNames := make(map[uint8]string, len(players))
		for _, p := range players {
			if p.SpecSlot != nil && *p.SpecSlot >= 0 && *p.SpecSlot <= 255 {
				slotNames[uint8(*p.SpecSlot)] = p.PlayerName
			}
		}
		totals := zones.Setup(replays, rounds, tickRate, set)
		var anns []storage.MapAnnotation
		if db != nil {
			if rows, err := db.MapAnnotations().ListByDemo(context.Background(), demo.ID); err == nil {
				anns = rows
			}
		}
		pins := pinsFromAnnotations(set, anns)
		deaths := zones.Deaths(replays, rounds, tickRate, set, slotNames)
		sort.Slice(deaths, func(i, j int) bool {
			if deaths[i].Round != deaths[j].Round {
				return deaths[i].Round < deaths[j].Round
			}
			return deaths[i].Tick < deaths[j].Tick
		})

		killCtx := map[string]killctx.Kill{}
		{
			var bvh *mapgeom.BVH
			if mapName != "" {
				if loaded, ok := mapgeom.LoadForMap(dataRoot, mapName); ok {
					bvh = loaded
				}
			}
			for _, k := range killctx.Build(replays, rounds, players, tickRate, bvh, killctx.DefaultConfig()).Kills {
				killCtx[fmt.Sprintf("%d:%d", k.Round, k.Tick)] = k
			}
		}

		lineups := nadebook.Build(replays, rounds, players, demo.TeamAName, tickRate, set, nadebook.DefaultConfig()).Lineups

		var control mapcontrol.Result
		controlErr := ""
		if set != nil {
			if mesh, err := navmesh.Load(filepath.Join(dataRoot, "cache", "map_nav", mapName+".nav")); err != nil {
				controlErr = fmt.Sprintf("内置包不含 %s 的导航网格：可运行 `strikeye-cli navs-fetch --map %s` 联网补齐。", mapName, mapName)
			} else {
				control = mapcontrol.Build(replays, rounds, mesh, set, tickRate, mapcontrol.DefaultConfig())
			}
		}

		v.mu.Lock()
		if v.zoneSeq != seq {
			v.mu.Unlock()
			return
		}
		v.zoneTotals = totals
		v.zoneDeaths = deaths
		v.zoneKillCtx = killCtx
		v.zoneLineups = lineups
		v.zoneControl = control
		v.zoneControlErr = controlErr
		v.zoneSet = set
		v.zoneAnnotations = anns
		v.zonePins = pins
		if rows, err := db.MapStrokes().List(context.Background(), demo.ID); err == nil {
			v.zoneStrokes = rows
		}
		v.zoneRound = v.replayRoundNum
		v.zoneErr = zoneErr
		v.zoneReady = true
		v.zoneLoading = false
		v.mu.Unlock()
		v.applyPins()
		v.applyStrokes()
		v.invalidate()
	}()
}

// renderZones draws the callout-zone tab: default setups per area and where
// every death happened.
func (v *View) renderZones(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	loading := v.zoneLoading
	zoneErr := v.zoneErr
	totals := v.zoneTotals
	deaths := append([]zones.Death(nil), v.zoneDeaths...)
	killCtx := v.zoneKillCtx
	control := v.zoneControl
	controlErr := v.zoneControlErr
	lineups := append([]nadebook.Lineup(nil), v.zoneLineups...)
	anns := append([]storage.MapAnnotation(nil), v.zoneAnnotations...)
	v.mu.RUnlock()
	pinned := make(map[string]storage.MapAnnotation, len(anns))
	for _, a := range anns {
		pinned[a.CalloutID] = a
	}
	if demo == nil {
		return
	}

	if zoneErr != "" {
		theme.EmptyStateCard(c, theme.IconRadar, "区域数据不可用", zoneErr)
		return
	}
	if loading && len(deaths) == 0 {
		theme.Card(c, func() {
			theme.MutedText(c, "正在解析区域数据（读取 32Hz 回放帧与 callout 数据表）…")
		})
		return
	}

	named := totals.Sampled - totals.Unnamed
	namedPct := 0.0
	if totals.Sampled > 0 {
		namedPct = float64(named) / float64(totals.Sampled) * 100
	}
	theme.RowCenter(c).FillWidth().Gap(12).Children(func() {
		theme.StatTile(c, "采样回合", fmt.Sprintf("%d", totals.Rounds), fmt.Sprintf("每回合开局 %.0f 秒", zones.SetupSampleSeconds), theme.AccentCyan)
		theme.StatTile(c, "位置样本", fmt.Sprintf("%d", totals.Sampled), "仅统计存活选手", theme.AccentAmber)
		theme.StatTile(c, "已命名位置", fmt.Sprintf("%.1f%%", namedPct), fmt.Sprintf("%d 精确 · %d 邻近", totals.Exact, totals.Snapped), theme.StatusGreen)
		theme.StatTile(c, "记录击杀", fmt.Sprintf("%d", len(deaths)), "按倒地位置归属区域", theme.StatusRed)
	})

	theme.Card(c, func() {
		theme.SectionTitle(c, "地图控制曲线（navmesh 可达性模型）")
		theme.MutedText(c, "按导航网格可达性判定每个区域归 T / CT / 争夺；净值为正表示 CT 控制更多。烟雾遮蔽与火焰封路覆盖的区域不计入归属。")
		if controlErr != "" {
			ui.Spacer(c)
			theme.MutedText(c, controlErr)
			return
		}
		buckets, order := controlBuckets(control)
		if len(order) == 0 {
			ui.Spacer(c)
			theme.MutedText(c, "没有可用的控制采样。")
			return
		}
		avg := make([]float64, 0, len(order))
		ctAdvantage := 0
		for _, key := range order {
			vals := buckets[key]
			sum := 0.0
			for _, v := range vals {
				sum += v
			}
			mean := sum / float64(len(vals))
			avg = append(avg, mean)
			if mean > 0 {
				ctAdvantage++
			}
		}
		ui.Spacer(c)
		theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
			ui.Text(c, "T").FontSize(12).Bold().TextColor(theme.TeamT)
			ui.Text(c, sparkline(avg)).Font("Consolas").FontSize(16).TextColor(theme.AccentAmber)
			ui.Text(c, "CT").FontSize(12).Bold().TextColor(theme.TeamCT)
		})
		blockedSum := 0
		for _, f := range control.Frames {
			blockedSum += f.Blocked
		}
		blockedAvg := 0.0
		if len(control.Frames) > 0 {
			blockedAvg = float64(blockedSum) / float64(len(control.Frames))
		}
		theme.MutedText(c, fmt.Sprintf("采样 %d 次 · 回合内平均净值 %+.2f · CT 占优时段 %.0f%% · 烟雾/火焰遮挡平均 %.1f 个区域", len(control.Frames), meanOf(avg), float64(ctAdvantage)/float64(len(avg))*100, blockedAvg))

		top := controlCallouts(control, 8)
		if len(top) == 0 {
			return
		}
		ui.Spacer(c)
		for _, cc := range top {
			bar := controlBar(cc.AvgNet)
			theme.RowCenter(c).FillWidth().Gap(8).PaddingY(1).Children(func() {
				ui.Text(c, cc.Callout).Width(160).FontSize(12).TextColor(theme.TextPrimary)
				ui.Text(c, bar).Font("Consolas").FontSize(12).TextColor(theme.AccentCyan)
				ui.Text(c, fmt.Sprintf("%+.2f", cc.AvgNet)).Width(60).FontSize(12).Bold().TextColor(theme.TextMuted)
			})
		}
	})

	theme.Card(c, func() {
		theme.SectionTitle(c, "道具线位（按投掷点与落点聚类）")
		theme.MutedText(c, "同一方、同一道具、投掷点与落点相近的投掷合并为一条线位；“后续击杀”统计落点附近 8 秒内的击杀。")
		ui.Spacer(c)
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, "道具").Width(90).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "方").Width(40).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "投掷点 → 落点").Width(330).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "次数").Width(50).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "回合").Width(50).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "后续击杀").Width(70).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "回合胜率").Width(70).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "收益").Width(60).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "平均时间").Width(70).FontSize(11).Bold().TextColor(theme.TextMuted)
		})
		if len(lineups) == 0 {
			theme.MutedText(c, "回放缓存中没有可用道具轨迹。")
			return
		}
		limit := len(lineups)
		if limit > 12 {
			limit = 12
		}
		for _, l := range lineups[:limit] {
			from := l.StartCallout
			if from == "" {
				from = "未命名"
			}
			to := l.LandCallout
			if to == "" {
				to = "未命名"
			}
			sideText, sideColor := "T", theme.TeamT
			if l.Side == "ct" {
				sideText, sideColor = "CT", theme.TeamCT
			} else if l.Side == "" {
				sideText, sideColor = "—", theme.TextMuted
			}
			kills := fmt.Sprintf("%d", l.FollowUpKills)
			killsColor := theme.TextMuted
			if l.FollowUpKills > 0 {
				killsColor = theme.StatusRed
			}
			winRate := "—"
			winColor := theme.TextMuted
			impact := "—"
			impactColor := theme.TextMuted
			if l.Rounds > 0 {
				winRate = fmt.Sprintf("%.0f%%", l.WinRate)
				winColor = theme.TextPrimary
				impact = fmt.Sprintf("%+.1f", l.Impact)
				switch {
				case l.Impact >= 10:
					impactColor = theme.StatusGreen
				case l.Impact <= -10:
					impactColor = theme.StatusRed
				default:
					impactColor = theme.TextMuted
				}
			}
			theme.RowCenter(c).FillWidth().Gap(8).PaddingY(2).Children(func() {
				ui.Text(c, l.Type).Width(90).FontSize(12).TextColor(theme.AccentCyan)
				ui.Text(c, sideText).Width(40).FontSize(12).Bold().TextColor(sideColor)
				ui.Text(c, fmt.Sprintf("%s → %s", from, to)).Width(330).FontSize(12).TextColor(theme.TextPrimary)
				ui.Text(c, fmt.Sprintf("%d", l.Throws)).Width(50).FontSize(12).Bold().TextColor(theme.TextPrimary)
				ui.Text(c, fmt.Sprintf("%d", l.Rounds)).Width(50).FontSize(12).TextColor(theme.TextMuted)
				ui.Text(c, kills).Width(70).FontSize(12).TextColor(killsColor)
				ui.Text(c, winRate).Width(70).FontSize(12).TextColor(winColor)
				ui.Text(c, impact).Width(60).FontSize(12).Bold().TextColor(impactColor)
				ui.Text(c, formatClock(l.AvgTimeSec)).Width(70).FontSize(12).TextColor(theme.TextMuted)
			})
		}
		if len(lineups) > limit {
			theme.MutedText(c, fmt.Sprintf("仅显示前 %d 条（共 %d 条）。", limit, len(lineups)))
		}
	})

	rows := totals.Rows()
	theme.Card(c, func() {
		theme.SectionTitle(c, fmt.Sprintf("默认站位分布（开局 %.0f 秒，按区域）", zones.SetupSampleSeconds))
		theme.MutedText(c, "同一回合内生存选手所在的 callout 计数；用于识别对手默认架点与己方站位习惯。点击 ◎ 可在该区域放置/取消标记（同步到 2D 回放的雷达图）。")
		ui.Spacer(c)
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, "区域").Width(200).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "T").Width(50).FontSize(11).Bold().TextColor(theme.TeamT)
			ui.Text(c, "CT").Width(50).FontSize(11).Bold().TextColor(theme.TeamCT)
			ui.Text(c, "合计").Width(50).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "占比").Width(60).FontSize(11).Bold().TextColor(theme.TextMuted)
		})
		if len(rows) == 0 {
			theme.MutedText(c, "没有可用的站位样本。")
			return
		}
		for _, row := range rows {
			share := 0.0
			if totals.Sampled > 0 {
				share = float64(row.Total) / float64(totals.Sampled) * 100
			}
			mark, markColor := "◎", theme.TextMuted
			if _, ok := pinned[row.PlaceID]; ok {
				mark, markColor = "◉", theme.AccentAmber
			}
			placeID := row.PlaceID
			theme.RowCenter(c).FillWidth().Gap(8).PaddingY(2).Children(func() {
				ui.Text(c, mark).Width(18).FontSize(12).TextColor(markColor).OnClick(func() {
					v.toggleAnnotation(placeID)
				})
				ui.Text(c, row.Callout).Width(182).FontSize(12).TextColor(theme.TextPrimary)
				ui.Text(c, fmt.Sprintf("%d", row.T)).Width(50).FontSize(12).TextColor(theme.TeamT)
				ui.Text(c, fmt.Sprintf("%d", row.CT)).Width(50).FontSize(12).TextColor(theme.TeamCT)
				ui.Text(c, fmt.Sprintf("%d", row.Total)).Width(50).FontSize(12).Bold().TextColor(theme.TextPrimary)
				ui.Text(c, fmt.Sprintf("%.0f%%", share)).Width(60).FontSize(12).TextColor(theme.TextMuted)
			})
		}
	})

	theme.Card(c, func() {
		theme.SectionTitle(c, "自由笔刷（2D 回放画板）")
		theme.MutedText(c, "在 2D 回放雷达图上按住鼠标左键拖动即可画线，松手即保存；笔迹按 demo 与回合持久化，切换回合会自动切换笔迹。")
		ui.Spacer(c)
		v.renderStrokeControls(c)
	})

	if len(anns) > 0 {
		theme.Card(c, func() {
			theme.SectionTitle(c, fmt.Sprintf("战术标记（%d 处）", len(anns)))
			theme.MutedText(c, "标记与 demo 绑定，持久化在本地数据库；2D 回放的雷达图会以圆圈+地名显示。")
			ui.Spacer(c)
			for _, a := range anns {
				id := a.ID
				label := a.CalloutID
				note := a.Note
				color := a.Color
				theme.RowCenter(c).FillWidth().Gap(10).PaddingY(2).Children(func() {
					ui.Text(c, "●").FontSize(12).TextColor(annotationColor(color))
					ui.Text(c, label).Width(200).FontSize(12).TextColor(theme.TextPrimary)
					if note != "" {
						ui.Text(c, note).FontSize(12).TextColor(theme.TextMuted)
					}
					ui.Button(c, "移除").FontSize(11).OnClick(func() {
						v.removeAnnotation(id)
					})
				})
			}
		})
	}

	theme.Card(c, func() {
		header := "击杀区位"
		if len(deaths) > 0 {
			header = fmt.Sprintf("击杀区位（共 %d 次击杀）", len(deaths))
		}
		theme.SectionTitle(c, header)
		theme.MutedText(c, "按倒地位置归属 callout；HS 表示爆头。用于查看某个区域反复阵亡的选手与进攻路线。")
		ui.Spacer(c)
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, "回合").Width(50).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "时间").Width(60).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "击杀者").Width(150).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Box(c).Width(20)
			ui.Text(c, "阵亡者").Width(150).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "HS").Width(30).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "区域").Width(160).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "距离").Width(60).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "视线").Width(50).FontSize(11).Bold().TextColor(theme.TextMuted)
		})
		if len(deaths) == 0 {
			theme.MutedText(c, "回放缓存中没有击杀记录。")
			return
		}
		limit := len(deaths)
		if limit > 120 {
			limit = 120
		}
		for _, d := range deaths[:limit] {
			callout := d.Callout
			if !d.Found {
				callout = "—"
			}
			hs := ""
			if d.Headshot {
				hs = "✔"
			}
			killer := d.Killer
			if killer == "" {
				killer = "未知"
			}
			victim := d.Victim
			if victim == "" {
				victim = "未知"
			}
			distance, sight, sightColor := "—", "—", theme.TextMuted
			if k, ok := killCtx[fmt.Sprintf("%d:%d", d.Round, d.Tick)]; ok {
				distance = fmt.Sprintf("%.0f", k.Distance)
				if !k.Occluded {
					sight = "未知"
				} else if k.Visible {
					sight, sightColor = "可见", theme.StatusGreen
				} else {
					sight, sightColor = "遮挡", theme.StatusRed
				}
				if k.SmokeBlocked {
					sight, sightColor = sight+"·烟", theme.AccentAmber
				}
			}
			theme.RowCenter(c).FillWidth().Gap(8).PaddingY(2).Children(func() {
				ui.Text(c, fmt.Sprintf("R%d", d.Round)).Width(50).FontSize(12).TextColor(theme.TextMuted)
				ui.Text(c, formatClock(d.TimeSec)).Width(60).FontSize(12).TextColor(theme.TextMuted)
				ui.Text(c, killer).Width(150).FontSize(12).TextColor(theme.TeamCT)
				ui.Text(c, "→").Width(20).FontSize(12).TextColor(theme.TextMuted)
				ui.Text(c, victim).Width(150).FontSize(12).TextColor(theme.TeamT)
				ui.Text(c, hs).Width(30).FontSize(12).TextColor(theme.StatusRed)
				ui.Text(c, callout).Width(160).FontSize(12).TextColor(theme.TextPrimary)
				ui.Text(c, distance).Width(60).FontSize(12).TextColor(theme.TextMuted)
				ui.Text(c, sight).Width(50).FontSize(12).TextColor(sightColor)
			})
		}
		if len(deaths) > limit {
			theme.MutedText(c, fmt.Sprintf("仅显示前 %d 条（完整数据可用 strikeye-cli export 导出）。", limit))
		}
	})
}

func formatClock(sec float64) string {
	if sec <= 0 {
		return "—"
	}
	s := int(sec)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// controlBuckets groups net control by whole in-round second so rounds of
// different length still line up on a match clock.
func controlBuckets(res mapcontrol.Result) (map[int][]float64, []int) {
	buckets := map[int][]float64{}
	for _, f := range res.Frames {
		key := int(f.TimeSec)
		buckets[key] = append(buckets[key], f.Net)
	}
	order := make([]int, 0, len(buckets))
	for key := range buckets {
		order = append(order, key)
	}
	sort.Ints(order)
	return buckets, order
}

var sparkGlyphs = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// sparkline renders net values in [-1, 1] as a single-line bar chart.
func sparkline(vals []float64) string {
	if len(vals) == 0 {
		return ""
	}
	// Keep the sparkline legible in narrow panels.
	const maxWidth = 90
	if len(vals) > maxWidth {
		stride := float64(len(vals)) / maxWidth
		reduced := make([]float64, 0, maxWidth)
		for i := 0; i < maxWidth; i++ {
			lo := int(float64(i) * stride)
			hi := int(float64(i+1) * stride)
			if hi <= lo {
				hi = lo + 1
			}
			sum := 0.0
			for _, v := range vals[lo:hi] {
				sum += v
			}
			reduced = append(reduced, sum/float64(hi-lo))
		}
		vals = reduced
	}
	var b strings.Builder
	for _, v := range vals {
		idx := int(math.Round((v + 1) / 2 * float64(len(sparkGlyphs)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkGlyphs) {
			idx = len(sparkGlyphs) - 1
		}
		b.WriteString(sparkGlyphs[idx])
	}
	return b.String()
}

// controlBar renders a single net value as a signed bar.
func controlBar(net float64) string {
	const width = 14
	filled := int(math.Round(math.Abs(net) * width))
	if filled < 1 && net != 0 {
		filled = 1
	}
	if net < 0 {
		return strings.Repeat("═", width-filled) + strings.Repeat("█", filled)
	}
	return strings.Repeat("█", filled) + strings.Repeat("═", width-filled)
}

func meanOf(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// controlCallouts picks the callouts whose net control is furthest from
// neutral, which are the areas that actually decided the map.
func controlCallouts(res mapcontrol.Result, limit int) []mapcontrol.CalloutControl {
	items := append([]mapcontrol.CalloutControl(nil), res.Callouts...)
	sort.Slice(items, func(i, j int) bool {
		return math.Abs(items[i].AvgNet) > math.Abs(items[j].AvgNet)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

// pinsFromAnnotations resolves annotation callout ids to world centroids so the
// radar canvas can draw them without knowing about storage.
func pinsFromAnnotations(set *callouts.Set, anns []storage.MapAnnotation) []components.Pin {
	if set == nil || len(anns) == 0 {
		return nil
	}
	pins := make([]components.Pin, 0, len(anns))
	for _, a := range anns {
		place, ok := set.Find(a.CalloutID)
		if !ok {
			continue
		}
		c := place.Centroid()
		pins = append(pins, components.Pin{
			X:     float64(c[0]),
			Y:     float64(c[1]),
			Label: place.Label,
			Color: a.Color,
		})
	}
	return pins
}

// applyPins pushes the current annotation pins onto the replay canvas and
// repaints so a toggle is visible immediately.
func (v *View) applyPins() {
	v.mu.RLock()
	pins := append([]components.Pin(nil), v.zonePins...)
	canvas := v.radarCanvas
	v.mu.RUnlock()
	if canvas == nil {
		return
	}
	canvas.SetPins(pins)
	if len(pins) > 0 {
		v.renderReplayFrameAsync(0, false)
	}
}

// annotationFor returns the annotation pinned to calloutID, if any.
func (v *View) annotationFor(calloutID string) (storage.MapAnnotation, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	for _, a := range v.zoneAnnotations {
		if a.CalloutID == calloutID {
			return a, true
		}
	}
	return storage.MapAnnotation{}, false
}

// toggleAnnotation pins or unpins a callout for this demo.
func (v *View) toggleAnnotation(calloutID string) {
	if calloutID == "" {
		return
	}
	v.mu.RLock()
	db := v.db
	demo := v.demo
	set := v.zoneSet
	v.mu.RUnlock()
	if db == nil || demo == nil {
		return
	}
	demoID := demo.ID
	go func() {
		ctx := context.Background()
		repo := db.MapAnnotations()
		if _, found := repo.Get(ctx, demoID, 0, calloutID); found == nil {
			if _, err := repo.DeleteByKey(ctx, demoID, 0, calloutID); err != nil {
				return
			}
		} else if _, err := repo.Add(ctx, storage.MapAnnotation{DemoID: demoID, CalloutID: calloutID}); err != nil {
			return
		}
		anns, err := repo.ListByDemo(ctx, demoID)
		if err != nil {
			return
		}
		v.mu.Lock()
		v.zoneAnnotations = anns
		v.zonePins = pinsFromAnnotations(set, anns)
		v.mu.Unlock()
		v.applyPins()
		v.invalidate()
	}()
}

// annotationColor maps a stored colour name onto a theme token.
func annotationColor(color string) ui.Color {
	switch color {
	case "red":
		return theme.StatusRed
	case "green":
		return theme.StatusGreen
	case "blue":
		return theme.AccentCyan
	}
	return theme.AccentAmber
}

// removeAnnotation deletes one annotation by id and refreshes the pins.
func (v *View) removeAnnotation(id int64) {
	v.mu.RLock()
	db := v.db
	demo := v.demo
	set := v.zoneSet
	v.mu.RUnlock()
	if db == nil || demo == nil {
		return
	}
	demoID := demo.ID
	go func() {
		ctx := context.Background()
		repo := db.MapAnnotations()
		if _, err := repo.Delete(ctx, id); err != nil {
			return
		}
		anns, err := repo.ListByDemo(ctx, demoID)
		if err != nil {
			return
		}
		v.mu.Lock()
		v.zoneAnnotations = anns
		v.zonePins = pinsFromAnnotations(set, anns)
		v.mu.Unlock()
		v.applyPins()
		v.invalidate()
	}()
}
