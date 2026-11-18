package parser

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"strikeye/internal/storage"
)

const (
	rapidKillWindowSec  = 10.0
	burstKillInterval   = 3.0
	longRangeKillDist   = 1500.0 // aligned with legacy tag_constants.py _HIGHLIGHT_LONG_RANGE_DIST
	lowHPThreshold      = 15
)

// DefuseEvent captures a bomb defusal for highlight detection.
type DefuseEvent struct {
	RoundNumber   int
	Tick          int64
	PlayerSteamID string
	PlayerName    string
	TeamKey       string
	TimeRemaining float64
	EnemiesAlive  int
}

// HighlightInput holds the parsed round, kill, clutch, and defuse data for one demo.
type HighlightInput struct {
	ContentSHA256 string
	TickRate      float64
	HalftimeRound *int
	PreBufferSec  float64
	PostBufferSec float64
	Rounds        []*RoundWindow
	Kills         []KillEvent
	Clutches      []RoundClutchResult
	Defuses       []DefuseEvent
	CreatedAt     string
}

var tagCoverageRules = [][2]string{
	{"爆头", "💥 颗秒"},
	{"爆头", "枪枪爆头"},
	{"🙈 盲狙", "✈️ 飞天盲狙"},
	{"🔙 偷背身", "🔙 背刺"},
}

// DeduplicateTags preserves tag order while removing exact duplicates and
// suppressing broader tags covered by more specific ones.
func DeduplicateTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	present := make(map[string]bool, len(tags))
	for _, t := range tags {
		present[t] = true
	}
	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t == "" || seen[t] {
			continue
		}
		covered := false
		for _, rule := range tagCoverageRules {
			if t == rule[0] && present[rule[1]] {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// DetectCompetitiveHighlights identifies competitive highlight clips and assigns
// deduplicated tactical/skill tags while excluding fail and meme tags.
func DetectCompetitiveHighlights(in HighlightInput) []storage.HighlightClip {
	tickRate := in.TickRate
	if tickRate <= 0 {
		tickRate = 64.0
	}
	preSec := in.PreBufferSec
	if preSec <= 0 {
		preSec = 3.0
	}
	postSec := in.PostBufferSec
	if postSec <= 0 {
		postSec = 2.0
	}
	preTicks := int64(math.Round(preSec * tickRate))
	postTicks := int64(math.Round(postSec * tickRate))
	rapidTicks := int64(math.Round(rapidKillWindowSec * tickRate))
	burstTicks := int64(math.Round(burstKillInterval * tickRate))

	roundByNum := make(map[int]*RoundWindow, len(in.Rounds))
	for _, r := range in.Rounds {
		roundByNum[r.RoundNumber] = r
	}

	clutchByRoundPlayer := make(map[string]RoundClutchResult)
	for _, c := range in.Clutches {
		if c.Won {
			clutchByRoundPlayer[fmt.Sprintf("%d:%s", c.RoundNumber, c.SteamID64)] = c
		}
	}

	defuseByRoundPlayer := make(map[string]DefuseEvent)
	for _, d := range in.Defuses {
		defuseByRoundPlayer[fmt.Sprintf("%d:%s", d.RoundNumber, d.PlayerSteamID)] = d
	}

	// Group valid non-team kills by round -> player, preserving tick order.
	type roundPlayerKey struct {
		round int
		steam string
	}
	killsByRP := make(map[roundPlayerKey][]KillEvent)
	var rpOrder []roundPlayerKey
	firstKillOfRound := make(map[int]string) // round -> attacker steamID
	teamKillsInRound := make(map[roundPlayerKey]int)

	for _, k := range in.Kills {
		if k.TeamKill || k.AttackerSteamID == "" || k.AttackerSteamID == k.VictimSteamID {
			continue
		}
		if _, exists := firstKillOfRound[k.RoundNumber]; !exists {
			firstKillOfRound[k.RoundNumber] = k.AttackerSteamID
		}
		tkKey := roundPlayerKey{round: k.RoundNumber, steam: k.AttackerTeamKey}
		teamKillsInRound[tkKey]++

		rp := roundPlayerKey{round: k.RoundNumber, steam: k.AttackerSteamID}
		if _, exists := killsByRP[rp]; !exists {
			rpOrder = append(rpOrder, rp)
		}
		killsByRP[rp] = append(killsByRP[rp], k)
	}

	var clips []storage.HighlightClip

	for _, rp := range rpOrder {
		rKills := killsByRP[rp]
		r := roundByNum[rp.round]
		if r == nil || len(rKills) == 0 {
			continue
		}

		// Segment kills by rapidKillWindowSec into candidate highlight windows.
		var segments [][]KillEvent
		cur := []KillEvent{rKills[0]}
		for i := 1; i < len(rKills); i++ {
			if rKills[i].Tick-rKills[i-1].Tick <= rapidTicks {
				cur = append(cur, rKills[i])
			} else {
				segments = append(segments, cur)
				cur = []KillEvent{rKills[i]}
			}
		}
		segments = append(segments, cur)

		clutch, hasWonClutch := clutchByRoundPlayer[fmt.Sprintf("%d:%s", rp.round, rp.steam)]
		defuse, hasDefuse := defuseByRoundPlayer[fmt.Sprintf("%d:%s", rp.round, rp.steam)]

		for segIdx, seg := range segments {
			tags, score, clipType := evaluateSegment(seg, r, in.HalftimeRound, firstKillOfRound[rp.round] == rp.steam, teamKillsInRound[roundPlayerKey{rp.round, seg[0].AttackerTeamKey}], burstTicks, hasWonClutch && segIdx == len(segments)-1, clutch, hasDefuse && segIdx == len(segments)-1, defuse)
			if len(seg) < 2 && len(tags) == 0 {
				// Single ordinary kill without any competitive highlight criteria -> skip.
				continue
			}
			if len(seg) >= 2 && clipType == "" {
				clipType = fmt.Sprintf("%dK", len(seg))
			}

			anchorTicks := make([]int64, 0, len(seg))
			for _, k := range seg {
				anchorTicks = append(anchorTicks, k.Tick)
			}
			startTick := seg[0].Tick - preTicks
			if startTick < r.StartTick {
				startTick = r.StartTick
			}
			endTick := seg[len(seg)-1].Tick + postTicks
			if r.EndTick > r.StartTick && endTick > r.EndTick {
				endTick = r.EndTick
			}

			anchorsJSON, _ := json.Marshal(anchorTicks)
			tagsJSON, _ := json.Marshal(DeduplicateTags(tags))
			weapon := seg[0].Weapon

			clips = append(clips, storage.HighlightClip{
				ID:              highlightClipID(in.ContentSHA256, rp.round, rp.steam, segIdx),
				PlayerSteamID64: rp.steam,
				PlayerName:      seg[0].AttackerName,
				RoundNumber:     rp.round,
				ClipType:        clipType,
				KillCount:       len(seg),
				StartTick:       startTick,
				EndTick:         endTick,
				AnchorTicksJSON: string(anchorsJSON),
				PreBufferSec:    preSec,
				PostBufferSec:   postSec,
				TagsJSON:        string(tagsJSON),
				WeaponLabel:     &weapon,
				ImportanceScore: score,
				CreatedAt:       in.CreatedAt,
			})
		}
	}

	return clips
}

func evaluateSegment(
	seg []KillEvent,
	r *RoundWindow,
	halftimeRound *int,
	isFirstKill bool,
	totalTeamKills int,
	burstTicks int64,
	hasWonClutch bool,
	clutch RoundClutchResult,
	hasDefuse bool,
	defuse DefuseEvent,
) ([]string, float64, string) {
	var tags []string
	score := float64(len(seg)) * 20.0
	clipType := ""

	switch {
	case len(seg) >= 5:
		tags = append(tags, "五杀 (ACE)")
		clipType = "ace"
		score += 40
	case len(seg) == 4:
		tags = append(tags, "四杀")
		clipType = "4k"
		score += 30
	case len(seg) == 3:
		tags = append(tags, "三杀")
		clipType = "3k"
		score += 20
	case len(seg) == 2:
		tags = append(tags, "双杀")
		clipType = "2k"
		score += 10
	}

	if len(seg) >= 2 {
		allHS := true
		allBurst := true
		for i, k := range seg {
			if !k.IsHeadshot {
				allHS = false
			}
			if i > 0 && k.Tick-seg[i-1].Tick > burstTicks {
				allBurst = false
			}
		}
		if allBurst {
			tags = append(tags, "爆发刷屏")
			score += 8
		}
		if allHS {
			tags = append(tags, "枪枪爆头")
			score += 8
		}
	}

	if len(seg) >= 5 && totalTeamKills == len(seg) {
		tags = append(tags, "🧹 清盘")
		score += 15
	}
	if isFirstKill && len(seg) >= 2 {
		tags = append(tags, "⚔️ 首杀")
		score += 5
	}

	if hasWonClutch {
		if clutch.MaxEnemies >= 2 {
			tags = append(tags, "🔥 1vN 史诗残局")
			clipType = "clutch"
			score += 35
		} else if clutch.MaxEnemies == 1 {
			tags = append(tags, "🐂 1v1 斗牛")
			if clipType == "" {
				clipType = "clutch"
			}
			score += 20
		}
	}

	if hasDefuse {
		if defuse.TimeRemaining > 0 && defuse.TimeRemaining < 1.0 {
			tags = append(tags, "⏱️ 极限拆包")
			if clipType == "" {
				clipType = "defuse"
			}
			score += 25
		}
		if defuse.EnemiesAlive > 0 {
			tags = append(tags, "🥷 忍者偷包")
			if clipType == "" {
				clipType = "defuse"
			}
			score += 25
		}
	}

	// Per-kill tactical and weapon skill tags.
	for _, k := range seg {
		w := strings.ToLower(k.Weapon)
		isKnife := strings.Contains(w, "knife") || strings.Contains(w, "bayonet") || w == "karambit"
		isSniper := w == "awp" || w == "ssg08" || w == "scar20" || w == "g3sg1"
		isLongRangeWep := isSniper || w == "deagle" || w == "revolver"

		if k.IsHeadshot {
			tags = append(tags, "爆头")
			if !isSniper && (w == "ak47" || w == "deagle" || w == "m4a1" || w == "m4a1_silencer" || w == "usp_silencer" || w == "glock") {
				tags = append(tags, "💥 颗秒")
				if clipType == "" {
					clipType = "one_tap"
				}
				score += 10
			}
		}
		if isKnife {
			tags = append(tags, "🔪 刀杀")
			if clipType == "" {
				clipType = "knife"
			}
			score += 20
			if !k.VictimFacing {
				tags = append(tags, "🔙 背刺")
				score += 8
			}
		}
		if isLongRangeWep && k.Distance >= longRangeKillDist {
			tags = append(tags, "🔭 百步穿杨")
			if clipType == "" {
				clipType = "long_range"
			}
			score += 12
		}
		if isSniper && k.IsNoscope {
			if k.IsAirborne {
				tags = append(tags, "✈️ 飞天盲狙")
				score += 25
			} else {
				tags = append(tags, "🙈 盲狙")
				score += 15
			}
			if clipType == "" {
				clipType = "noscope"
			}
		} else if k.IsAirborne {
			tags = append(tags, "🪂 跳杀")
			if clipType == "" {
				clipType = "airborne"
			}
			score += 12
		}
		if k.IsWallbang {
			tags = append(tags, "🧱 穿墙杀")
			score += 10
		}
		if k.IsThroughSmoke {
			tags = append(tags, "🌫️ 混烟")
			score += 10
		}
		if k.IsAttackerBlind {
			tags = append(tags, "😎 全白反杀")
			score += 12
		}
		if k.IsAssistedFlash {
			tags = append(tags, "🤝 好闪配好人")
			score += 6
		}
	}

	lastKill := seg[len(seg)-1]
	if lastKill.AttackerHP > 0 && lastKill.AttackerHP <= lowHPThreshold && (len(seg) >= 2 || hasWonClutch) {
		tags = append(tags, "❤️ 极限锁血")
		score += 10
	}

	isPistolRound := r.RoundNumber == 1 || (halftimeRound != nil && r.RoundNumber == *halftimeRound)
	if isPistolRound && len(seg) >= 2 {
		tags = append(tags, "🔫 手枪局专家")
		score += 8
	}
	if r.RoundNumber > 24 && (len(seg) >= 2 || hasWonClutch) {
		tags = append(tags, "⚔️ 加时生死战")
		score += 10
	}

	// Eco upset check.
	if len(seg) >= 2 && r.WinnerTeamKey != nil && *r.WinnerTeamKey == seg[0].AttackerTeamKey {
		attEcon, enemyEcon := r.TeamAEconomyType, r.TeamBEconomyType
		if seg[0].AttackerTeamKey == "team_b" {
			attEcon, enemyEcon = r.TeamBEconomyType, r.TeamAEconomyType
		}
		if (attEcon == EconomyEco || attEcon == EconomySemi) && enemyEcon == EconomyFull {
			tags = append(tags, "💸 ECO翻盘")
			score += 15
		}
	}

	return DeduplicateTags(tags), score, clipType
}

func highlightClipID(sha string, roundNum int, steamID string, segIdx int) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s:%d:%s:%d", sha, roundNum, steamID, segIdx)))
	return "hl_" + hex.EncodeToString(h[:8])
}
