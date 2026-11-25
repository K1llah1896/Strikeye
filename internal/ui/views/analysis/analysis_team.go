package analysis

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/callouts"
	"strikeye/internal/h2h"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
	teamreport "strikeye/internal/team"
	"strikeye/internal/tendency"
	"strikeye/internal/ui/theme"
)

// teamTabTopN caps the opening-kill callout list on the team tab.
const teamTabTopN = 8

// teamBundle is everything the team tab shows for one focus team.
type teamBundle struct {
	Team     teamreport.Report
	Tendency tendency.Report
	H2H      h2h.Report
}

// teamCandidates lists the team names the current demo offers, so the tab is
// reachable without typing a name.
func teamCandidates(demo *storage.Demo) []string {
	if demo == nil {
		return nil
	}
	var out []string
	for _, name := range []string{demo.TeamAName, demo.TeamBName} {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		dup := false
		for _, seen := range out {
			if strings.EqualFold(seen, name) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, name)
		}
	}
	return out
}

// loadTeamBundle builds the dossier, scouting profile and head-to-head for one
// team. Replay caches are only needed by the scouting profile, and a missing
// cache degrades that section instead of failing the tab.
func loadTeamBundle(ctx context.Context, db *storage.DB, dataRoot, team, opponent string) (teamBundle, error) {
	var bundle teamBundle
	dossier, err := teamreport.Build(ctx, db, team)
	if err != nil {
		return bundle, fmt.Errorf("team dossier: %w", err)
	}
	bundle.Team = dossier

	replays := map[int64][]parser.ReplayRoundData{}
	demos, err := db.Demos().List(ctx)
	if err == nil {
		for i := range demos {
			demo := &demos[i]
			if demo.ParseStatus != "parsed" {
				continue
			}
			rounds, cacheErr := parser.LoadReplayCache(dataRoot, demo)
			if cacheErr != nil || len(rounds) == 0 {
				continue
			}
			replays[demo.ID] = parser.ReplayRoundList(rounds)
		}
	}

	var table *callouts.Table
	if set, err := callouts.LoadCSV(filepath.Join(dataRoot, "cache", "map_data", "callouts.csv")); err == nil {
		table = set
	}

	if profile, err := tendency.Build(ctx, db, tendency.Input{Team: team, Replays: replays, Callouts: table, TopN: teamTabTopN}); err == nil {
		bundle.Tendency = profile
	}

	if opponent != "" {
		if report, err := h2h.Build(ctx, db, team, opponent); err == nil {
			bundle.H2H = report
		}
	}
	return bundle, nil
}

// ensureTeamAsync loads the team bundle once per (team, opponent) pair.
func (v *View) ensureTeamAsync() {
	v.mu.RLock()
	demo := v.demo
	focus := v.teamFocus
	ready := v.teamReady
	loading := v.teamLoading
	v.mu.RUnlock()
	if demo == nil || ready || loading {
		return
	}
	if focus == "" {
		candidates := teamCandidates(demo)
		if len(candidates) == 0 {
			return
		}
		focus = candidates[0]
		v.mu.Lock()
		v.teamFocus = focus
		v.mu.Unlock()
	}
	opponent := ""
	if strings.EqualFold(focus, demo.TeamAName) {
		opponent = demo.TeamBName
	} else if strings.EqualFold(focus, demo.TeamBName) {
		opponent = demo.TeamAName
	}

	v.mu.Lock()
	v.teamLoading = true
	v.teamSeq++
	seq := v.teamSeq
	db := v.db
	dataRoot := v.dataRoot
	v.mu.Unlock()

	go func() {
		bundle, err := loadTeamBundle(context.Background(), db, dataRoot, focus, opponent)
		v.mu.Lock()
		defer v.mu.Unlock()
		if seq != v.teamSeq {
			return
		}
		v.teamLoading = false
		v.teamReady = true
		v.teamErr = err
		v.teamBundle = bundle
	}()
}

// renderTeam shows the cross-match team view: dossier, scouting profile and the
// head-to-head against the opponent of the open demo.
func (v *View) renderTeam(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	focus := v.teamFocus
	bundle := v.teamBundle
	err := v.teamErr
	loading := v.teamLoading
	v.mu.RUnlock()
	if demo == nil {
		return
	}

	candidates := teamCandidates(demo)
	theme.Card(c, func() {
		theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
			theme.SectionTitle(c, "队伍视角")
			if loading {
				theme.PillBadge(c, "加载中...", theme.AccentCyan)
			}
			ui.Spacer(c)
			for _, name := range candidates {
				selected := strings.EqualFold(name, focus)
				if theme.TabButton(c, name, selected).Clicked() {
					v.mu.Lock()
					v.teamFocus = name
					v.teamReady = false
					v.teamErr = nil
					v.teamSeq++
					v.mu.Unlock()
					v.ensureTeamAsync()
				}
			}
		})
		theme.MutedText(c, "跨场次聚合：分图分侧战绩、经济结构、下包点偏好、开场击杀区位，以及与当前对手的历史交手。")
	})

	if err != nil {
		theme.Card(c, func() {
			theme.MutedText(c, fmt.Sprintf("队伍视图不可用：%v", err))
		})
		return
	}
	if !v.teamReady {
		return
	}

	report := bundle.Team
	if len(report.Maps) == 0 {
		theme.Card(c, func() {
			theme.SectionTitle(c, report.Team)
			theme.MutedText(c, "这个队名在资料库里还没有已解析的比赛。导入并解析该队的 demo 后，这里会显示分图战绩、阵容与对手交手记录。")
		})
		return
	}
	theme.Card(c, func() {
		theme.SectionTitle(c, fmt.Sprintf("%s · %d 张图", report.Team, len(report.Maps)))
		theme.RowCenter(c).FillWidth().Gap(12).Children(func() {
			theme.StatTile(c, "地图胜-负", fmt.Sprintf("%d-%d", report.MapWins, report.MapLosses), "只统计已解析 demo", theme.StatusGreen)
			theme.StatTile(c, "回合胜-负", fmt.Sprintf("%d-%d", report.RoundsWon, report.RoundsLost), "按回合加权", theme.AccentCyan)
			theme.StatTile(c, "阵容人数", fmt.Sprintf("%d", len(report.Roster)), "跨场次去重", theme.AccentAmber)
			theme.StatTile(c, "比赛场次", fmt.Sprintf("%d", len(report.MatchDates)), "含历史日期", theme.TextMuted)
		})
		if len(report.Roster) > 0 {
			theme.MutedText(c, "阵容（按回合数排序）")
			for i, entry := range report.Roster {
				if i >= 5 {
					break
				}
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					ui.Text(c, entry.PlayerName).FontSize(12).TextColor(theme.TextPrimary)
					theme.MutedText(c, fmt.Sprintf("%d 图 · %d 回合 · %d-%d · Rating %.2f", entry.Maps, entry.Rounds, entry.Kills, entry.Deaths, entry.Rating))
				})
			}
		}
		if len(report.Maps) > 0 {
			theme.MutedText(c, "分图战绩")
			for _, m := range report.Maps {
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					ui.Text(c, fmt.Sprintf("%-12s", m.MapName)).Font("Consolas").FontSize(12).TextColor(theme.TextPrimary)
					res := "胜"
					color := theme.StatusGreen
					if !m.Won {
						res = "负"
						color = theme.StatusRed
					}
					ui.Text(c, fmt.Sprintf("%d-%d %s", m.TeamScore, m.OpponentScore, res)).FontSize(12).TextColor(color)
					theme.MutedText(c, fmt.Sprintf("vs %s · %d 回合 · Rating %.2f", m.Opponent, m.RoundsPlayed, m.Rating))
				})
			}
		}
	})

	profile := bundle.Tendency
	if len(profile.Sides) > 0 {
		theme.Card(c, func() {
			theme.SectionTitle(c, fmt.Sprintf("%s · 分图分侧", profile.Team))
			theme.MutedText(c, "地图 / 阵营 / 回合 / 胜率 / 手枪局 / 平均装备 / 每回合道具")
			for _, side := range profile.Sides {
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					ui.Text(c, fmt.Sprintf("%-12s", side.Map)).Font("Consolas").FontSize(12).TextColor(theme.TextPrimary)
					ui.Text(c, fmt.Sprintf("%-2s", side.Side)).Font("Consolas").FontSize(12).TextColor(theme.AccentCyan)
					ui.Text(c, fmt.Sprintf("%3d 回合", side.Rounds)).FontSize(12).TextColor(theme.TextPrimary)
					winColor := theme.StatusGreen
					if side.WinPct < 50 {
						winColor = theme.StatusRed
					}
					ui.Text(c, fmt.Sprintf("%5.1f%%", side.WinPct)).FontSize(12).TextColor(winColor)
					theme.MutedText(c, fmt.Sprintf("手枪 %d/%d · 装备 $%d · 道具 %.1f", side.PistolWins, side.Pistols, side.AvgEquip, side.UtilityPerRound))
				})
			}
			theme.MutedText(c, fmt.Sprintf("共 %d 回合 · 开场击杀 %d 次 · 平均 %.2f 秒", profile.Rounds, profile.OpeningKills, profile.OpeningSecAvg))
		})
	}

	if len(profile.Economy) > 0 || len(profile.Sites) > 0 || len(profile.Opening) > 0 {
		theme.Card(c, func() {
			theme.SectionTitle(c, "结构与偏好")
			if len(profile.Economy) > 0 {
				theme.MutedText(c, "经济类型（阵营 / 类型 / 回合 / 胜率）")
				for _, eco := range profile.Economy {
					theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
						ui.Text(c, fmt.Sprintf("%-2s %-10s", eco.Side, eco.Type)).Font("Consolas").FontSize(12).TextColor(theme.TextPrimary)
						ui.Text(c, fmt.Sprintf("%3d 回合", eco.Rounds)).FontSize(12).TextColor(theme.TextPrimary)
						color := theme.StatusGreen
						if eco.WinPct < 50 {
							color = theme.StatusRed
						}
						ui.Text(c, fmt.Sprintf("%5.1f%%", eco.WinPct)).FontSize(12).TextColor(color)
					})
				}
			}
			if len(profile.Sites) > 0 {
				theme.MutedText(c, "T 侧下包点（地图 / 包点 / 次数 / 转化率）")
				for _, site := range profile.Sites {
					theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
						ui.Text(c, fmt.Sprintf("%-12s %-8s", site.Map, site.Site)).Font("Consolas").FontSize(12).TextColor(theme.TextPrimary)
						ui.Text(c, fmt.Sprintf("%2d 次", site.Plants)).FontSize(12).TextColor(theme.TextPrimary)
						theme.MutedText(c, fmt.Sprintf("转化 %d/%d · %.1f%%", site.Wins, site.Plants, site.WinPct))
					})
				}
			}
			if len(profile.Opening) > 0 {
				theme.MutedText(c, "开场击杀区位（callout / 次数 / 占比）")
				for _, opening := range profile.Opening {
					theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
						ui.Text(c, fmt.Sprintf("%-20s", opening.Callout)).Font("Consolas").FontSize(12).TextColor(theme.TextPrimary)
						ui.Text(c, fmt.Sprintf("%2d 次", opening.Kills)).FontSize(12).TextColor(theme.TextPrimary)
						theme.MutedText(c, fmt.Sprintf("%.1f%%", opening.Share*100))
					})
				}
			} else if profile.CalloutsMissing() {
				theme.MutedText(c, "缺少 callout 数据：运行 `strikeye-cli callouts-fetch` 后开场区位才有名字")
			}
		})
	}

	vs := bundle.H2H
	if vs.TeamA != "" && vs.Matches > 0 {
		theme.Card(c, func() {
			theme.SectionTitle(c, fmt.Sprintf("交手记录 · %s vs %s", vs.TeamA, vs.TeamB))
			theme.RowCenter(c).FillWidth().Gap(12).Children(func() {
				theme.StatTile(c, "大场比分", fmt.Sprintf("%d-%d", vs.WinsA, vs.WinsB), "图数胜负", theme.StatusGreen)
				theme.StatTile(c, "地图回合", fmt.Sprintf("%d-%d", vs.RoundsA, vs.RoundsB), "回合小分", theme.AccentCyan)
				theme.StatTile(c, "交手场次", fmt.Sprintf("%d", vs.Matches), "含两图系列", theme.AccentAmber)
			})
			for _, m := range vs.Maps {
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					ui.Text(c, fmt.Sprintf("%-12s", m.Map)).Font("Consolas").FontSize(12).TextColor(theme.TextPrimary)
					ui.Text(c, fmt.Sprintf("%s %d-%d %s", vs.TeamA, m.WinsA, m.WinsB, vs.TeamB)).FontSize(12).TextColor(theme.AccentCyan)
					theme.MutedText(c, fmt.Sprintf("%d 回合（%d-%d）", m.RoundsA+m.RoundsB, m.RoundsA, m.RoundsB))
				})
			}
			for i, m := range vs.Recent {
				if i >= 5 {
					break
				}
				theme.MutedText(c, fmt.Sprintf("%s · %s · %d-%d · 胜者 %s", m.Date, m.Map, m.ScoreA, m.ScoreB, m.Winner))
			}
		})
	}
}
