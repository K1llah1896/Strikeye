// Package spectator implements View 2: Spectator independent multi-POV video center
// with strict verified-MP4 vs unrendered separation, Event Timeline, and Key HUD.
package spectator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
	"strikeye/internal/ui/components"
	"strikeye/internal/ui/theme"
)

// AgentCaller abstracts JSON-RPC calls to strikeye-agent.
type AgentCaller interface {
	Call(ctx context.Context, method string, params any, result any) error
}

// POVMediaState describes the real media readiness of a player/observer in a round.
type POVMediaState struct {
	Key        string // steamID64 or "observer"
	Label      string
	Team       string
	SpecSlot   int
	State      string // "ready", "queued", "unrendered"
	Video      *storage.MediaVideo
	KillsRound int
	DeathsRnd  int
}

// View manages Spectator state, POV switching, Event Timeline, and Key HUD.
type View struct {
	mu              sync.RWMutex
	db              *storage.DB
	agent           AgentCaller
	ffmpegPath      string
	demos           []storage.Demo
	selectedDemo    *storage.Demo
	players         []storage.MatchPlayer
	rounds          []storage.MatchRound
	events          []storage.MatchEvent
	videos          []storage.MediaVideo
	tasks           []storage.RenderTask
	selectedRound   int
	selectedPOVKey  string
	showKeyHUD      bool
	replayHeader    parser.ReplayHeader
	replayRound     parser.ReplayRoundData
	hasReplayRound  bool
	playerWidget    *components.VideoPlayerWidget
	loadedVideoPath string
	statusMsg       string
	onStartRender   func()
}

// NewView constructs a Spectator view.
func NewView(db *storage.DB, agent AgentCaller, ffmpegPath string, onStartRender func()) *View {
	return &View{
		db:            db,
		agent:         agent,
		ffmpegPath:    ffmpegPath,
		selectedRound: 1,
		showKeyHUD:    true,
		playerWidget:  components.NewVideoPlayerWidget(ffmpegPath),
		onStartRender: onStartRender,
	}
}

// Close releases active video decoder resources.
func (v *View) Close() {
	if v.playerWidget != nil {
		v.playerWidget.Close()
	}
}

// SelectDemo loads all metadata, media, tasks, and replay index for demoID.
func (v *View) SelectDemo(ctx context.Context, demoID int64) error {
	if v.db == nil {
		return nil
	}
	demos, err := v.db.Demos().List(ctx)
	if err != nil {
		return err
	}
	var target *storage.Demo
	for i := range demos {
		if demos[i].ID == demoID {
			cp := demos[i]
			target = &cp
			break
		}
	}
	if target == nil && len(demos) > 0 {
		cp := demos[0]
		target = &cp
	}
	if target == nil {
		v.mu.Lock()
		v.demos = demos
		v.selectedDemo = nil
		v.mu.Unlock()
		return nil
	}
	players, _ := v.db.Analysis().ListMatchPlayers(ctx, target.ID)
	rounds, _ := v.db.Analysis().ListMatchRounds(ctx, target.ID)
	events, _ := v.db.Analysis().ListMatchEvents(ctx, target.ID, nil)
	videos, _ := v.db.Media().ListByDemo(ctx, target.ID)
	queued, _ := v.db.RenderTasks().ListByStatus(ctx, "queued")
	running, _ := v.db.RenderTasks().ListByStatus(ctx, "running")
	tasks := append(queued, running...)

	selectedRound := 1
	if len(rounds) > 0 {
		selectedRound = rounds[0].RoundNumber
	}
	var replayHdr parser.ReplayHeader
	var replayRd parser.ReplayRoundData
	hasReplay := false
	if target.ReplayCachePath != nil && *target.ReplayCachePath != "" {
		if hdr, rd, rerr := parser.ReadReplayRound(*target.ReplayCachePath, uint32(selectedRound)); rerr == nil {
			replayHdr = hdr
			replayRd = rd
			hasReplay = true
		}
	}

	v.mu.Lock()
	v.demos = demos
	v.selectedDemo = target
	v.players = players
	v.rounds = rounds
	v.events = events
	v.videos = videos
	v.tasks = tasks
	v.selectedRound = selectedRound
	if v.selectedPOVKey == "" && len(players) > 0 {
		v.selectedPOVKey = players[0].SteamID64
	}
	v.replayHeader = replayHdr
	v.replayRound = replayRd
	v.hasReplayRound = hasReplay
	v.mu.Unlock()

	v.syncVideoForSelection(0)
	return nil
}

func (v *View) loadReplayRound() {
	v.mu.RLock()
	demo := v.selectedDemo
	roundNum := v.selectedRound
	v.mu.RUnlock()

	if demo == nil || demo.ReplayCachePath == nil || *demo.ReplayCachePath == "" {
		v.mu.Lock()
		v.hasReplayRound = false
		v.mu.Unlock()
		return
	}
	hdr, rd, err := parser.ReadReplayRound(*demo.ReplayCachePath, uint32(roundNum))
	v.mu.Lock()
	if err == nil {
		v.replayHeader = hdr
		v.replayRound = rd
		v.hasReplayRound = true
	} else {
		v.hasReplayRound = false
	}
	v.mu.Unlock()
}

// ResolvePOVStates computes strict real-MP4 vs queued vs unrendered state for all 10 players + Observer.
func ResolvePOVStates(
	players []storage.MatchPlayer,
	videos []storage.MediaVideo,
	tasks []storage.RenderTask,
	events []storage.MatchEvent,
	demoID int64,
	roundNum int,
) []POVMediaState {
	out := make([]POVMediaState, 0, len(players)+1)

	findReadyVideo := func(steamID, persp string) *storage.MediaVideo {
		for i := range videos {
			mv := &videos[i]
			if mv.RoundNumber == nil || *mv.RoundNumber != roundNum {
				continue
			}
			if mv.Perspective != persp {
				continue
			}
			if persp == "pov" && mv.PlayerSteamID64 != nil && *mv.PlayerSteamID64 == steamID {
				if _, err := os.Stat(mv.FilePath); err == nil {
					return mv
				}
			}
			if persp == "observer" {
				if _, err := os.Stat(mv.FilePath); err == nil {
					return mv
				}
			}
		}
		return nil
	}

	isQueued := func(steamID, persp string) bool {
		for _, t := range tasks {
			if t.DemoID != demoID || t.Perspective != persp {
				continue
			}
			if t.RoundNumber != nil && *t.RoundNumber != roundNum {
				continue
			}
			if persp == "observer" || t.TargetSteamID64 == steamID {
				return true
			}
		}
		return false
	}

	for idx, p := range players {
		kills, deaths := 0, 0
		for _, ev := range events {
			if ev.RoundNumber != roundNum || ev.EventType != "kill" {
				continue
			}
			if ev.ActorSteamID64 != nil && *ev.ActorSteamID64 == p.SteamID64 {
				kills++
			}
			if ev.TargetSteamID64 != nil && *ev.TargetSteamID64 == p.SteamID64 {
				deaths++
			}
		}
		st := "unrendered"
		vid := findReadyVideo(p.SteamID64, "pov")
		if vid != nil {
			st = "ready"
		} else if isQueued(p.SteamID64, "pov") {
			st = "queued"
		}
		out = append(out, POVMediaState{
			Key:        p.SteamID64,
			Label:      p.PlayerName,
			Team:       p.TeamKey,
			SpecSlot:   idx + 1,
			State:      st,
			Video:      vid,
			KillsRound: kills,
			DeathsRnd:  deaths,
		})
	}

	obsState := "unrendered"
	obsVid := findReadyVideo("", "observer")
	if obsVid != nil {
		obsState = "ready"
	} else if isQueued("", "observer") {
		obsState = "queued"
	}
	out = append(out, POVMediaState{
		Key:      "observer",
		Label:    "Observer (导播视角)",
		Team:     "OBS",
		SpecSlot: 0,
		State:    obsState,
		Video:    obsVid,
	})
	return out
}

func (v *View) currentRoundBounds() (int64, int64, float64) {
	tickRate := 64.0
	if v.selectedDemo != nil && v.selectedDemo.TickRate > 0 {
		tickRate = v.selectedDemo.TickRate
	}
	for _, r := range v.rounds {
		if r.RoundNumber == v.selectedRound {
			return r.StartTick, r.EndTick, tickRate
		}
	}
	return 0, 6400, tickRate
}

func (v *View) syncVideoForSelection(preserveRelSec float64) {
	v.mu.RLock()
	demo := v.selectedDemo
	players := v.players
	videos := v.videos
	tasks := v.tasks
	events := v.events
	selectedRound := v.selectedRound
	selectedPOVKey := v.selectedPOVKey
	v.mu.RUnlock()

	if demo == nil {
		return
	}
	states := ResolvePOVStates(players, videos, tasks, events, demo.ID, selectedRound)
	for _, st := range states {
		if st.Key != selectedPOVKey {
			continue
		}
		if st.State == "ready" && st.Video != nil {
			startT, endT, tickRate := v.currentRoundBounds()
			if st.Video.StartTick > 0 {
				startT = st.Video.StartTick
			}
			if st.Video.EndTick > 0 {
				endT = st.Video.EndTick
			}
			dur := st.Video.DurationSec
			if dur <= 0 && endT > startT {
				dur = float64(endT-startT) / tickRate
			}
			v.mu.Lock()
			v.loadedVideoPath = st.Video.FilePath
			v.mu.Unlock()
			v.playerWidget.Load(st.Video.FilePath, dur, startT, endT, tickRate, preserveRelSec)
			return
		}
	}
	v.mu.Lock()
	v.loadedVideoPath = ""
	v.mu.Unlock()
	v.playerWidget.Close()
}

func (v *View) roundEventAnchors() []components.TimelineAnchor {
	startT, _, tickRate := v.currentRoundBounds()
	var anchors []components.TimelineAnchor
	for _, ev := range v.events {
		if ev.RoundNumber != v.selectedRound {
			continue
		}
		relSec := float64(ev.Tick-startT) / tickRate
		if relSec < 0 {
			relSec = 0
		}
		kind := ev.EventType
		if ev.EventType == "kill" {
			if ev.ActorSteamID64 != nil && *ev.ActorSteamID64 == v.selectedPOVKey {
				kind = "kill"
			} else if ev.TargetSteamID64 != nil && *ev.TargetSteamID64 == v.selectedPOVKey {
				kind = "death"
			} else {
				kind = "teammate_kill"
			}
		}
		label := ev.EventType
		if ev.Weapon != nil && *ev.Weapon != "" {
			label = *ev.Weapon
		}
		anchors = append(anchors, components.TimelineAnchor{
			EventID:  ev.ID,
			Tick:     ev.Tick,
			TimeSec:  relSec,
			Kind:     kind,
			Label:    label,
			RoundNum: ev.RoundNumber,
		})
	}
	return anchors
}

// Render draws the Spectator round selector, video/empty-state viewport, Event Timeline, Key HUD, and 10-player POV list.
func (v *View) Render(c *ui.Context) {
	v.mu.RLock()
	demo := v.selectedDemo
	players := v.players
	rounds := v.rounds
	events := v.events
	videos := v.videos
	tasks := v.tasks
	selectedRound := v.selectedRound
	selectedPOVKey := v.selectedPOVKey
	hasReplayRound := v.hasReplayRound
	replayHeader := v.replayHeader
	replayRound := v.replayRound
	v.mu.RUnlock()

	if demo == nil {
		ui.Column(c).Fill().Center().Padding(32).Children(func() {
			ui.Box(c).Width(460).Children(func() {
				theme.EmptyStateCard(c, theme.IconSpectator, "尚未选择比赛", "请先在比赛资料库选择一场比赛进入独立观战。")
			})
		})
		return
	}

	povStates := ResolvePOVStates(players, videos, tasks, events, demo.ID, selectedRound)
	var activePOV *POVMediaState
	for i := range povStates {
		if povStates[i].Key == selectedPOVKey {
			activePOV = &povStates[i]
			break
		}
	}

	ui.Column(c).Fill().Padding(14).Gap(14).Children(func() {
		// Match Hero summary banner (same component style as Match Library & Match Analysis)
		mapLabel := "未知地图"
		if demo.MapName != nil {
			mapLabel = *demo.MapName
		}
		theme.RenderMapScoreBanner(c, mapLabel, demo.TeamAName, demo.TeamBName, demo.TeamAScore, demo.TeamBScore, func() {
			theme.PillBadge(c, fmt.Sprintf("R%d  •  共 %d 回合", selectedRound, len(rounds)), theme.AccentAmber)
		})

		// Round selector pill track + Key HUD toggle
		theme.GlassCard(c, func() {
			theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
				ui.ScrollHorizontal(c).FillWidth().Grow(1).MinWidth(0).Children(func() {
					theme.RowCenter(c).Gap(6).PaddingY(2).Children(func() {
						for _, r := range rounds {
							active := r.RoundNumber == selectedRound
							bg := theme.BgInput
							fg := theme.TextMuted
							borderCol := ui.Transparent
							if active {
								bg = ui.RGBA(56, 189, 248, 0.18)
								fg = theme.TextPrimary
								borderCol = ui.RGBA(56, 189, 248, 0.42)
							}
							pill := ui.ButtonBase(c).
								Background(bg).
								Border(1, borderCol).
								Radius(999).
								PaddingX(10).
								PaddingY(4).
								Children(func() {
									el := theme.StatText(c, fmt.Sprintf("R%d", r.RoundNumber)).FontSize(11).TextColor(fg)
									if active {
										el.Bold()
									}
								}).
								Transition(ui.ElementTransition{Colors: true, Duration: 150 * time.Millisecond, Ease: ui.EaseOut})
							if pill.Hovered() {
								pill = pill.AnimatePose(ui.Pose{Y: -1}, ui.Snappy)
							} else {
								pill = pill.AnimatePose(ui.Pose{}, ui.Snappy)
							}
							if pill.Clicked() {
								v.mu.Lock()
								v.selectedRound = r.RoundNumber
								v.mu.Unlock()
								v.loadReplayRound()
								v.syncVideoForSelection(0)
							}
						}
					})
				})
				if len(rounds) > 16 {
					theme.StatText(c, fmt.Sprintf("共 %d 回合 ›", len(rounds))).Shrink(0).FontSize(11).TextColor(theme.TextMuted)
				}
				ui.Checkbox(c, &v.showKeyHUD, "显示实时按键 HUD")
			})
		})

		// Main split: Left Video/Empty Card + Key HUD | Right 10-Player POV Switcher
		ui.Row(c).FillWidth().Grow(1).MinHeight(0).Gap(12).Children(func() {
			ui.Column(c).FillHeight().Grow(1).MinWidth(0).MinHeight(0).Gap(10).Children(func() {
				if activePOV != nil && activePOV.State == "ready" && activePOV.Video != nil {
					anchors := v.roundEventAnchors()
					v.playerWidget.Render(c, anchors, false, nil)
					if v.showKeyHUD {
						tick := v.playerWidget.CurrentTick()
						var buttons uint16
						hasData := false
						if hasReplayRound {
							buttons, hasData = components.ResolveButtonsAtTick(replayRound.InputEdges, uint8(activePOV.SpecSlot), tick, replayHeader.InputCap)
						}
						components.RenderKeyHUD(c, components.KeyHUDState{
							Visible:    true,
							HasData:    hasData,
							Buttons:    buttons,
							PlayerName: activePOV.Label,
						})
					}
				} else if activePOV != nil {
					v.renderUnrenderedCard(c, *activePOV)
				}
			})

			// Right 10-player + Observer POV switcher
			ui.Column(c).Width(296).FillHeight().MinHeight(0).Children(func() {
				theme.Card(c, func() {
					theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
						theme.SectionTitle(c, "视角切换")
						ui.Spacer(c)
						theme.MutedText(c, "真实 MP4 绑定")
					})
					ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
						ui.Column(c).FillWidth().Gap(8).Children(func() {
							var ctList, tList, otherList []POVMediaState
							var obs *POVMediaState
							for i := range povStates {
								st := povStates[i]
								if st.Key == "observer" {
									obs = &st
									continue
								}
								tk := strings.ToUpper(st.Team)
								if tk == "CT" || tk == "A" {
									ctList = append(ctList, st)
								} else if tk == "T" || tk == "B" {
									tList = append(tList, st)
								} else {
									otherList = append(otherList, st)
								}
							}
							if len(ctList) > 0 {
								v.renderPOVGroup(c, "CT 阵营", theme.TeamCT, ctList)
							}
							if len(tList) > 0 {
								v.renderPOVGroup(c, "T 阵营", theme.TeamT, tList)
							}
							if len(otherList) > 0 {
								v.renderPOVGroup(c, "选手视角", theme.AccentCyan, otherList)
							}
							if obs != nil {
								v.renderPOVGroup(c, "导播视角", theme.AccentAmber, []POVMediaState{*obs})
							}
						})
					})
				}).FillHeight().MinHeight(0)
			})
		})
	})
}

func (v *View) renderPOVGroup(c *ui.Context, title string, accent ui.Color, items []POVMediaState) {
	ui.Column(c).FillWidth().Gap(5).Children(func() {
		theme.RowCenter(c).Gap(6).Children(func() {
			theme.StatusDot(c, accent, 6)
			ui.Text(c, title).FontSize(11).Bold().TextColor(accent)
		})
		for _, st := range items {
			v.renderPOVItem(c, st)
		}
	})
}

func (v *View) renderPOVItem(c *ui.Context, st POVMediaState) {
	statusCol := theme.MediaStatusColor(st.State)
	stateText := "未渲染"
	switch st.State {
	case "ready":
		stateText = "已就绪"
	case "queued":
		stateText = "排队中"
	}
	bg := theme.BgInput
	border := theme.RimBorder
	if st.Key == v.selectedPOVKey {
		bg = theme.BgElevated
		border = theme.AccentAmber
	}
	btn := ui.ButtonBase(c).
		FillWidth().
		Background(bg).
		Border(1, border).
		Radius(12).
		PaddingX(10).
		PaddingY(8).
		Gap(8).
		AlignItems(ui.Center).
		Transition(ui.ElementTransition{Colors: true, Duration: 150 * time.Millisecond, Ease: ui.EaseOut}).
		Children(func() {
			theme.StatusDot(c, statusCol, 8)
			ui.Text(c, st.Label).FontSize(12).Bold().TextColor(theme.TextPrimary)
			ui.Spacer(c)
			if st.Key != "observer" {
				theme.StatText(c, fmt.Sprintf("%dK/%dD", st.KillsRound, st.DeathsRnd)).FontSize(11).TextColor(theme.TextMuted)
			}
			ui.Text(c, stateText).FontSize(10).Bold().TextColor(statusCol)
		})
	if btn.Hovered() {
		btn = btn.AnimatePose(ui.Pose{X: 3}, ui.Snappy)
	} else {
		btn = btn.AnimatePose(ui.Pose{}, ui.Snappy)
	}
	if btn.Clicked() {
		relSec := 0.0
		if v.loadedVideoPath != "" {
			relSec = v.playerWidget.CurrentTimeSec()
		}
		v.selectedPOVKey = st.Key
		v.syncVideoForSelection(relSec)
	}
}

func (v *View) renderUnrenderedCard(c *ui.Context, st POVMediaState) {
	theme.GlassCard(c, func() {
		ui.Column(c).FillWidth().Height(380).Center().Gap(14).Children(func() {
			ui.Box(c).Width(46).Height(46).Radius(23).
				Background(ui.RGBA(56, 189, 248, 0.12)).
				Border(1, ui.RGBA(56, 189, 248, 0.25)).
				Center().
				Children(func() {
					theme.RenderIcon(c, theme.IconFilm, 22, theme.AccentAmber)
				})
			stateLabel := "未生成独立视角视频"
			if st.State == "queued" {
				stateLabel = "排队渲染中"
			} else if st.State == "running" {
				stateLabel = "正在渲染中"
			}
			theme.PillBadge(c, stateLabel, theme.MediaStatusColor(st.State))
			theme.RowCenter(c).Gap(10).Children(func() {
				theme.ScoreText(c, fmt.Sprintf("R%d", v.selectedRound)).FontSize(24).TextColor(theme.AccentAmber)
				ui.Text(c, st.Label).FontSize(18).Bold().TextColor(theme.TextPrimary)
			})
			theme.StatText(c, fmt.Sprintf("本回合战绩: %d 击杀  /  %d 阵亡", st.KillsRound, st.DeathsRnd)).
				FontSize(13).TextColor(theme.TextMuted)
			theme.MutedText(c, "Strikeye 严格区分真实 MP4 与未生成视角，不会拿其他视角视频冒充。")
			theme.RowCenter(c).Gap(10).Children(func() {
				if theme.IconActionButton(c, theme.IconPlus, "加入后台渲染队列").Clicked() {
					v.enqueuePOV(st)
				}
				if theme.IconAccentButton(c, theme.IconBolt, "立即启动 CS2 渲染此视角").Clicked() {
					v.enqueuePOV(st)
					if v.onStartRender != nil {
						v.onStartRender()
					}
				}
			})
			if v.statusMsg != "" {
				theme.MutedText(c, v.statusMsg)
			}
		})
	})
}

func (v *View) enqueuePOV(st POVMediaState) {
	if v.agent == nil || v.selectedDemo == nil {
		return
	}
	startT, endT, _ := v.currentRoundBounds()
	persp := "pov"
	if st.Key == "observer" {
		persp = "observer"
	}
	rnd := v.selectedRound
	var res map[string]any
	err := v.agent.Call(context.Background(), "render.enqueue", map[string]any{
		"demo_id":            v.selectedDemo.ID,
		"round_number":       rnd,
		"target_steam_id64":  st.Key,
		"target_player_name": st.Label,
		"target_spec_slot":   st.SpecSlot,
		"perspective":        persp,
		"task_scope":         "round_pov",
		"start_tick":         startT,
		"end_tick":           endT,
		"priority":           100,
	}, &res)
	if err != nil {
		v.statusMsg = "入队失败: " + err.Error()
		return
	}
	v.statusMsg = fmt.Sprintf("已以最高优先级(100)加入渲染队列: R%d %s", rnd, st.Label)
	_ = v.SelectDemo(context.Background(), v.selectedDemo.ID)
}
