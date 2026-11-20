// Package analysis implements View 3: Match Analysis with 5 tabs:
// Overview & Scoreboard, Rounds & Economy, Players & CS Data Radar, 2D Replay, and Heatmaps.
package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"strikeye/assets/maps"
	"strikeye/internal/config"
	"strikeye/internal/parser"
	"strikeye/internal/radar"
	"strikeye/internal/storage"
	"strikeye/internal/ui/components"
	"strikeye/internal/ui/theme"
)

// Tab identifies one of the 5 Match Analysis sub-views.
type Tab string

const (
	TabOverview    Tab = "overview"
	TabEconomy     Tab = "economy"
	TabPerformance Tab = "performance"
	TabRadar       Tab = "radar"
	TabReplay2D    Tab = "replay2d"
	TabHeatmaps    Tab = "heatmaps"
)

// View manages Match Analysis state across all 6 tabs.
type View struct {
	mu                sync.RWMutex
	db                *storage.DB
	dataRoot          string
	ffmpegPath        string
	demo              *storage.Demo
	players           []storage.MatchPlayer
	playerSides       []storage.MatchPlayerSide
	rounds            []storage.MatchRound
	events            []storage.MatchEvent
	cards             []storage.RadarCard
	activeTab         Tab
	sortKey           string
	sortDesc          bool
	sideFilter        string // "ALL", "CT", "T"
	ecoAdjusted       bool
	expandedPlayerSID string
	selectedRoundNum  int
	compareA          string
	compareB          string
	selectedCardID    string
	portraitInput     string
	logoInput         string
	radarPreview      *components.RadarHexPreview
	radarCanvas       *components.Radar2DCanvas
	heatmapCanvas     *components.Radar2DCanvas
	replayIndex       []parser.RoundIndexEntry
	replayRound       parser.ReplayRoundData
	replayRoundNum    int
	replayFrameIdx    int
	replayFramePos    float64
	replaySpeed       float64
	replayPlaying     bool
	playStop          chan struct{}
	lastStepAt        time.Time
	lowerLayer        bool
	hmPlayer           string
	hmTeam             string
	hmMode             string
	hmRoundType        string
	heatmapRounds      map[int]parser.ReplayRoundData
	heatmapSampleCount int
	replayMapReady     bool
	replayLoading      bool
	heatmapReady       bool
	heatmapLoading     bool
	replaySeq         uint64
	frameRenderSeq    uint64
	heatmapSeq        uint64
	statusMsg         string
	opMu              sync.RWMutex
	opLoading         bool
	opProgress        float64
	opStage           string
	onInvalidate      func()
}

// SetInvalidate registers a UI window invalidation callback for background updates.
func (v *View) SetInvalidate(fn func()) {
	v.opMu.Lock()
	v.onInvalidate = fn
	v.opMu.Unlock()
}

func (v *View) invalidate() {
	v.opMu.RLock()
	inv := v.onInvalidate
	v.opMu.RUnlock()
	if inv != nil {
		inv()
	}
}

// Close stops any active 2D replay playback goroutine.
func (v *View) Close() {
	v.mu.Lock()
	v.stopPlaybackLocked()
	v.mu.Unlock()
}

func (v *View) stopPlaybackLocked() {
	v.replayPlaying = false
	if v.playStop != nil {
		close(v.playStop)
		v.playStop = nil
	}
}

func (v *View) startPlaybackLocked() {
	if v.playStop != nil {
		close(v.playStop)
		v.playStop = nil
	}
	if len(v.replayRound.Frames) == 0 {
		v.replayPlaying = false
		return
	}
	v.replayPlaying = true
	v.replayFramePos = float64(v.replayFrameIdx)
	v.lastStepAt = time.Now()
	stopCh := make(chan struct{})
	v.playStop = stopCh

	go func(stop <-chan struct{}) {
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				v.mu.Lock()
				if !v.replayPlaying || len(v.replayRound.Frames) == 0 {
					v.mu.Unlock()
					return
				}
				elapsed := now.Sub(v.lastStepAt)
				v.lastStepAt = now
				changed := v.advanceReplayLocked(elapsed)
				round := v.replayRound
				frameIdx := v.replayFrameIdx
				stillPlaying := v.replayPlaying
				v.mu.Unlock()

				if changed {
					v.radarCanvas.RenderReplayFrame(round, frameIdx)
					v.invalidate()
				}
				if !stillPlaying {
					v.invalidate()
					return
				}
			}
		}
	}(stopCh)
}

func (v *View) setOpProgress(loading bool, progress float64, stage string) {
	v.opMu.Lock()
	v.opLoading = loading
	v.opProgress = progress
	v.opStage = stage
	inv := v.onInvalidate
	v.opMu.Unlock()
	if inv != nil {
		inv()
	}
}

// NewView constructs a Match Analysis view.
func NewView(db *storage.DB, dataRoot, ffmpegPath string) *View {
	return &View{
		db:            db,
		dataRoot:      dataRoot,
		ffmpegPath:    ffmpegPath,
		activeTab:     TabOverview,
		sortKey:       "rating",
		sortDesc:      true,
		sideFilter:    "ALL",
		radarPreview:  components.NewRadarHexPreview(),
		radarCanvas:   components.NewRadar2DCanvas(560),
		heatmapCanvas: components.NewRadar2DCanvas(560),
		replaySpeed:   1.0,
		hmPlayer:      "全部选手",
		hmTeam:        "全部阵营",
		hmMode:        "position",
		hmRoundType:   "全部回合",
	}
}

// SetActiveTab switches the active Analysis sub-view tab and asynchronously prepares heavy tab assets.
func (v *View) SetActiveTab(tab Tab) {
	v.mu.Lock()
	v.activeTab = tab
	if tab != TabReplay2D {
		v.stopPlaybackLocked()
	}
	v.mu.Unlock()
	if tab == TabReplay2D {
		v.ensureReplayReadyAsync()
	} else if tab == TabHeatmaps {
		v.ensureHeatmapReadyAsync()
	}
}

func (v *View) ensureReplayReady() {
	v.mu.RLock()
	if v.replayMapReady || v.demo == nil {
		v.mu.RUnlock()
		return
	}
	mapName := ""
	if v.demo.MapName != nil {
		mapName = *v.demo.MapName
	}
	lower := v.lowerLayer
	firstRound := 0
	if len(v.replayIndex) > 0 {
		firstRound = int(v.replayIndex[0].RoundNumber)
	}
	v.mu.RUnlock()

	v.radarCanvas.ConfigureMap(mapName, lower, [4]float64{-2500, 2500, -2500, 2500})
	if firstRound > 0 {
		v.loadReplayRound(firstRound)
	}
	v.mu.Lock()
	v.replayMapReady = true
	v.mu.Unlock()
}

// TrimLeadingStationaryFrames drops leading frames at round start where no alive player has moved yet
// (e.g. freeze-time spawn idle frames), keeping one frame right before movement begins.
func TrimLeadingStationaryFrames(rd parser.ReplayRoundData) parser.ReplayRoundData {
	if len(rd.Frames) <= 1 {
		return rd
	}
	type pos2D struct{ x, y float32 }
	origin := make(map[uint8]pos2D, len(rd.Frames[0].Players))
	for _, p := range rd.Frames[0].Players {
		if p.Alive {
			origin[p.Slot] = pos2D{x: p.X, y: p.Y}
		}
	}
	if len(origin) == 0 {
		return rd
	}
	const moveThreshSq = 4.0 // > 2.0 CS2 units in XY plane
	for i := 1; i < len(rd.Frames); i++ {
		for _, p := range rd.Frames[i].Players {
			if !p.Alive {
				continue
			}
			o, ok := origin[p.Slot]
			if !ok {
				continue
			}
			dx := float64(p.X - o.x)
			dy := float64(p.Y - o.y)
			if dx*dx+dy*dy > moveThreshSq {
				startIdx := i - 1
				if startIdx > 0 {
					rd.Frames = rd.Frames[startIdx:]
				}
				return rd
			}
		}
	}
	return rd
}

func (v *View) ensureReplayReadyAsync() {
	v.mu.Lock()
	if v.replayMapReady || v.replayLoading || v.demo == nil {
		v.mu.Unlock()
		return
	}
	v.replayLoading = true
	v.replaySeq++
	seq := v.replaySeq
	mapName := ""
	if v.demo.MapName != nil {
		mapName = *v.demo.MapName
	}
	cachePath := ""
	if v.demo.ReplayCachePath != nil {
		cachePath = *v.demo.ReplayCachePath
	}
	lower := v.lowerLayer
	firstRound := 0
	if len(v.replayIndex) > 0 {
		firstRound = int(v.replayIndex[0].RoundNumber)
	}
	v.mu.Unlock()
	v.invalidate()

	go func() {
		v.radarCanvas.ConfigureMap(mapName, lower, [4]float64{-2500, 2500, -2500, 2500})
		var rd parser.ReplayRoundData
		loaded := false
		if firstRound > 0 && cachePath != "" {
			if _, decoded, err := parser.ReadReplayRound(cachePath, uint32(firstRound)); err == nil {
				rd = TrimLeadingStationaryFrames(decoded)
				loaded = true
				if len(rd.Frames) > 0 {
					v.radarCanvas.RenderReplayFrame(rd, 0)
				}
			}
		}
		v.mu.Lock()
		if v.replaySeq == seq {
			if loaded {
				v.replayRound = rd
				v.replayRoundNum = firstRound
				v.replayFrameIdx = 0
				v.replayFramePos = 0
			}
			v.replayMapReady = true
			v.replayLoading = false
		}
		v.mu.Unlock()
		v.invalidate()
	}()
}

func (v *View) ensureHeatmapReady() {
	v.mu.RLock()
	if v.heatmapReady || v.demo == nil {
		v.mu.RUnlock()
		return
	}
	mapName := ""
	if v.demo.MapName != nil {
		mapName = *v.demo.MapName
	}
	lower := v.lowerLayer
	needRound := len(v.replayRound.Frames) == 0 && len(v.replayIndex) > 0 && v.demo.ReplayCachePath != nil && *v.demo.ReplayCachePath != ""
	cachePath := ""
	firstRound := uint32(0)
	if needRound {
		cachePath = *v.demo.ReplayCachePath
		firstRound = v.replayIndex[0].RoundNumber
	}
	v.mu.RUnlock()

	v.heatmapCanvas.ConfigureMap(mapName, lower, [4]float64{-2500, 2500, -2500, 2500})
	if needRound {
		if _, rd, err := parser.ReadReplayRound(cachePath, firstRound); err == nil {
			v.mu.Lock()
			if len(v.replayRound.Frames) == 0 {
				v.replayRound = rd
			}
			v.mu.Unlock()
		}
	}
	v.refreshHeatmap()
	v.mu.Lock()
	v.heatmapReady = true
	v.mu.Unlock()
}

func (v *View) ensureHeatmapReadyAsync() {
	v.mu.RLock()
	if v.heatmapReady || v.heatmapLoading || v.demo == nil {
		v.mu.RUnlock()
		return
	}
	v.mu.RUnlock()
	v.refreshHeatmapAsync(true)
}

// SelectDemo loads analysis data, radar cards, and CS2RPL01 replay index for demoID.
// Heavy 2D replay frames and KDE heatmaps are lazily initialized on tab switch so
// opening Match Analysis from Match Library remains instantaneous.
func (v *View) SelectDemo(ctx context.Context, demoID int64) error {
	if v.db == nil {
		return nil
	}
	d, err := v.db.Demos().GetByID(ctx, demoID)
	if err != nil {
		return err
	}
	players, _ := v.db.Analysis().ListMatchPlayers(ctx, demoID)
	playerSides, _ := v.db.Analysis().ListMatchPlayerSides(ctx, demoID)
	rounds, _ := v.db.Analysis().ListMatchRounds(ctx, demoID)
	events, _ := v.db.Analysis().ListMatchEvents(ctx, demoID, nil)
	cards, _ := v.db.Analysis().ListRadarCards(ctx, demoID)

	var replayIndex []parser.RoundIndexEntry
	if d.ReplayCachePath != nil && *d.ReplayCachePath != "" {
		if _, entries, err := parser.ReadReplayIndex(*d.ReplayCachePath); err == nil && len(entries) > 0 {
			replayIndex = entries
		}
	}

	var firstCardPath string
	var firstCardID string
	if len(cards) > 0 {
		firstCardID = cards[0].ID
		firstCardPath = cards[0].StaticImagePath
	}
	if firstCardPath != "" {
		v.radarPreview.SetCard(firstCardPath, false)
	}

	v.mu.Lock()
	v.stopPlaybackLocked()
	v.replaySeq++
	v.frameRenderSeq++
	v.heatmapSeq++
	v.demo = &d
	v.players = players
	v.playerSides = playerSides
	v.rounds = rounds
	v.events = events
	v.cards = cards
	v.expandedPlayerSID = ""
	v.selectedRoundNum = 0
	v.replayIndex = replayIndex
	v.replayRound = parser.ReplayRoundData{}
	v.replayRoundNum = 0
	v.replayFrameIdx = 0
	v.replayFramePos = 0
	v.replayMapReady = false
	v.replayLoading = false
	v.heatmapReady = false
	v.heatmapLoading = false
	v.heatmapRounds = nil
	v.heatmapSampleCount = 0
	v.selectedCardID = firstCardID
	if len(players) >= 2 {
		v.compareA = players[0].PlayerName
		v.compareB = players[1].PlayerName
	}
	activeTab := v.activeTab
	v.mu.Unlock()

	if activeTab == TabReplay2D {
		v.ensureReplayReadyAsync()
	} else if activeTab == TabHeatmaps {
		v.ensureHeatmapReadyAsync()
	}
	return nil
}

func (v *View) loadReplayRound(roundNum int) {
	v.mu.RLock()
	if v.demo == nil || v.demo.ReplayCachePath == nil || *v.demo.ReplayCachePath == "" {
		v.mu.RUnlock()
		return
	}
	cachePath := *v.demo.ReplayCachePath
	v.mu.RUnlock()

	if _, rd, err := parser.ReadReplayRound(cachePath, uint32(roundNum)); err == nil {
		rd = TrimLeadingStationaryFrames(rd)
		if len(rd.Frames) > 0 {
			v.radarCanvas.RenderReplayFrame(rd, 0)
		}
		v.mu.Lock()
		v.stopPlaybackLocked()
		v.replayRound = rd
		v.replayRoundNum = roundNum
		v.replayFrameIdx = 0
		v.replayFramePos = 0
		v.mu.Unlock()
	}
}

func (v *View) loadReplayRoundAsync(roundNum int) {
	v.mu.Lock()
	if v.demo == nil || v.demo.ReplayCachePath == nil || *v.demo.ReplayCachePath == "" {
		v.mu.Unlock()
		return
	}
	v.stopPlaybackLocked()
	v.replaySeq++
	seq := v.replaySeq
	v.replayLoading = true
	cachePath := *v.demo.ReplayCachePath
	mapName := ""
	if v.demo.MapName != nil {
		mapName = *v.demo.MapName
	}
	lower := v.lowerLayer
	v.mu.Unlock()
	v.invalidate()

	go func() {
		v.radarCanvas.ConfigureMap(mapName, lower, [4]float64{-2500, 2500, -2500, 2500})
		_, rd, err := parser.ReadReplayRound(cachePath, uint32(roundNum))
		if err == nil {
			rd = TrimLeadingStationaryFrames(rd)
			if len(rd.Frames) > 0 {
				v.radarCanvas.RenderReplayFrame(rd, 0)
			}
		}
		v.mu.Lock()
		if v.replaySeq == seq {
			if err == nil {
				v.replayRound = rd
				v.replayRoundNum = roundNum
				v.replayFrameIdx = 0
				v.replayFramePos = 0
				v.replayMapReady = true
			}
			v.replayLoading = false
		}
		v.mu.Unlock()
		v.invalidate()
	}()
}

func (v *View) renderReplayFrameAsync(frameIdx int, reconfigureMap bool) {
	v.mu.Lock()
	v.frameRenderSeq++
	seq := v.frameRenderSeq
	round := v.replayRound
	mapName := ""
	if v.demo != nil && v.demo.MapName != nil {
		mapName = *v.demo.MapName
	}
	lower := v.lowerLayer
	v.mu.Unlock()

	go func() {
		if reconfigureMap {
			v.radarCanvas.ConfigureMap(mapName, lower, [4]float64{-2500, 2500, -2500, 2500})
		}
		v.mu.RLock()
		stale := v.frameRenderSeq != seq
		v.mu.RUnlock()
		if stale {
			return
		}
		v.radarCanvas.RenderReplayFrame(round, frameIdx)
		v.invalidate()
	}()
}

func (v *View) refreshHeatmap() {
	v.mu.RLock()
	demo := v.demo
	players := v.players
	events := v.events
	rounds := v.rounds
	rd := v.replayRound
	hmRounds := v.heatmapRounds
	hmPlayer := v.hmPlayer
	hmTeam := v.hmTeam
	hmMode := v.hmMode
	hmRoundType := v.hmRoundType
	v.mu.RUnlock()

	teamAName := ""
	if demo != nil {
		teamAName = demo.TeamAName
	}
	f := resolveHeatmapFilter(players, hmPlayer, hmTeam, hmMode, hmRoundType, teamAName)
	var pts []components.HeatmapPoint
	if len(hmRounds) > 0 {
		pts = BuildHeatmapPointsMulti(events, rounds, players, hmRounds, f)
	} else {
		pts = BuildHeatmapPoints(events, rounds, rd, f)
	}
	v.heatmapCanvas.RenderHeatmap(pts)
	v.mu.Lock()
	v.heatmapSampleCount = len(pts)
	v.mu.Unlock()
}

func (v *View) refreshHeatmapAsync(reconfigureMap bool) {
	v.mu.Lock()
	if v.demo == nil {
		v.mu.Unlock()
		return
	}
	v.heatmapSeq++
	seq := v.heatmapSeq
	v.heatmapLoading = true
	mapName := ""
	if v.demo.MapName != nil {
		mapName = *v.demo.MapName
	}
	teamAName := v.demo.TeamAName
	cachePath := ""
	if v.demo.ReplayCachePath != nil {
		cachePath = *v.demo.ReplayCachePath
	}
	lower := v.lowerLayer
	players := v.players
	events := v.events
	rounds := v.rounds
	replayIndex := v.replayIndex
	cachedRounds := v.heatmapRounds
	rd := v.replayRound
	hmPlayer := v.hmPlayer
	hmTeam := v.hmTeam
	hmMode := v.hmMode
	hmRoundType := v.hmRoundType
	v.mu.Unlock()
	v.invalidate()

	go func() {
		if reconfigureMap {
			v.heatmapCanvas.ConfigureMap(mapName, lower, [4]float64{-2500, 2500, -2500, 2500})
		}

		// Lazily load all round replay slices once from .cs2rpl so every filter
		// (position across all rounds, player filter, round type, and event coord fallback) works immediately.
		loadedAll := false
		if len(cachedRounds) == 0 && cachePath != "" && len(replayIndex) > 0 {
			loaded := make(map[int]parser.ReplayRoundData, len(replayIndex))
			for _, ent := range replayIndex {
				if _, decoded, err := parser.ReadReplayRound(cachePath, ent.RoundNumber); err == nil {
					loaded[int(ent.RoundNumber)] = decoded
				}
			}
			if len(loaded) > 0 {
				cachedRounds = loaded
				loadedAll = true
			}
		} else if len(cachedRounds) == 0 && len(rd.Frames) > 0 {
			rn := int(rd.RoundNumber)
			if rn <= 0 {
				rn = 1
			}
			cachedRounds = map[int]parser.ReplayRoundData{rn: rd}
		}

		v.mu.RLock()
		stale := v.heatmapSeq != seq
		v.mu.RUnlock()
		if stale {
			return
		}

		f := resolveHeatmapFilter(players, hmPlayer, hmTeam, hmMode, hmRoundType, teamAName)
		pts := BuildHeatmapPointsMulti(events, rounds, players, cachedRounds, f)
		v.heatmapCanvas.RenderHeatmap(pts)

		v.mu.Lock()
		if v.heatmapSeq == seq {
			if loadedAll {
				v.heatmapRounds = cachedRounds
			}
			v.heatmapSampleCount = len(pts)
			v.heatmapReady = true
			v.heatmapLoading = false
		}
		v.mu.Unlock()
		v.invalidate()
	}()
}

func resolveHeatmapFilter(players []storage.MatchPlayer, hmPlayer, hmTeam, hmMode, hmRoundType, teamAName string) HeatmapFilter {
	var steamID string
	var specSlot int
	if hmPlayer != "" && hmPlayer != "全部选手" {
		for idx, p := range players {
			if p.PlayerName == hmPlayer {
				steamID = p.SteamID64
				if p.SpecSlot != nil && *p.SpecSlot > 0 {
					specSlot = *p.SpecSlot
				} else {
					specSlot = idx + 1
				}
				break
			}
		}
	}
	return HeatmapFilter{
		PlayerName:     hmPlayer,
		PlayerSteamID:  steamID,
		PlayerSpecSlot: specSlot,
		Team:           hmTeam,
		TeamAName:      teamAName,
		Mode:           hmMode,
		RoundType:      hmRoundType,
	}
}

// HeatmapFilter controls KDE sample extraction.
type HeatmapFilter struct {
	PlayerName     string
	PlayerSteamID  string
	PlayerSpecSlot int
	Team           string
	TeamAName      string
	Mode           string // "position", "firefight", "kill", "death"
	RoundType      string // "全部回合", "手枪局", "长枪局", "Eco"
}

// BuildHeatmapPoints extracts world-space HeatmapPoints from events and a single replay round.
func BuildHeatmapPoints(events []storage.MatchEvent, rounds []storage.MatchRound, rd parser.ReplayRoundData, f HeatmapFilter) []components.HeatmapPoint {
	var roundMap map[int]parser.ReplayRoundData
	if len(rd.Frames) > 0 {
		rn := int(rd.RoundNumber)
		if rn <= 0 {
			rn = 1
		}
		roundMap = map[int]parser.ReplayRoundData{rn: rd}
	}
	return BuildHeatmapPointsMulti(events, rounds, nil, roundMap, f)
}

// BuildHeatmapPointsMulti extracts world-space HeatmapPoints across all matching rounds,
// supporting position, firefight, kill, and death modes with player, side, and round-type filters.
func BuildHeatmapPointsMulti(
	events []storage.MatchEvent,
	rounds []storage.MatchRound,
	players []storage.MatchPlayer,
	roundReplays map[int]parser.ReplayRoundData,
	f HeatmapFilter,
) []components.HeatmapPoint {
	roundAllowed := make(map[int]bool, len(rounds))
	roundSides := make(map[int][2]string, len(rounds))
	for _, r := range rounds {
		sideA := strings.ToUpper(strings.TrimSpace(r.TeamASide))
		sideB := strings.ToUpper(strings.TrimSpace(r.TeamBSide))
		roundSides[r.RoundNumber] = [2]string{sideA, sideB}

		if f.RoundType == "" || f.RoundType == "全部回合" {
			roundAllowed[r.RoundNumber] = true
			continue
		}
		eco := strings.ToLower(r.TeamAEconomyType + " " + r.TeamBEconomyType)
		switch f.RoundType {
		case "手枪局":
			if r.RoundNumber == 1 || r.RoundNumber == 13 || strings.Contains(eco, "pistol") {
				roundAllowed[r.RoundNumber] = true
			}
		case "Eco":
			if strings.Contains(eco, "eco") {
				roundAllowed[r.RoundNumber] = true
			}
		case "长枪局":
			if strings.Contains(eco, "full") {
				roundAllowed[r.RoundNumber] = true
			}
		default:
			roundAllowed[r.RoundNumber] = true
		}
	}

	if len(rounds) > 0 && len(roundAllowed) == 0 {
		return nil
	}

	slotBySteam := make(map[string]int, len(players))
	for idx, p := range players {
		if p.SteamID64 != "" {
			if p.SpecSlot != nil && *p.SpecSlot > 0 {
				slotBySteam[p.SteamID64] = *p.SpecSlot
			} else {
				slotBySteam[p.SteamID64] = idx + 1
			}
		}
	}

	var pts []components.HeatmapPoint
	if f.Mode == "position" {
		if len(roundReplays) == 0 {
			return nil
		}
		totalFrames := 0
		for rn, rd := range roundReplays {
			if len(rounds) > 0 && !roundAllowed[rn] {
				continue
			}
			totalFrames += len(rd.Frames)
		}
		stride := 1
		if totalFrames > 4000 {
			stride = 12
		} else if totalFrames > 1500 {
			stride = 6
		} else if totalFrames > 400 {
			stride = 4
		}

		for rn, rd := range roundReplays {
			if len(rounds) > 0 && !roundAllowed[rn] {
				continue
			}
			for i := 0; i < len(rd.Frames); i += stride {
				fr := rd.Frames[i]
				for _, p := range fr.Players {
					if !p.Alive {
						continue
					}
					if f.Team == "T" && p.Team != 1 {
						continue
					}
					if f.Team == "CT" && p.Team != 2 {
						continue
					}
					if f.PlayerSpecSlot > 0 && int(p.Slot) != f.PlayerSpecSlot {
						continue
					}
					pts = append(pts, components.HeatmapPoint{X: float64(p.X), Y: float64(p.Y), Z: float64(p.Z), Weight: 1})
				}
			}
		}
		return pts
	}

	eventSide := func(roundNum int, teamKey *string) string {
		if teamKey == nil || *teamKey == "" {
			return ""
		}
		sides, ok := roundSides[roundNum]
		if !ok {
			if strings.EqualFold(*teamKey, "CT") {
				return "CT"
			}
			if strings.EqualFold(*teamKey, "T") {
				return "T"
			}
			return ""
		}
		if isTeamAKey(*teamKey, f.TeamAName) {
			return sides[0]
		}
		return sides[1]
	}

	lookupReplayPos := func(roundNum int, tick int64, steamID *string) (float64, float64, bool) {
		if steamID == nil || *steamID == "" {
			return 0, 0, false
		}
		slot := slotBySteam[*steamID]
		if slot <= 0 {
			return 0, 0, false
		}
		rd, ok := roundReplays[roundNum]
		if !ok || len(rd.Frames) == 0 {
			return 0, 0, false
		}
		idx := sort.Search(len(rd.Frames), func(i int) bool {
			return rd.Frames[i].Tick >= tick
		})
		if idx >= len(rd.Frames) {
			idx = len(rd.Frames) - 1
		} else if idx > 0 {
			dCurr := rd.Frames[idx].Tick - tick
			dPrev := tick - rd.Frames[idx-1].Tick
			if dPrev < dCurr {
				idx--
			}
		}
		for _, p := range rd.Frames[idx].Players {
			if int(p.Slot) == slot {
				return float64(p.X), float64(p.Y), true
			}
		}
		return 0, 0, false
	}

	for _, ev := range events {
		if len(rounds) > 0 && !roundAllowed[ev.RoundNumber] {
			continue
		}
		if f.PlayerSteamID != "" {
			if f.Mode == "death" {
				if ev.TargetSteamID64 == nil || *ev.TargetSteamID64 != f.PlayerSteamID {
					continue
				}
			} else if ev.ActorSteamID64 == nil || *ev.ActorSteamID64 != f.PlayerSteamID {
				continue
			}
		}
		if f.Team == "T" || f.Team == "CT" {
			var tk *string
			if f.Mode == "death" {
				tk = ev.TargetTeamKey
			} else {
				tk = ev.ActorTeamKey
			}
			if sd := eventSide(ev.RoundNumber, tk); sd != "" && sd != f.Team {
				continue
			}
		}

		var posX, posY, vicX, vicY *float64
		if ev.ExtraJSON != nil && *ev.ExtraJSON != "" {
			var coords struct {
				PosX    *float64 `json:"pos_x"`
				PosY    *float64 `json:"pos_y"`
				VictimX *float64 `json:"victim_x"`
				VictimY *float64 `json:"victim_y"`
			}
			if err := json.Unmarshal([]byte(*ev.ExtraJSON), &coords); err == nil {
				posX, posY, vicX, vicY = coords.PosX, coords.PosY, coords.VictimX, coords.VictimY
			}
		}

		if (posX == nil || posY == nil) && ev.EventType == "kill" {
			if x, y, ok := lookupReplayPos(ev.RoundNumber, ev.Tick, ev.ActorSteamID64); ok {
				posX, posY = &x, &y
			}
		}
		if (vicX == nil || vicY == nil) && ev.EventType == "kill" {
			if x, y, ok := lookupReplayPos(ev.RoundNumber, ev.Tick, ev.TargetSteamID64); ok {
				vicX, vicY = &x, &y
			}
		}

		if f.Mode == "kill" && ev.EventType == "kill" && posX != nil && posY != nil {
			pts = append(pts, components.HeatmapPoint{X: *posX, Y: *posY, Weight: 1})
		} else if f.Mode == "death" && ev.EventType == "kill" && vicX != nil && vicY != nil {
			pts = append(pts, components.HeatmapPoint{X: *vicX, Y: *vicY, Weight: 1})
		} else if f.Mode == "firefight" {
			if posX != nil && posY != nil {
				pts = append(pts, components.HeatmapPoint{X: *posX, Y: *posY, Weight: 1})
			}
			if f.PlayerSteamID == "" && vicX != nil && vicY != nil {
				pts = append(pts, components.HeatmapPoint{X: *vicX, Y: *vicY, Weight: 1})
			}
		}
	}
	return pts
}

// SortPlayers returns a sorted copy of players by key and direction.
func SortPlayers(players []storage.MatchPlayer, key string, desc bool) []storage.MatchPlayer {
	out := make([]storage.MatchPlayer, len(players))
	copy(out, players)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less bool
		switch key {
		case "kills":
			less = a.Kills < b.Kills
		case "deaths":
			less = a.Deaths < b.Deaths
		case "assists":
			less = a.Assists < b.Assists
		case "kd":
			less = a.KDRatio < b.KDRatio
		case "adr":
			less = a.ADR < b.ADR
		case "kast":
			less = a.KASTPercent < b.KASTPercent
		case "hs":
			less = a.HSPercent < b.HSPercent
		case "fk":
			less = a.FirstKills < b.FirstKills
		case "fd":
			less = a.FirstDeaths < b.FirstDeaths
		case "tk":
			less = a.TradeKills < b.TradeKills
		case "multi":
			less = a.MultiKillRounds < b.MultiKillRounds
		case "clutch":
			less = a.ClutchWins < b.ClutchWins
		case "swing":
			less = a.RoundSwing < b.RoundSwing
		case "eco_rating":
			less = a.EcoAdjRating < b.EcoAdjRating
		default:
			less = a.StrikeyeRating < b.StrikeyeRating
		}
		if desc {
			return !less
		}
		return less
	})
	return out
}

// Render draws the 6-tab Match Analysis workspace.
func (v *View) Render(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	activeTab := v.activeTab
	statusMsg := v.statusMsg
	v.mu.RUnlock()

	if demo == nil {
		ui.Column(c).Fill().Center().Padding(32).Children(func() {
			ui.Box(c).Width(460).Children(func() {
				theme.EmptyStateCard(c, theme.IconAnalysis, "尚未选择比赛", "请先在比赛资料库选择一场已解析的 Demo 以查看深度数据与 2D 战术回放。")
			})
		})
		return
	}

	ui.Column(c).Fill().Padding(14).Gap(14).Children(func() {
		// Match Hero summary banner (aligned with Match Library map logo + scene scoreline style)
		mapName := "未知地图"
		if demo.MapName != nil {
			mapName = *demo.MapName
		}
		theme.RenderMapScoreBanner(c, mapName, demo.TeamAName, demo.TeamBName, demo.TeamAScore, demo.TeamBScore, func() {
			theme.RowCenter(c).Gap(8).Children(func() {
				if statusMsg != "" {
					theme.PillBadge(c, statusMsg, theme.AccentCyan)
				}
				theme.PillBadge(c, fmt.Sprintf("总回合 %d  •  %.0fs", demo.TotalRounds, demo.DurationSec), theme.AccentAmber)
			})
		})

		// Segmented pill tab bar
		theme.RowCenter(c).FillWidth().Children(func() {
			theme.SegmentedControl(c, []theme.SegmentItem{
				{Key: string(TabOverview), Label: "1. 概览与记分板", Icon: theme.IconAnalysis},
				{Key: string(TabEconomy), Label: "2. 回合与经济走势", Icon: theme.IconBolt},
				{Key: string(TabPerformance), Label: "3. 对位与武器表现", Icon: theme.IconSliders},
				{Key: string(TabRadar), Label: "4. 选手对比与雷达图", Icon: theme.IconRadar},
				{Key: string(TabReplay2D), Label: "5. 2D 雷达回放", Icon: theme.IconSpectator},
				{Key: string(TabHeatmaps), Label: "6. 战术热力图", Icon: theme.IconHighlights},
			}, string(activeTab), func(k string) {
				v.SetActiveTab(Tab(k))
			})
			ui.Spacer(c)
		})

		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				v.opMu.RLock()
				opLoading := v.opLoading
				opProg := v.opProgress
				opStage := v.opStage
				v.opMu.RUnlock()
				if opLoading {
					theme.Card(c, func() {
						theme.ProgressBar(c, opProg, opStage)
					})
					return
				}
				switch activeTab {
				case TabOverview:
					v.renderOverview(c)
				case TabEconomy:
					v.renderEconomy(c)
				case TabPerformance:
					v.renderPerformance(c)
				case TabRadar:
					v.renderRadarWorkbench(c)
				case TabReplay2D:
					v.renderReplay2D(c)
				case TabHeatmaps:
					v.renderHeatmaps(c)
				}
			})
		})
	})
}

func (v *View) renderOverview(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	players := v.players
	playerSides := v.playerSides
	v.mu.RUnlock()
	if demo == nil {
		return
	}
	activePlayers, hasSideData := MergeSideStats(players, playerSides, v.sideFilter)
	sorted := SortPlayers(activePlayers, v.sortKey, v.sortDesc)
	if len(sorted) > 0 {
		mvp := SortPlayers(activePlayers, "rating", true)[0]
		topADR := SortPlayers(activePlayers, "adr", true)[0]
		topFK := SortPlayers(activePlayers, "fk", true)[0]
		topSwing := SortPlayers(activePlayers, "swing", true)[0]
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			ratingVal := mvp.StrikeyeRating
			ratingLbl := "MVP (STRIKEYE RATING)"
			if v.ecoAdjusted && mvp.EcoAdjRating > 0 {
				ratingVal = mvp.EcoAdjRating
				ratingLbl = "MVP (ECO-ADJUSTED)"
			}
			leaderCard(c, ratingLbl, mvp.PlayerName, fmt.Sprintf("%.2f", ratingVal))
			leaderCard(c, "TOP ADR", topADR.PlayerName, fmt.Sprintf("%.1f", topADR.ADR))
			leaderCard(c, "FIRST KILLS (FK)", topFK.PlayerName, fmt.Sprintf("%d", topFK.FirstKills))
			leaderCard(c, "ROUND SWING", topSwing.PlayerName, fmtSwing(topSwing.RoundSwing))
		})
	}

	theme.Card(c, func() {
		// Side-split & Eco-Adjusted toolbar
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.MutedText(c, "阵营视角:")
			for _, sf := range []struct {
				id, label string
			}{
				{"ALL", "全部 (All)"},
				{"CT", "CT 防守"},
				{"T", "T 进攻"},
			} {
				if theme.TabButton(c, sf.label, v.sideFilter == sf.id).Clicked() {
					v.sideFilter = sf.id
				}
			}
			ui.Spacer(c)
			if ui.Checkbox(c, &v.ecoAdjusted, "Eco-Adjusted 经济修正").Changed() {
				if v.ecoAdjusted && v.sortKey == "rating" {
					v.sortKey = "eco_rating"
				} else if !v.ecoAdjusted && v.sortKey == "eco_rating" {
					v.sortKey = "rating"
				}
			}
		})
		if (v.sideFilter == "CT" || v.sideFilter == "T") && !hasSideData {
			theme.MutedText(c, "提示：当前为旧版 v1 解析记录，暂无 CT/T 分阵营数据（重新解析 Demo 后可启用）。")
		}

		ratingColKey := "rating"
		ratingColLabel := "Rating"
		adrColLabel := "ADR"
		if v.ecoAdjusted {
			ratingColKey = "eco_rating"
			ratingColLabel = "Eco-Rat"
			adrColLabel = "Eco-ADR"
		}

		cols := []struct {
			key   string
			label string
		}{
			{"kills", "K"}, {"deaths", "D"}, {"assists", "A"}, {"kd", "K/D"},
			{"adr", adrColLabel}, {"kast", "KAST%"}, {"hs", "HS%"}, {"fk", "FK"},
			{"fd", "FD"}, {"tk", "TK"}, {"multi", "Multi"}, {"clutch", "1vN"},
			{"swing", "Swing"}, {ratingColKey, ratingColLabel},
		}
		theme.RowCenter(c).FillWidth().Background(theme.BgInput).Radius(4).PaddingX(6).PaddingY(6).Gap(4).Children(func() {
			ui.Text(c, "选手 (点击展开详情)").Width(150).FontSize(12).Bold().TextColor(theme.TextMuted)
			for _, col := range cols {
				fg := theme.TextMuted
				bar := ui.Transparent
				label := col.label
				if v.sortKey == col.key {
					fg = theme.AccentAmber
					bar = theme.AccentAmber
					if v.sortDesc {
						label += " ▼"
					} else {
						label += " ▲"
					}
				}
				btn := ui.ButtonBase(c).Width(52).Background(ui.Transparent).PaddingY(2).Center().Children(func() {
					ui.Column(c).AlignItems(ui.Center).Gap(3).Children(func() {
						ui.Text(c, label).Font("Rajdhani").FontSize(11).Bold().TextColor(fg)
						ui.Box(c).Width(22).Height(2).Radius(1).Background(bar)
					})
				})
				if btn.Clicked() {
					if v.sortKey == col.key {
						v.sortDesc = !v.sortDesc
					} else {
						v.sortKey = col.key
						v.sortDesc = true
					}
				}
			}
		})

		groupA, groupB := SplitPlayersByTeam(sorted, demo.TeamAName)
		if len(groupA) > 0 && len(groupB) > 0 {
			v.renderScoreboardGroup(c, demo.TeamAName, theme.TeamCT, groupA)
			v.renderScoreboardGroup(c, demo.TeamBName, theme.TeamT, groupB)
		} else {
			v.renderScoreboardGroup(c, "全场选手数据", theme.AccentAmber, sorted)
		}
	})
}

func (v *View) renderScoreboardGroup(c *ui.Context, title string, accent ui.Color, rows []storage.MatchPlayer) {
	ui.Divider(c)
	theme.RowCenter(c).FillWidth().Gap(8).PaddingY(2).Children(func() {
		theme.StatusDot(c, accent, 7)
		ui.Text(c, title).Font("Rajdhani").FontSize(12).Bold().TextColor(accent)
	})
	for idx, p := range rows {
		bg := ui.Transparent
		if idx%2 == 1 {
			bg = ui.RGBA(255, 255, 255, 0.03)
		}
		isExpanded := v.expandedPlayerSID == p.SteamID64
		adrVal := p.ADR
		ratingVal := p.StrikeyeRating
		if v.ecoAdjusted && p.EcoAdjRating > 0 {
			adrVal = p.EcoAdjADR
			ratingVal = p.EcoAdjRating
		}
		swingCol := theme.TextMuted
		if p.RoundSwing > 0 {
			swingCol = theme.StatusGreen
		} else if p.RoundSwing < 0 {
			swingCol = theme.StatusRed
		}

		rowBtn := ui.ButtonBase(c).FillWidth().Background(bg).Radius(10).PaddingX(6).PaddingY(6).Children(func() {
			theme.RowCenter(c).FillWidth().Gap(4).Children(func() {
				theme.RowCenter(c).Width(150).Gap(6).Children(func() {
					theme.StatusDot(c, accent, 6)
					ui.Text(c, p.PlayerName).FontSize(12).Bold().TextColor(theme.TextPrimary)
				})
				statCell(c, fmt.Sprintf("%d", p.Kills))
				statCell(c, fmt.Sprintf("%d", p.Deaths))
				statCell(c, fmt.Sprintf("%d", p.Assists))
				statCell(c, fmt.Sprintf("%.2f", p.KDRatio))
				statCell(c, fmt.Sprintf("%.1f", adrVal))
				statCell(c, fmt.Sprintf("%.0f%%", p.KASTPercent))
				statCell(c, fmt.Sprintf("%.0f%%", p.HSPercent))
				statCell(c, fmt.Sprintf("%d", p.FirstKills))
				statCell(c, fmt.Sprintf("%d", p.FirstDeaths))
				statCell(c, fmt.Sprintf("%d", p.TradeKills))
				statCell(c, fmt.Sprintf("%d", p.MultiKillRounds))
				statCell(c, fmt.Sprintf("%d", p.ClutchWins))
				ui.Row(c).Width(52).Center().Children(func() {
					theme.StatText(c, fmtSwing(p.RoundSwing)).FontSize(11).TextColor(swingCol)
				})
				ui.Row(c).Width(52).Center().Children(func() {
					theme.ScoreText(c, fmt.Sprintf("%.2f", ratingVal)).FontSize(13).TextColor(theme.AccentAmber)
				})
			})
		})
		if rowBtn.Clicked() {
			if isExpanded {
				v.expandedPlayerSID = ""
			} else {
				v.expandedPlayerSID = p.SteamID64
			}
		}

		if isExpanded {
			v.renderPlayerDetailDrawer(c, p)
		}
	}
}

func (v *View) renderPlayerDetailDrawer(c *ui.Context, p storage.MatchPlayer) {
	clutches := ParseClutchBreakdown(p.ClutchBreakdownJSON)
	clutchText := "无残局记录"
	if len(clutches) > 0 {
		parts := make([]string, 0, len(clutches))
		for _, cl := range clutches {
			parts = append(parts, fmt.Sprintf("%s: %d/%d", cl.Tier, cl.Wins, cl.Attempts))
		}
		clutchText = strings.Join(parts, "  |  ")
	}

	ui.Column(c).FillWidth().Background(theme.BgBase).Border(1, theme.BorderSubtle).Radius(6).Padding(10).Gap(8).Children(func() {
		theme.RowCenter(c).FillWidth().Gap(16).Children(func() {
			theme.StatText(c, fmt.Sprintf("多杀分布: 2K=%d  3K=%d  4K=%d  5K=%d", p.TwoKillRounds, p.ThreeKillRounds, p.FourKillRounds, p.FiveKillRounds)).FontSize(12).TextColor(theme.TextPrimary)
			theme.StatText(c, fmt.Sprintf("残局明细: %s", clutchText)).FontSize(12).TextColor(theme.AccentCyan)
		})
		theme.RowCenter(c).FillWidth().Gap(16).Children(func() {
			theme.StatText(c, fmt.Sprintf("道具 ADR: %.1f", p.UtilityADR)).FontSize(12).TextColor(theme.TextPrimary)
			theme.StatText(c, fmt.Sprintf("首杀胜率: %.1f%%", p.OpeningDuelWinRate)).FontSize(12).TextColor(theme.TextPrimary)
			theme.StatText(c, fmt.Sprintf("AWP 击杀: %d", p.AWPKills)).FontSize(12).TextColor(theme.TextPrimary)
			theme.StatText(c, fmt.Sprintf("闪光助攻: %d  •  致盲敌人: %d (%.1fs)", p.FlashAssists, p.EnemiesFlashed, p.TotalFlashSec)).FontSize(12).TextColor(theme.AccentAmber)
		})
	})
}

func leaderCard(c *ui.Context, title, player, val string) {
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		theme.StatTile(c, title, val, player, theme.AccentAmber)
	})
}

func statCell(c *ui.Context, val string) {
	ui.Row(c).Width(52).Center().Children(func() {
		theme.StatText(c, val).FontSize(12).TextColor(theme.TextPrimary)
	})
}

func kpiRateText(wins, total int) string {
	if total == 0 {
		return "0/0 (-)"
	}
	return fmt.Sprintf("%d/%d (%.0f%%)", wins, total, float64(wins)/float64(total)*100.0)
}

func (v *View) renderEconomy(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	rounds := v.rounds
	v.mu.RUnlock()
	if demo == nil {
		return
	}
	kpis := ComputeEconomyKPIs(rounds, demo.TeamAName)
	theme.Card(c, func() {
		theme.SectionTitle(c, "战术与经济胜率总览")
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			leaderCard(c, "手枪局 (Team A)", kpiRateText(kpis.PistolAWins, kpis.PistolTotal), fmt.Sprintf("%d 局", kpis.PistolTotal))
			leaderCard(c, "手枪后转化率", kpiRateText(kpis.ConversionAWins, kpis.ConversionTotal), fmt.Sprintf("%d 局", kpis.ConversionTotal))
			leaderCard(c, "长枪对决 (Team A)", kpiRateText(kpis.FullVsFullAWins, kpis.FullVsFullTotal), fmt.Sprintf("%d 局", kpis.FullVsFullTotal))
			leaderCard(c, "强起翻盘 (vs Full)", kpiRateText(kpis.ForceVsFullWins, kpis.ForceVsFullTotal), fmt.Sprintf("%d 局", kpis.ForceVsFullTotal))
			leaderCard(c, "Eco 翻盘 (vs Full)", kpiRateText(kpis.EcoVsFullWins, kpis.EcoVsFullTotal), fmt.Sprintf("%d 局", kpis.EcoVsFullTotal))
			leaderCard(c, "下包后胜率 (A / B)", kpiRateText(kpis.PostPlantWins, kpis.PostPlantTotal),
				fmt.Sprintf("A %d/%d · B %d/%d", kpis.SiteAWins, kpis.SiteATotal, kpis.SiteBWins, kpis.SiteBTotal))
		})
	})

	htRound := 13
	if demo.HalftimeRound != nil && *demo.HalftimeRound > 1 {
		htRound = *demo.HalftimeRound
	}

	theme.Card(c, func() {
		theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
			theme.SectionTitle(c, "回合胜负时间线 (点击回合展开详情)")
			ui.Spacer(c)
			theme.RowCenter(c).Gap(6).Children(func() {
				theme.StatusDot(c, theme.TeamCT, 7)
				theme.MutedText(c, "CT 胜")
				theme.StatusDot(c, theme.TeamT, 7)
				theme.MutedText(c, "T 胜")
			})
		})
		ui.ScrollHorizontal(c).FillWidth().Children(func() {
			theme.RowCenter(c).Gap(4).PaddingY(4).Children(func() {
				for _, r := range rounds {
					if r.RoundNumber == htRound || (r.RoundNumber == 25 && htRound == 13) {
						lbl := "HT"
						if r.RoundNumber == 25 {
							lbl = "OT"
						}
						ui.Column(c).Width(22).Height(32).Center().Children(func() {
							theme.MutedText(c, lbl)
						})
					}
					col := theme.StatusGray
					if r.WinnerSide != nil {
						if strings.EqualFold(*r.WinnerSide, "CT") {
							col = theme.TeamCT
						} else if strings.EqualFold(*r.WinnerSide, "T") {
							col = theme.TeamT
						}
					} else if r.WinnerTeamKey != nil {
						if isTeamAKey(*r.WinnerTeamKey, demo.TeamAName) {
							col = theme.TeamCT
						} else {
							col = theme.TeamT
						}
					}
					bg := theme.BgInput
					if v.selectedRoundNum == r.RoundNumber {
						bg = theme.BgSurface
					}
					btn := ui.ButtonBase(c).Width(36).Height(32).Background(bg).Border(1, col).Radius(4).Center().Children(func() {
						theme.ScoreText(c, fmt.Sprintf("R%d", r.RoundNumber)).FontSize(11).TextColor(col)
					})
					if btn.Clicked() {
						if v.selectedRoundNum == r.RoundNumber {
							v.selectedRoundNum = 0
						} else {
							v.selectedRoundNum = r.RoundNumber
						}
					}
				}
			})
		})
	})

	theme.Card(c, func() {
		theme.SectionTitle(c, "逐回合双队装备价值对比")
		const maxEquip = 28000.0
		const barMaxW float32 = 150
		theme.RowCenter(c).FillWidth().Background(theme.BgInput).Radius(4).PaddingX(8).PaddingY(6).Gap(10).Children(func() {
			ui.Text(c, "回合").Width(36).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, "胜方与方式").Width(150).FontSize(11).Bold().TextColor(theme.TextMuted)
			ui.Text(c, fmt.Sprintf("%s 装备 / 经济", demo.TeamAName)).Width(58+barMaxW+84+20).FontSize(11).Bold().TextColor(theme.TeamCT)
			ui.Text(c, fmt.Sprintf("%s 装备 / 经济", demo.TeamBName)).Width(58+barMaxW+84+20).FontSize(11).Bold().TextColor(theme.TeamT)
		})
		for idx, r := range rounds {
			sideA := strings.ToUpper(strings.TrimSpace(r.TeamASide))
			if sideA == "" {
				sideA = "CT"
			}
			sideB := strings.ToUpper(strings.TrimSpace(r.TeamBSide))
			if sideB == "" {
				sideB = "T"
			}
			colA := theme.TeamCT
			if sideA == "T" {
				colA = theme.TeamT
			}
			colB := theme.TeamT
			if sideB == "CT" {
				colB = theme.TeamCT
			}

			winner := "-"
			winCol := theme.TextMuted
			if r.WinnerTeamKey != nil {
				winner = *r.WinnerTeamKey
				if isTeamAKey(winner, demo.TeamAName) {
					winner = demo.TeamAName
					winCol = colA
				} else {
					winner = demo.TeamBName
					winCol = colB
				}
			}
			reason := "-"
			if r.WinReason != nil {
				reason = theme.WinReasonLabel(*r.WinReason)
			}
			bg := theme.BgSurface
			if idx%2 == 1 {
				bg = theme.BgInput
			}
			wA := float32(r.TeamAEquipValue) / maxEquip * barMaxW
			if wA < 4 && r.TeamAEquipValue > 0 {
				wA = 4
			}
			if wA > barMaxW {
				wA = barMaxW
			}
			wB := float32(r.TeamBEquipValue) / maxEquip * barMaxW
			if wB < 4 && r.TeamBEquipValue > 0 {
				wB = 4
			}
			if wB > barMaxW {
				wB = barMaxW
			}
			rowBtn := ui.ButtonBase(c).FillWidth().Background(bg).Radius(4).PaddingX(8).PaddingY(4).Children(func() {
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					theme.ScoreText(c, fmt.Sprintf("R%02d", r.RoundNumber)).Width(36).FontSize(13).TextColor(theme.AccentAmber)
					ui.Textf(c, "%s · %s", winner, reason).Width(150).FontSize(12).TextColor(winCol)
					theme.StatText(c, fmt.Sprintf("$%d", r.TeamAEquipValue)).Width(58).FontSize(12).TextColor(colA)
					ui.Box(c).Width(barMaxW).Height(7).Radius(4).Background(theme.BgBase).Children(func() {
						ui.Box(c).Width(wA).Height(7).Radius(4).Background(colA)
					})
					theme.MutedText(c, fmt.Sprintf("%s (%s)", theme.BuyTypeLabel(r.TeamAEconomyType), sideA)).Width(84)
					theme.StatText(c, fmt.Sprintf("$%d", r.TeamBEquipValue)).Width(58).FontSize(12).TextColor(colB)
					ui.Box(c).Width(barMaxW).Height(7).Radius(4).Background(theme.BgBase).Children(func() {
						ui.Box(c).Width(wB).Height(7).Radius(4).Background(colB)
					})
					theme.MutedText(c, fmt.Sprintf("%s (%s)", theme.BuyTypeLabel(r.TeamBEconomyType), sideB)).Width(84)
				})
			})
			if rowBtn.Clicked() {
				if v.selectedRoundNum == r.RoundNumber {
					v.selectedRoundNum = 0
				} else {
					v.selectedRoundNum = r.RoundNumber
				}
			}

			if v.selectedRoundNum == r.RoundNumber {
				v.renderRoundDetailDrawer(c, r)
			}
		}
	})
}

func resolvePlayerEconomySide(pe parser.PlayerRoundEconomy, r storage.MatchRound, teamAName string) string {
	s := strings.ToUpper(strings.TrimSpace(pe.Side))
	if s == "CT" || s == "T" {
		return s
	}
	sideA := strings.ToUpper(strings.TrimSpace(r.TeamASide))
	if sideA != "CT" && sideA != "T" {
		sideA = "CT"
	}
	sideB := strings.ToUpper(strings.TrimSpace(r.TeamBSide))
	if sideB != "CT" && sideB != "T" {
		sideB = "T"
	}
	if isTeamAKey(pe.TeamKey, teamAName) {
		return sideA
	}
	return sideB
}

func playerRemainingMoney(pe parser.PlayerRoundEconomy) int {
	if pe.MoneyRemaining > 0 {
		return pe.MoneyRemaining
	}
	if pe.AccountStart > pe.CashSpent {
		return pe.AccountStart - pe.CashSpent
	}
	return 0
}

func formatPlayerLoadout(pe parser.PlayerRoundEconomy) string {
	var parts []string
	if pe.HasHelmet {
		parts = append(parts, "全甲")
	} else if pe.HasArmor {
		parts = append(parts, "半甲")
	}
	if pe.HasDefuseKit {
		parts = append(parts, "钳")
	}
	parts = append(parts, pe.Weapons...)
	if len(parts) == 0 {
		return theme.BuyTypeLabel(pe.EconomyType)
	}
	return strings.Join(parts, " · ")
}

func (v *View) renderRoundDetailDrawer(c *ui.Context, r storage.MatchRound) {
	v.mu.RLock()
	demo := v.demo
	events := v.events
	v.mu.RUnlock()
	econRows := ParsePlayerEconomy(r.PlayerEconomyJSON)
	var roundKills []storage.MatchEvent
	for _, ev := range events {
		if ev.RoundNumber == r.RoundNumber && ev.EventType == "kill" {
			roundKills = append(roundKills, ev)
		}
	}

	teamAName := "Team A"
	teamBName := "Team B"
	if demo != nil {
		if demo.TeamAName != "" {
			teamAName = demo.TeamAName
		}
		if demo.TeamBName != "" {
			teamBName = demo.TeamBName
		}
	}

	var ctRows, tRows []parser.PlayerRoundEconomy
	for _, pe := range econRows {
		if resolvePlayerEconomySide(pe, r, teamAName) == "T" {
			tRows = append(tRows, pe)
		} else {
			ctRows = append(ctRows, pe)
		}
	}

	ctLabel := "CT 防守方"
	tLabel := "T 进攻方"
	if strings.EqualFold(r.TeamASide, "CT") {
		ctLabel = "CT · " + teamAName
		tLabel = "T · " + teamBName
	} else if strings.EqualFold(r.TeamASide, "T") {
		tLabel = "T · " + teamAName
		ctLabel = "CT · " + teamBName
	}

	renderSideEcon := func(title string, accent ui.Color, rows []parser.PlayerRoundEconomy) {
		if len(rows) == 0 {
			return
		}
		ui.Column(c).FillWidth().Background(theme.BgSurface).Border(1, accent).Radius(6).Padding(8).Gap(4).Children(func() {
			theme.RowCenter(c).FillWidth().Gap(6).Children(func() {
				theme.StatusDot(c, accent, 7)
				ui.Text(c, title).Font("Rajdhani").FontSize(12).Bold().TextColor(accent)
			})
			for _, pe := range rows {
				rem := playerRemainingMoney(pe)
				loadout := formatPlayerLoadout(pe)
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					ui.Text(c, pe.PlayerName).Width(116).FontSize(11).Bold().TextColor(accent)
					theme.StatText(c, fmt.Sprintf("装备 $%d", pe.EquipValue)).Width(74).FontSize(11).TextColor(theme.AccentCyan)
					theme.StatText(c, fmt.Sprintf("花费 $%d", pe.CashSpent)).Width(72).FontSize(11).TextColor(theme.TextMuted)
					theme.StatText(c, fmt.Sprintf("剩余 $%d", rem)).Width(72).FontSize(11).TextColor(theme.StatusGreen)
					ui.Text(c, loadout).Grow(1).MinWidth(0).FontSize(11).TextColor(theme.TextPrimary)
				})
			}
		})
	}

	ui.Column(c).FillWidth().Background(theme.BgBase).Border(1, theme.BorderSubtle).Radius(6).Padding(10).Gap(8).Children(func() {
		theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
			theme.SectionTitle(c, fmt.Sprintf("R%02d 回合详情", r.RoundNumber))
			if r.BombSite != nil && *r.BombSite != "" {
				theme.Capsule(c, "包点 "+strings.ToUpper(*r.BombSite), theme.AccentAmber)
			}
			ui.Spacer(c)
			if theme.AccentButton(c, fmt.Sprintf("跳转 2D 回放 (R%d)", r.RoundNumber)).Clicked() {
				v.SetActiveTab(TabReplay2D)
				v.loadReplayRoundAsync(r.RoundNumber)
			}
		})

		ui.Row(c).FillWidth().Gap(12).Children(func() {
			// Left: 10-player economy snapshot split by CT / T side
			ui.Column(c).Grow(3).MinWidth(0).Gap(6).Children(func() {
				theme.MutedText(c, "选手开局出装、花费与剩余金币 (CT / T 分阵营)")
				if len(econRows) == 0 {
					theme.MutedText(c, "暂无选手经济明细")
				} else {
					renderSideEcon(ctLabel, theme.TeamCT, ctRows)
					renderSideEcon(tLabel, theme.TeamT, tRows)
				}
			})

			// Right: Round kill feed
			ui.Column(c).Grow(2).MinWidth(0).Gap(5).Children(func() {
				theme.MutedText(c, "回合击杀流 (Kill Feed)")
				if len(roundKills) == 0 {
					theme.MutedText(c, "本回合无击杀记录")
				} else {
					sideColorForKey := func(tk *string) ui.Color {
						if tk == nil || *tk == "" {
							return theme.TextPrimary
						}
						side := strings.ToUpper(strings.TrimSpace(r.TeamBSide))
						if isTeamAKey(*tk, teamAName) {
							side = strings.ToUpper(strings.TrimSpace(r.TeamASide))
						}
						if side == "CT" {
							return theme.TeamCT
						}
						if side == "T" {
							return theme.TeamT
						}
						return theme.TextPrimary
					}
					for _, ev := range roundKills {
						att, vic, wep := "-", "-", ""
						if ev.ActorName != nil {
							att = *ev.ActorName
						}
						if ev.TargetName != nil {
							vic = *ev.TargetName
						}
						if ev.Weapon != nil {
							wep = *ev.Weapon
						}
						attCol := sideColorForKey(ev.ActorTeamKey)
						vicCol := sideColorForKey(ev.TargetTeamKey)

						theme.RowCenter(c).FillWidth().Background(theme.BgSurface).Border(1, theme.BorderSubtle).Radius(5).PaddingX(8).PaddingY(4).Gap(6).Children(func() {
							theme.StatText(c, fmt.Sprintf("%.1fs", ev.RoundTimeSec)).Width(36).FontSize(11).TextColor(theme.TextMuted)
							ui.Text(c, att).FontSize(11).Bold().TextColor(attCol)
							if ev.IsAssistedFlash {
								theme.RenderWeaponIcon(c, "flashbang_assist", 13, theme.AccentAmber)
								if ev.AssisterName != nil && *ev.AssisterName != "" {
									ui.Text(c, *ev.AssisterName).FontSize(10).TextColor(theme.TextMuted)
								}
							}
							if ev.IsAttackerBlind {
								theme.RenderWeaponIcon(c, "attacker_blind", 13, theme.AccentAmber)
							}
							if !theme.RenderWeaponIcon(c, wep, 15, theme.TextPrimary) {
								theme.StatText(c, "["+wep+"]").FontSize(11).TextColor(theme.TextMuted)
							}
							if ev.IsThroughSmoke {
								theme.RenderWeaponIcon(c, "through_smoke", 13, theme.AccentCyan)
							}
							if ev.IsWallbang {
								theme.RenderWeaponIcon(c, "wallbang", 13, theme.AccentAmber)
							}
							if ev.IsNoscope {
								theme.RenderWeaponIcon(c, "noscope", 13, theme.AccentCyan)
							}
							if ev.IsHeadshot {
								theme.RenderWeaponIcon(c, "headshot", 13, theme.StatusRed)
							}
							ui.Text(c, vic).FontSize(11).Bold().TextColor(vicCol)
						})
					}
				}
			})
		})
	})
}

func (v *View) renderPerformance(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	players := v.players
	events := v.events
	v.mu.RUnlock()
	if demo == nil {
		return
	}
	pm := BuildPerformanceMatrix(players, events, demo.TeamAName)

	renderMatrixCard := func(title string, matrix map[string]map[string]H2HCell) {
		theme.Card(c, func() {
			theme.SectionTitle(c, title)
			if len(pm.TeamA) == 0 || len(pm.TeamB) == 0 {
				theme.MutedText(c, "双方阵容人数不足，无法生成 5v5 对位矩阵")
				return
			}
			// Column header row (Team B)
			theme.RowCenter(c).FillWidth().Background(theme.BgInput).Radius(4).PaddingX(8).PaddingY(6).Gap(6).Children(func() {
				ui.Text(c, fmt.Sprintf("%s \\ %s", demo.TeamAName, demo.TeamBName)).Width(130).FontSize(11).Bold().TextColor(theme.TextMuted)
				for _, b := range pm.TeamB {
					ui.Text(c, b.PlayerName).Width(96).FontSize(11).Bold().TextColor(theme.TeamT)
				}
			})
			for idx, a := range pm.TeamA {
				bg := theme.BgSurface
				if idx%2 == 1 {
					bg = theme.BgInput
				}
				theme.RowCenter(c).FillWidth().Background(bg).Radius(4).PaddingX(8).PaddingY(5).Gap(6).Children(func() {
					ui.Text(c, a.PlayerName).Width(130).FontSize(12).Bold().TextColor(theme.TeamCT)
					for _, b := range pm.TeamB {
						cell := matrix[a.SteamID64][b.SteamID64]
						col := theme.TextMuted
						if cell.Kills > cell.Deaths {
							col = theme.StatusGreen
						} else if cell.Deaths > cell.Kills {
							col = theme.StatusRed
						}
						ui.Row(c).Width(96).Children(func() {
							theme.StatText(c, fmt.Sprintf("%d : %d", cell.Kills, cell.Deaths)).FontSize(12).TextColor(col)
						})
					}
				})
			}
		})
	}

	renderMatrixCard("5v5 选手互相击杀矩阵 (Head-to-Head)", pm.H2H)
	renderMatrixCard("首杀对位矩阵 (Opening Duels)", pm.OpeningH2H)

	theme.Card(c, func() {
		theme.SectionTitle(c, fmt.Sprintf("全场武器击杀分布 (总击杀 %d)", pm.TotalKills))
		if len(pm.Weapons) == 0 {
			theme.MutedText(c, "暂无击杀事件记录")
			return
		}
		limit := len(pm.Weapons)
		if limit > 12 {
			limit = 12
		}
		const barMaxW float32 = 220
		for idx, w := range pm.Weapons[:limit] {
			bg := theme.BgSurface
			if idx%2 == 1 {
				bg = theme.BgInput
			}
			hsPct := 0.0
			if w.Kills > 0 {
				hsPct = float64(w.Headshots) / float64(w.Kills) * 100.0
			}
			barW := float32(w.SharePct/100.0) * barMaxW
			if barW < 4 && w.Kills > 0 {
				barW = 4
			}
			theme.RowCenter(c).FillWidth().Background(bg).Radius(4).PaddingX(8).PaddingY(5).Gap(10).Children(func() {
				theme.RowCenter(c).Width(150).Gap(8).Children(func() {
					theme.RenderWeaponIcon(c, w.Weapon, 15, theme.TextPrimary)
					ui.Text(c, strings.ToUpper(w.Weapon)).FontSize(12).Bold().TextColor(theme.TextPrimary)
				})
				theme.StatText(c, fmt.Sprintf("%d 杀", w.Kills)).Width(64).FontSize(12).TextColor(theme.AccentAmber)
				ui.Box(c).Width(barMaxW).Height(8).Radius(4).Background(theme.BgBase).Children(func() {
					ui.Box(c).Width(barW).Height(8).Radius(4).Background(theme.AccentCyan)
				})
				theme.StatText(c, fmt.Sprintf("%.1f%%", w.SharePct)).Width(56).FontSize(11).TextColor(theme.TextMuted)
				theme.StatText(c, fmt.Sprintf("爆头 %.0f%%", hsPct)).Width(76).FontSize(11).TextColor(theme.TextPrimary)
			})
		}
	})
}

func (v *View) renderRadarWorkbench(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	players := v.players
	cards := v.cards
	selectedCardID := v.selectedCardID
	v.mu.RUnlock()
	if demo == nil {
		return
	}

	layout, _ := config.ResolveDataLayout(v.dataRoot)
	var svc *radar.Service
	if v.db != nil {
		svc = radar.NewService(v.db.Analysis(), layout)
	}

	// Side-by-side two-player comparison card
	if len(players) >= 2 {
		names := make([]string, 0, len(players))
		byName := make(map[string]storage.MatchPlayer, len(players))
		for _, p := range players {
			names = append(names, p.PlayerName)
			byName[p.PlayerName] = p
		}
		theme.Card(c, func() {
			theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
				theme.SectionTitle(c, "双选手核心数据并排对比")
				ui.Spacer(c)
				ui.Select(c, &v.compareA, names)
				theme.MutedText(c, "VS")
				ui.Select(c, &v.compareB, names)
			})
			pA, okA := byName[v.compareA]
			pB, okB := byName[v.compareB]
			if okA && okB {
				metrics := []struct {
					label string
					vA    float64
					vB    float64
					fmt   string
				}{
					{"Strikeye Rating", pA.StrikeyeRating, pB.StrikeyeRating, "%.2f"},
					{"ADR", pA.ADR, pB.ADR, "%.1f"},
					{"KAST%", pA.KASTPercent, pB.KASTPercent, "%.0f%%"},
					{"K/D", pA.KDRatio, pB.KDRatio, "%.2f"},
					{"首杀 (FK)", float64(pA.FirstKills), float64(pB.FirstKills), "%.0f"},
					{"残局 (1vN)", float64(pA.ClutchWins), float64(pB.ClutchWins), "%.0f"},
				}
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					for _, m := range metrics {
						colA, colB := theme.TextPrimary, theme.TextPrimary
						if m.vA > m.vB {
							colA = theme.TeamCT
						} else if m.vB > m.vA {
							colB = theme.TeamT
						}
						ui.Column(c).FillWidth().Background(theme.BgInput).Border(1, theme.BorderSubtle).Radius(6).Padding(8).Gap(4).Children(func() {
							theme.MutedText(c, m.label)
							theme.RowCenter(c).FillWidth().Gap(6).Children(func() {
								theme.ScoreText(c, fmt.Sprintf(m.fmt, m.vA)).FontSize(15).TextColor(colA)
								theme.MutedText(c, "/")
								theme.ScoreText(c, fmt.Sprintf(m.fmt, m.vB)).FontSize(15).TextColor(colB)
							})
						})
					}
				})
			}
		})
	}

	theme.Card(c, func() {
		theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
			if theme.AccentButton(c, "一键生成 10 人战力雷达图 (2560x1440)").Clicked() && svc != nil {
				demoID := demo.ID
				v.setOpProgress(true, -1, "正在生成 10 人六维战力雷达图...")
				go func() {
					genCards, err := svc.GenerateMatchCards(context.Background(), demoID)
					if err != nil {
						v.mu.Lock()
						v.statusMsg = "生成失败: " + err.Error()
						v.mu.Unlock()
					} else {
						var firstPath string
						var firstID string
						if len(genCards) > 0 {
							firstID = genCards[0].ID
							firstPath = genCards[0].StaticImagePath
						}
						if firstPath != "" {
							v.radarPreview.SetCard(firstPath, true)
						}
						v.mu.Lock()
						v.cards = genCards
						if firstID != "" {
							v.selectedCardID = firstID
						}
						v.statusMsg = fmt.Sprintf("已生成 %d 张雷达图", len(genCards))
						v.mu.Unlock()
					}
					v.setOpProgress(false, 1.0, "")
				}()
			}
			if theme.ActionButton(c, "批量生成 4.0s 开场动画 MP4").Clicked() && svc != nil {
				demoID := demo.ID
				cardsCopy := append([]storage.RadarCard(nil), cards...)
				v.setOpProgress(true, 0, "正在导出 4.0s 开场动画...")
				go func() {
					total := len(cardsCopy)
					for idx, card := range cardsCopy {
						if total > 0 {
							v.setOpProgress(true, float64(idx)/float64(total), fmt.Sprintf("正在导出开场动画: %s (%d/%d)", card.PlayerName, idx+1, total))
						}
						_, _ = svc.GenerateCardAnimation(context.Background(), demoID, card.SteamID64, radar.RenderAnimOptions{
							FFmpegExe: v.ffmpegPath,
						})
					}
					refreshed, _ := v.db.Analysis().ListRadarCards(context.Background(), demoID)
					v.mu.Lock()
					v.cards = refreshed
					v.statusMsg = "4.0s 开场动画批量导出完成"
					v.mu.Unlock()
					v.setOpProgress(false, 1.0, "")
				}()
			}
		})

		// Player selector + custom avatar/logo binding
		ui.ScrollHorizontal(c).FillWidth().Children(func() {
			theme.RowCenter(c).Gap(8).Children(func() {
				for _, card := range cards {
					if theme.TabButton(c, card.PlayerName, card.ID == selectedCardID).Clicked() {
						v.mu.Lock()
						v.selectedCardID = card.ID
						v.mu.Unlock()
						selectedCardID = card.ID
						v.radarPreview.SetCardAsync(card.StaticImagePath, false, v.invalidate)
					}
				}
			})
		})

		var activeCard *storage.RadarCard
		for i := range cards {
			if cards[i].ID == selectedCardID {
				activeCard = &cards[i]
				break
			}
		}

		if activeCard != nil {
			theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
				ui.TextInput(c, &v.portraitInput).Width(240).Placeholder("自定义写真头像 PNG/JPG 路径")
				ui.TextInput(c, &v.logoInput).Width(240).Placeholder("自定义战队 Logo PNG 路径")
				if theme.AccentButton(c, "绑定并重绘卡片").Clicked() && svc != nil {
					var pPtr, lPtr *string
					if strings.TrimSpace(v.portraitInput) != "" {
						s := strings.TrimSpace(v.portraitInput)
						pPtr = &s
					}
					if strings.TrimSpace(v.logoInput) != "" {
						s := strings.TrimSpace(v.logoInput)
						lPtr = &s
					}
					demoID := demo.ID
					steamID := activeCard.SteamID64
					v.setOpProgress(true, -1, "正在重绘选手雷达图卡片...")
					go func() {
						updated, err := svc.UpdateCardAppearance(context.Background(), demoID, steamID, pPtr, lPtr, radar.RenderAnimOptions{
							FFmpegExe: v.ffmpegPath,
						})
						if err != nil {
							v.mu.Lock()
							v.statusMsg = "重绘失败: " + err.Error()
							v.mu.Unlock()
						} else {
							refreshed, _ := v.db.Analysis().ListRadarCards(context.Background(), demoID)
							v.radarPreview.SetCard(updated.StaticImagePath, true)
							v.mu.Lock()
							v.cards = refreshed
							v.statusMsg = "已重绘雷达图: " + updated.PlayerName
							v.mu.Unlock()
						}
						v.setOpProgress(false, 1.0, "")
					}()
				}
			})
		}

		if activeCard == nil {
			ui.Column(c).FillWidth().Height(220).Background(theme.BgInput).Border(1, theme.BorderSubtle).Radius(8).Center().Gap(8).Children(func() {
				theme.StatusDot(c, theme.AccentAmber, 8)
				theme.SectionTitle(c, "尚未生成 10 人战力雷达图")
				theme.MutedText(c, "点击上方「一键生成 10 人战力雷达图 (2560x1440)」即可生成六维战力卡，并支持绑定自定义写真头像与战队 Logo。")
			})
		} else {
			v.radarPreview.Render(c, activeCard, 640, 360)
		}
	})
}

// advanceReplay advances 2D replay playback by elapsed wall-clock time scaled by replaySpeed.
func (v *View) advanceReplay(elapsed time.Duration) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.advanceReplayLocked(elapsed)
}

func (v *View) advanceReplayLocked(elapsed time.Duration) bool {
	if !v.replayPlaying || len(v.replayRound.Frames) == 0 || elapsed <= 0 {
		return false
	}
	spd := v.replaySpeed
	if spd <= 0 {
		spd = 1.0
	}
	v.replayFramePos += elapsed.Seconds() * 32.0 * spd
	nextIdx := int(v.replayFramePos)
	if nextIdx >= len(v.replayRound.Frames) {
		v.replayFrameIdx = 0
		v.replayFramePos = 0
		v.replayPlaying = false
		return true
	}
	if nextIdx != v.replayFrameIdx {
		v.replayFrameIdx = nextIdx
		return true
	}
	return false
}

func (v *View) renderReplay2D(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	replayIndex := v.replayIndex
	replayRoundNum := v.replayRoundNum
	replayPlaying := v.replayPlaying
	replaySpeed := v.replaySpeed
	replayFrameIdx := v.replayFrameIdx
	frameCount := len(v.replayRound.Frames)
	replayLoading := v.replayLoading
	v.mu.RUnlock()
	if demo == nil {
		return
	}

	theme.Card(c, func() {
		ui.Row(c).FillWidth().Gap(16).Children(func() {
			// Left 560x560 canvas
			v.radarCanvas.RenderUI(c, 560)

			// Right tactical control & legend panel
			ui.Column(c).Fill().Gap(12).Children(func() {
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					theme.SectionTitle(c, "2D 战术雷达回放")
					if replayLoading {
						theme.PillBadge(c, "加载中...", theme.AccentCyan)
					}
				})
				theme.MutedText(c, "回合选择:")
				if len(replayIndex) == 0 {
					theme.RowCenter(c).FillWidth().Background(theme.BgInput).Radius(6).PaddingX(10).PaddingY(8).Gap(8).Children(func() {
						theme.StatusDot(c, theme.StatusGray, 6)
						theme.MutedText(c, "暂无 32Hz 回放索引（需先完成 Demo 解析）")
					})
				} else {
					ui.ScrollHorizontal(c).FillWidth().Children(func() {
						theme.RowCenter(c).Gap(6).Children(func() {
							for _, ent := range replayIndex {
								if theme.TabButton(c, fmt.Sprintf("R%d", ent.RoundNumber), int(ent.RoundNumber) == replayRoundNum).Clicked() {
									v.loadReplayRoundAsync(int(ent.RoundNumber))
								}
							}
						})
					})
				}

				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					playLbl := "播放"
					if replayPlaying {
						playLbl = "暂停"
					}
					if theme.AccentButton(c, playLbl).Clicked() {
						v.mu.Lock()
						if v.replayPlaying {
							v.stopPlaybackLocked()
						} else {
							v.startPlaybackLocked()
						}
						v.mu.Unlock()
					}
					for _, spd := range []float64{0.5, 1.0, 2.0, 4.0, 8.0} {
						lbl := fmt.Sprintf("%.2gx", spd)
						btn := theme.ActionButton(c, lbl)
						activeSpd := replaySpeed
						if activeSpd <= 0 {
							activeSpd = 1.0
						}
						if activeSpd == spd {
							btn = btn.Border(1, theme.AccentAmber)
						}
						if btn.Clicked() {
							v.mu.Lock()
							v.replaySpeed = spd
							v.mu.Unlock()
						}
					}
					if demo.MapName != nil && maps.HasLowerLayer(*demo.MapName) {
						if ui.Checkbox(c, &v.lowerLayer, "下层地图 (Lower)").Changed() {
							v.renderReplayFrameAsync(replayFrameIdx, true)
						}
					}
					ui.Spacer(c)
					theme.StatText(c, fmt.Sprintf("帧 %d / %d", replayFrameIdx, maxInt(0, frameCount-1))).
						FontSize(12).TextColor(theme.AccentCyan)
				})

				frameVal := float64(replayFrameIdx)
				maxF := float64(maxInt(1, frameCount-1))
				if ui.Slider(c, &frameVal, 0, maxF).FillWidth().Changed() {
					newIdx := int(frameVal)
					v.mu.Lock()
					v.stopPlaybackLocked()
					v.replayFrameIdx = newIdx
					v.replayFramePos = frameVal
					v.mu.Unlock()
					v.renderReplayFrameAsync(newIdx, false)
				}

				// Tactical legend
				ui.Column(c).FillWidth().Background(theme.BgInput).Border(1, theme.RimBorder).Radius(14).Padding(12).Gap(6).Children(func() {
					theme.MutedText(c, "图例说明")
					theme.RowCenter(c).Gap(12).Children(func() {
						theme.StatusDot(c, theme.TeamCT, 8)
						theme.MutedText(c, "CT 阵营")
						theme.StatusDot(c, theme.TeamT, 8)
						theme.MutedText(c, "T 阵营")
						theme.StatusDot(c, theme.StatusRed, 8)
						theme.MutedText(c, "C4 携带者")
					})
					theme.RowCenter(c).Gap(12).Children(func() {
						theme.StatusDot(c, theme.StatusGreen, 8)
						theme.MutedText(c, "生命值环")
						theme.StatusDot(c, theme.AccentCyan, 8)
						theme.MutedText(c, "投掷物轨迹")
					})
				})
			})
		})
	})
}

func (v *View) renderHeatmaps(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	players := v.players
	heatmapLoading := v.heatmapLoading
	sampleCount := v.heatmapSampleCount
	v.mu.RUnlock()
	if demo == nil {
		return
	}

	playerOpts := []string{"全部选手"}
	for _, p := range players {
		playerOpts = append(playerOpts, p.PlayerName)
	}

	theme.Card(c, func() {
		ui.Row(c).FillWidth().Gap(16).Children(func() {
			// Left 560x560 heatmap canvas
			v.heatmapCanvas.RenderUI(c, 560)

			// Right filter & density legend panel
			ui.Column(c).Fill().Gap(12).Children(func() {
				theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
					theme.SectionTitle(c, "KDE 战术热力图")
					if heatmapLoading {
						theme.PillBadge(c, "计算中...", theme.AccentCyan)
					} else {
						theme.PillBadge(c, fmt.Sprintf("采样点 %d", sampleCount), theme.AccentAmber)
					}
				})
				theme.MutedText(c, "通过核密度估计 (KDE) 叠加展示交火、击杀与站位分布。")

				ui.Column(c).FillWidth().Background(theme.BgInput).Border(1, theme.RimBorder).Radius(14).Padding(14).Gap(10).Children(func() {
					theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
						theme.MutedText(c, "分析维度").Width(72)
						if ui.Select(c, &v.hmMode, []string{"position", "firefight", "kill", "death"}).Width(220).Changed() {
							v.refreshHeatmapAsync(false)
						}
					})
					theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
						theme.MutedText(c, "目标选手").Width(72)
						if ui.Select(c, &v.hmPlayer, playerOpts).Width(220).Changed() {
							v.refreshHeatmapAsync(false)
						}
					})
					theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
						theme.MutedText(c, "阵营筛选").Width(72)
						if ui.Select(c, &v.hmTeam, []string{"全部阵营", "T", "CT"}).Width(220).Changed() {
							v.refreshHeatmapAsync(false)
						}
					})
					theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
						theme.MutedText(c, "回合类型").Width(72)
						if ui.Select(c, &v.hmRoundType, []string{"全部回合", "手枪局", "长枪局", "Eco"}).Width(220).Changed() {
							v.refreshHeatmapAsync(false)
						}
					})
					if demo.MapName != nil && maps.HasLowerLayer(*demo.MapName) {
						if ui.Checkbox(c, &v.lowerLayer, "切换下层地图 (Lower)").Changed() {
							v.refreshHeatmapAsync(true)
						}
					}
					theme.RowCenter(c).FillWidth().Children(func() {
						if theme.AccentButton(c, "重算热力图").Clicked() {
							v.refreshHeatmapAsync(false)
						}
					})
				})

				// Thermal density legend
				ui.Column(c).FillWidth().Background(theme.BgInput).Border(1, theme.BorderSubtle).Radius(6).Padding(10).Gap(6).Children(func() {
					theme.MutedText(c, "热力密度色阶")
					theme.RowCenter(c).FillWidth().Gap(4).Children(func() {
						theme.MutedText(c, "低")
						ui.Box(c).FillWidth().Height(10).Radius(3).Background(theme.AccentCyan)
						ui.Box(c).FillWidth().Height(10).Radius(3).Background(theme.AccentAmber)
						ui.Box(c).FillWidth().Height(10).Radius(3).Background(theme.StatusRed)
						theme.MutedText(c, "高")
					})
				})
			})
		})
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
