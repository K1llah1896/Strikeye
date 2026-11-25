package parser

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"strikeye/internal/storage"
)

// KillEvent captures a single kill within an official round for metrics and highlights.
type KillEvent struct {
	RoundNumber     int
	Tick            int64
	RoundTimeSec    float64
	AttackerSteamID string
	AttackerName    string
	AttackerTeamKey string
	VictimSteamID   string
	VictimName      string
	VictimTeamKey   string
	AssisterSteamID string
	AssisterName    string
	Weapon          string
	IsHeadshot      bool
	IsWallbang      bool
	IsThroughSmoke  bool
	IsNoscope       bool
	IsAttackerBlind bool
	IsAssistedFlash bool
	IsAirborne      bool
	Distance        float64
	AttackerHP      int
	VictimFacing    bool
	TeamKill        bool
}

// DamageEvent captures a single PlayerHurt event within an official round.
type DamageEvent struct {
	RoundNumber     int
	Tick            int64
	AttackerSteamID string
	AttackerTeamKey string
	VictimSteamID   string
	VictimTeamKey   string
	Weapon          string
	RawHealthDamage int
}

// RoundClutchResult records a 1vN clutch attempt detected during a round.
type RoundClutchResult struct {
	RoundNumber  int
	SteamID64    string
	TeamKey      string
	MaxEnemies   int
	Won          bool
	LastKillTick int64
}

// PlayerMeta holds static/latest identity info for a match participant.
type PlayerMeta struct {
	SteamID64  string
	PlayerName string
	TeamKey    string
	SpecSlot   *int
}

// FlashEvent captures a single enemy flashbang blind event within an official round.
type FlashEvent struct {
	RoundNumber     int
	Tick            int64
	AttackerSteamID string
	AttackerTeamKey string
	VictimSteamID   string
	VictimTeamKey   string
	DurationSec     float64
}

// MetricsInput provides all official round data needed to compute player statistics.
type MetricsInput struct {
	TickRate             float64
	TotalRounds          int
	Players              []PlayerMeta
	Rounds               []*RoundWindow
	Kills                []KillEvent
	Damages              []DamageEvent
	Flashes              []FlashEvent
	TTDSamples           []TTDSample
	BombPlantTickByRound map[int]int64
	EquipValues          map[string][]int // steamID64 -> equip values at freeze end
}

// MetricsResult holds computed MatchPlayer rows, side-split rows, and per-round clutch records.
type MetricsResult struct {
	Players  []storage.MatchPlayer
	Sides    []storage.MatchPlayerSide
	Clutches []RoundClutchResult
}

var utilityWeapons = map[string]bool{
	"hegrenade":  true,
	"inferno":    true,
	"molotov":    true,
	"incgrenade": true,
}

type clutchTierCount struct {
	Attempts int `json:"attempts"`
	Wins     int `json:"wins"`
}

type statAcc struct {
	roundsPlayed    int
	kills           int
	deaths          int
	assists         int
	headshots       int
	awpKills        int
	totalDamage     int
	utilityDamage   int
	firstKills      int
	firstDeaths     int
	tradeKills      int
	tradeDeaths     int
	survivedRounds  int
	kastRounds      int
	oneKillRounds   int
	twoKillRounds   int
	threeKillRounds int
	fourKillRounds  int
	fiveKillRounds  int
	multiKillRounds int
	clutchAttempts  int
	clutchWins      int
	clutchByTier    map[int]*clutchTierCount
	flashAssists    int
	enemiesFlashed  int
	totalFlashSec   float64
	roundSwingSum   float64
	ttdMS           float64
	ttdSamples      int
	ecoAdjKills     float64
	ecoAdjDeaths    float64
	ecoAdjAssists   float64
	ecoAdjDamage    float64
	ttkTicks        int64
	ttkSamples      int
}

func (a *statAcc) recordClutch(enemies int, won bool) {
	a.clutchAttempts++
	if won {
		a.clutchWins++
	}
	if enemies < 1 {
		enemies = 1
	}
	if enemies > 5 {
		enemies = 5
	}
	if a.clutchByTier == nil {
		a.clutchByTier = make(map[int]*clutchTierCount)
	}
	tc := a.clutchByTier[enemies]
	if tc == nil {
		tc = &clutchTierCount{}
		a.clutchByTier[enemies] = tc
	}
	tc.Attempts++
	if won {
		tc.Wins++
	}
}

type derivedMetrics struct {
	kd, kpr, dpr, apr, adr, uadr, kast, hsPct, openWR, survRate, impact, rating float64
	roundSwing, ecoAdjRating, ecoAdjADR                                         float64
}

func computeDerivedMetrics(a *statAcc, roundsDenom float64) derivedMetrics {
	var d derivedMetrics
	if a.deaths > 0 {
		d.kd = roundTo(float64(a.kills)/float64(a.deaths), 2)
	} else if a.kills > 0 {
		d.kd = float64(a.kills)
	}
	if a.kills > 0 {
		d.hsPct = roundTo(float64(a.headshots)/float64(a.kills)*100.0, 1)
	}
	if a.firstKills+a.firstDeaths > 0 {
		d.openWR = roundTo(float64(a.firstKills)/float64(a.firstKills+a.firstDeaths)*100.0, 1)
	}
	if roundsDenom > 0 {
		d.kpr = float64(a.kills) / roundsDenom
		d.dpr = float64(a.deaths) / roundsDenom
		d.apr = float64(a.assists) / roundsDenom
		d.adr = roundTo(float64(a.totalDamage)/roundsDenom, 1)
		d.uadr = roundTo(float64(a.utilityDamage)/roundsDenom, 1)
		d.kast = roundTo(float64(a.kastRounds)/roundsDenom*100.0, 1)
		d.survRate = roundTo(float64(a.survivedRounds)/roundsDenom*100.0, 1)
		d.impact = 2.13*d.kpr + 0.42*d.apr - 0.41
		d.rating = roundTo(0.0073*d.kast+0.3591*d.kpr-0.5329*d.dpr+0.2372*d.impact+0.0032*d.adr+0.1587, 2)

		d.roundSwing = roundTo(a.roundSwingSum/roundsDenom, 2)
		ecoKPR := a.ecoAdjKills / roundsDenom
		ecoDPR := a.ecoAdjDeaths / roundsDenom
		ecoAPR := a.ecoAdjAssists / roundsDenom
		ecoImpact := 2.13*ecoKPR + 0.42*ecoAPR - 0.41
		d.ecoAdjADR = roundTo(a.ecoAdjDamage/roundsDenom, 1)
		d.ecoAdjRating = roundTo(0.0073*d.kast+0.3591*ecoKPR-0.5329*ecoDPR+0.2372*ecoImpact+0.0032*d.ecoAdjADR+0.1587+0.015*d.roundSwing, 2)
	}
	return d
}

// ctWinProbTable[ctAlive][tAlive] approximates CT round win probability before bomb plant.
var ctWinProbTable = [6][6]float64{
	{0.50, 0.00, 0.00, 0.00, 0.00, 0.00},
	{1.00, 0.50, 0.28, 0.14, 0.06, 0.02},
	{1.00, 0.72, 0.50, 0.32, 0.18, 0.09},
	{1.00, 0.86, 0.68, 0.50, 0.34, 0.20},
	{1.00, 0.94, 0.82, 0.66, 0.50, 0.35},
	{1.00, 0.98, 0.91, 0.80, 0.65, 0.50},
}

func teamWinProb(ctAlive, tAlive int, bombPlanted bool, forSide string) float64 {
	ctAlive = clampIntVal(ctAlive, 0, 5)
	tAlive = clampIntVal(tAlive, 0, 5)
	ctProb := ctWinProbTable[ctAlive][tAlive]
	if bombPlanted && ctAlive > 0 {
		if tAlive == 0 {
			ctProb = 0.78 // CT still needs time to defuse
		} else {
			ctProb = math.Max(0.02, ctProb-0.18)
		}
	}
	if strings.EqualFold(forSide, "T") {
		return 1.0 - ctProb
	}
	return ctProb
}

func clampIntVal(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func roundEcoWeight(attTeamKey string, r *RoundWindow) float64 {
	if r == nil {
		return 1.0
	}
	var attEq, vicEq int
	switch attTeamKey {
	case "team_a":
		attEq, vicEq = r.TeamAEquipValue, r.TeamBEquipValue
	case "team_b":
		attEq, vicEq = r.TeamBEquipValue, r.TeamAEquipValue
	default:
		return 1.0
	}
	if attEq <= 0 || vicEq <= 0 {
		return 1.0
	}
	ratio := float64(vicEq) / float64(attEq)
	if ratio < 0.45 {
		return 0.45
	}
	if ratio > 1.65 {
		return 1.65
	}
	return ratio
}

func sideForTeamInRound(teamKey string, r *RoundWindow) string {
	if r == nil {
		return ""
	}
	switch teamKey {
	case "team_a":
		return strings.ToUpper(r.TeamASide)
	case "team_b":
		return strings.ToUpper(r.TeamBSide)
	default:
		return ""
	}
}

// ComputeMetrics calculates all player statistics and Strikeye Rating from official round events.
func ComputeMetrics(in MetricsInput) MetricsResult {
	tickRate := in.TickRate
	if tickRate <= 0 {
		tickRate = 64.0
	}
	tradeWindowTicks := int64(math.Round(5.0 * tickRate))
	duelWindowTicks := int64(math.Round(3.0 * tickRate))

	type pAcc struct {
		meta  PlayerMeta
		all   statAcc
		sides map[string]*statAcc // "CT", "T"
	}

	accMap := make(map[string]*pAcc, len(in.Players))
	order := make([]string, 0, len(in.Players))
	teamOf := make(map[string]string, len(in.Players))

	ensurePlayer := func(steamID, name, teamKey string) *pAcc {
		if steamID == "" {
			return nil
		}
		if a, ok := accMap[steamID]; ok {
			if a.meta.PlayerName == "" && name != "" {
				a.meta.PlayerName = name
			}
			if a.meta.TeamKey == "" && teamKey != "" {
				a.meta.TeamKey = teamKey
			}
			return a
		}
		a := &pAcc{
			meta: PlayerMeta{
				SteamID64:  steamID,
				PlayerName: name,
				TeamKey:    teamKey,
			},
			sides: map[string]*statAcc{
				"CT": {},
				"T":  {},
			},
		}
		accMap[steamID] = a
		order = append(order, steamID)
		if teamKey != "" {
			teamOf[steamID] = teamKey
		}
		return a
	}

	addForRound := func(p *pAcc, r *RoundWindow, fn func(s *statAcc)) {
		if p == nil {
			return
		}
		fn(&p.all)
		side := sideForTeamInRound(p.meta.TeamKey, r)
		if side == "CT" || side == "T" {
			fn(p.sides[side])
		}
	}

	for _, p := range in.Players {
		a := ensurePlayer(p.SteamID64, p.PlayerName, p.TeamKey)
		if a != nil && p.SpecSlot != nil {
			a.meta.SpecSlot = p.SpecSlot
		}
	}

	// Index kills, damages, and flashes by round number.
	killsByRound := make(map[int][]KillEvent)
	for _, k := range in.Kills {
		killsByRound[k.RoundNumber] = append(killsByRound[k.RoundNumber], k)
	}
	dmgsByRound := make(map[int][]DamageEvent)
	for _, d := range in.Damages {
		dmgsByRound[d.RoundNumber] = append(dmgsByRound[d.RoundNumber], d)
	}
	flashesByRound := make(map[int][]FlashEvent)
	for _, f := range in.Flashes {
		flashesByRound[f.RoundNumber] = append(flashesByRound[f.RoundNumber], f)
	}
	ttdByRound := make(map[int][]TTDSample)
	for _, t := range in.TTDSamples {
		ttdByRound[t.RoundNumber] = append(ttdByRound[t.RoundNumber], t)
	}

	var clutches []RoundClutchResult

	for _, r := range in.Rounds {
		// Increment roundsPlayed per player and side.
		for _, p := range accMap {
			addForRound(p, r, func(s *statAcc) {
				s.roundsPlayed++
			})
		}

		// 0. Time-to-damage samples per attacker.
		for _, t := range ttdByRound[r.RoundNumber] {
			if p := accMap[t.AttackerSteamID]; p != nil {
				addForRound(p, r, func(s *statAcc) {
					s.ttdMS += t.DurationMS
					s.ttdSamples++
				})
			}
		}

		// 1. Capped damage (100 HP per victim per round).
		victimHP := make(map[string]int)
		for _, d := range dmgsByRound[r.RoundNumber] {
			if d.AttackerSteamID == "" || d.VictimSteamID == "" {
				continue
			}
			if d.AttackerSteamID == d.VictimSteamID {
				continue
			}
			if d.AttackerTeamKey != "" && d.VictimTeamKey != "" && d.AttackerTeamKey == d.VictimTeamKey {
				continue
			}
			rem, ok := victimHP[d.VictimSteamID]
			if !ok {
				rem = 100
			}
			if rem <= 0 || d.RawHealthDamage <= 0 {
				continue
			}
			dealt := d.RawHealthDamage
			if dealt > rem {
				dealt = rem
			}
			victimHP[d.VictimSteamID] = rem - dealt
			if att := ensurePlayer(d.AttackerSteamID, "", d.AttackerTeamKey); att != nil {
				w := roundEcoWeight(d.AttackerTeamKey, r)
				isUtil := utilityWeapons[strings.ToLower(d.Weapon)]
				addForRound(att, r, func(s *statAcc) {
					s.totalDamage += dealt
					s.ecoAdjDamage += float64(dealt) * w
					if isUtil {
						s.utilityDamage += dealt
					}
				})
			}
		}

		// 1b. Enemy flashbang utility tracking.
		for _, f := range flashesByRound[r.RoundNumber] {
			if f.AttackerSteamID == "" || f.VictimSteamID == "" || f.AttackerSteamID == f.VictimSteamID {
				continue
			}
			if f.AttackerTeamKey != "" && f.VictimTeamKey != "" && f.AttackerTeamKey == f.VictimTeamKey {
				continue
			}
			if f.DurationSec <= 0 {
				continue
			}
			if att := ensurePlayer(f.AttackerSteamID, "", f.AttackerTeamKey); att != nil {
				dur := f.DurationSec
				addForRound(att, r, func(s *statAcc) {
					s.enemiesFlashed++
					s.totalFlashSec += dur
				})
			}
		}

		// 1c. Duel timing: time from a player's first damage on a victim to the
		// killing blow, measured within one life (engagement resets on death).
		type engagement struct {
			attacker string
			tick     int64
		}
		firstHit := make(map[string]engagement)
		dmgs := dmgsByRound[r.RoundNumber]
		killList := killsByRound[r.RoundNumber]
		di, ki := 0, 0
		for di < len(dmgs) || ki < len(killList) {
			if ki < len(killList) && (di >= len(dmgs) || killList[ki].Tick <= dmgs[di].Tick) {
				k := killList[ki]
				ki++
				if e, ok := firstHit[k.VictimSteamID]; ok {
					delete(firstHit, k.VictimSteamID)
					if e.attacker != "" && e.attacker == k.AttackerSteamID && k.Tick >= e.tick && k.Tick-e.tick <= duelWindowTicks {
						if att := ensurePlayer(e.attacker, k.AttackerName, k.AttackerTeamKey); att != nil {
							delta := k.Tick - e.tick
							addForRound(att, r, func(s *statAcc) {
								s.ttkTicks += delta
								s.ttkSamples++
							})
						}
					}
				}
				continue
			}
			d := dmgs[di]
			di++
			if d.AttackerSteamID == "" || d.VictimSteamID == "" || d.AttackerSteamID == d.VictimSteamID {
				continue
			}
			if d.AttackerTeamKey != "" && d.VictimTeamKey != "" && d.AttackerTeamKey == d.VictimTeamKey {
				continue
			}
			if d.RawHealthDamage <= 0 {
				continue
			}
			prev, ok := firstHit[d.VictimSteamID]
			if !ok || prev.attacker != d.AttackerSteamID || d.Tick-prev.tick > duelWindowTicks {
				firstHit[d.VictimSteamID] = engagement{attacker: d.AttackerSteamID, tick: d.Tick}
			}
		}

		// 2. Round kills, trades, KAST flags, first duel, clutches, Round Swing.
		rKills := killsByRound[r.RoundNumber]
		roundKillCount := make(map[string]int)
		roundHadAssist := make(map[string]bool)
		roundDied := make(map[string]bool)
		roundTraded := make(map[string]bool)

		// Track alive counts per team for 1vN clutches and Round Swing.
		aliveByTeam := map[string]map[string]bool{
			"team_a": {},
			"team_b": {},
		}
		for sid, tk := range teamOf {
			if tk == "team_a" || tk == "team_b" {
				aliveByTeam[tk][sid] = true
			}
		}
		aliveCountsForSide := func() (int, int) {
			aAlive := len(aliveByTeam["team_a"])
			bAlive := len(aliveByTeam["team_b"])
			if strings.EqualFold(r.TeamASide, "CT") {
				return aAlive, bAlive
			}
			return bAlive, aAlive
		}
		plantTick := int64(0)
		if in.BombPlantTickByRound != nil {
			plantTick = in.BombPlantTickByRound[r.RoundNumber]
		}
		roundClutchSolo := make(map[string]*RoundClutchResult) // teamKey -> solo clutch

		firstDuelRecorded := false
		for i, k := range rKills {
			bombPlanted := plantTick > 0 && k.Tick >= plantTick
			ctBefore, tBefore := aliveCountsForSide()

			if k.VictimSteamID != "" {
				vAcc := ensurePlayer(k.VictimSteamID, k.VictimName, k.VictimTeamKey)
				if vAcc != nil {
					vicW := roundEcoWeight(k.VictimTeamKey, r)
					ecoDeathPenalty := 1.0 / vicW
					addForRound(vAcc, r, func(s *statAcc) {
						s.deaths++
						s.ecoAdjDeaths += ecoDeathPenalty
					})
					roundDied[k.VictimSteamID] = true
				}
				if k.VictimTeamKey != "" {
					delete(aliveByTeam[k.VictimTeamKey], k.VictimSteamID)
				}
			}

			ctAfter, tAfter := aliveCountsForSide()

			if k.AssisterSteamID != "" && k.AssisterSteamID != k.VictimSteamID {
				aAcc := ensurePlayer(k.AssisterSteamID, k.AssisterName, "")
				if aAcc != nil {
					attW := roundEcoWeight(k.AttackerTeamKey, r)
					isFlashAssist := k.IsAssistedFlash
					addForRound(aAcc, r, func(s *statAcc) {
						s.assists++
						s.ecoAdjAssists += attW
						if isFlashAssist {
							s.flashAssists++
						}
					})
					roundHadAssist[k.AssisterSteamID] = true
				}
			}
			if k.AttackerSteamID != "" && !k.TeamKill && k.AttackerSteamID != k.VictimSteamID {
				attAcc := ensurePlayer(k.AttackerSteamID, k.AttackerName, k.AttackerTeamKey)
				if attAcc != nil {
					attSide := sideForTeamInRound(k.AttackerTeamKey, r)
					var swingDelta float64
					if attSide == "CT" || attSide == "T" {
						pBefore := teamWinProb(ctBefore, tBefore, bombPlanted, attSide)
						pAfter := teamWinProb(ctAfter, tAfter, bombPlanted, attSide)
						swingDelta = (pAfter - pBefore) * 100.0
					}
					attW := roundEcoWeight(k.AttackerTeamKey, r)
					isHS := k.IsHeadshot
					isAWP := strings.EqualFold(k.Weapon, "awp")
					isFirst := !firstDuelRecorded
					addForRound(attAcc, r, func(s *statAcc) {
						s.kills++
						s.ecoAdjKills += attW
						s.roundSwingSum += swingDelta
						if isHS {
							s.headshots++
						}
						if isAWP {
							s.awpKills++
						}
						if isFirst {
							s.firstKills++
						}
					})
					roundKillCount[k.AttackerSteamID]++
					if !firstDuelRecorded {
						firstDuelRecorded = true
						if vAcc := accMap[k.VictimSteamID]; vAcc != nil {
							addForRound(vAcc, r, func(s *statAcc) {
								s.firstDeaths++
							})
						}
					}
				}
			}

			// 5-second trade check: if victim killed someone earlier, or someone kills attacker within 5s.
			if !k.TeamKill && k.AttackerSteamID != "" && k.VictimSteamID != "" {
				for j := i + 1; j < len(rKills); j++ {
					nk := rKills[j]
					if nk.Tick-k.Tick > tradeWindowTicks {
						break
					}
					if !nk.TeamKill && nk.VictimSteamID == k.AttackerSteamID && nk.AttackerTeamKey == k.VictimTeamKey && nk.AttackerSteamID != "" {
						if trader := accMap[nk.AttackerSteamID]; trader != nil {
							addForRound(trader, r, func(s *statAcc) {
								s.tradeKills++
							})
						}
						if traded := accMap[k.VictimSteamID]; traded != nil && !roundTraded[k.VictimSteamID] {
							addForRound(traded, r, func(s *statAcc) {
								s.tradeDeaths++
							})
							roundTraded[k.VictimSteamID] = true
						}
						break
					}
				}
			}

			// Check 1vN clutch condition after each death.
			for _, tk := range []string{"team_a", "team_b"} {
				other := "team_b"
				if tk == "team_b" {
					other = "team_a"
				}
				if len(aliveByTeam[tk]) == 1 && len(aliveByTeam[other]) >= 1 {
					var soloID string
					for sid := range aliveByTeam[tk] {
						soloID = sid
					}
					enemies := len(aliveByTeam[other])
					if existing, ok := roundClutchSolo[tk]; ok {
						if enemies > existing.MaxEnemies {
							existing.MaxEnemies = enemies
						}
					} else {
						roundClutchSolo[tk] = &RoundClutchResult{
							RoundNumber: r.RoundNumber,
							SteamID64:   soloID,
							TeamKey:     tk,
							MaxEnemies:  enemies,
						}
					}
				}
			}
			if k.AttackerSteamID != "" && k.AttackerTeamKey != "" {
				if c, ok := roundClutchSolo[k.AttackerTeamKey]; ok && c.SteamID64 == k.AttackerSteamID {
					c.LastKillTick = k.Tick
				}
			}
		}

		// Finalize clutch attempts/wins for the round.
		for tk, c := range roundClutchSolo {
			won := r.WinnerTeamKey != nil && *r.WinnerTeamKey == tk
			c.Won = won
			if p := accMap[c.SteamID64]; p != nil {
				enemies := c.MaxEnemies
				addForRound(p, r, func(s *statAcc) {
					s.recordClutch(enemies, won)
				})
			}
			clutches = append(clutches, *c)
		}

		// Finalize per-round survival, multi-kills, and KAST.
		for sid, p := range accMap {
			kc := roundKillCount[sid]
			survived := !roundDied[sid]
			hadKast := kc >= 1 || roundHadAssist[sid] || survived || roundTraded[sid]
			addForRound(p, r, func(s *statAcc) {
				switch kc {
				case 1:
					s.oneKillRounds++
				case 2:
					s.twoKillRounds++
					s.multiKillRounds++
				case 3:
					s.threeKillRounds++
					s.multiKillRounds++
				case 4:
					s.fourKillRounds++
					s.multiKillRounds++
				default:
					if kc >= 5 {
						s.fiveKillRounds++
						s.multiKillRounds++
					}
				}
				if survived {
					s.survivedRounds++
				}
				if hadKast {
					s.kastRounds++
				}
			})
		}
	}

	roundsDenom := float64(in.TotalRounds)
	if roundsDenom <= 0 {
		roundsDenom = float64(len(in.Rounds))
	}

	playersOut := make([]storage.MatchPlayer, 0, len(order))
	sidesOut := make([]storage.MatchPlayerSide, 0, len(order)*2)
	for _, sid := range order {
		p := accMap[sid]
		displayName := p.meta.PlayerName
		if displayName == "" {
			displayName = sid
		}

		dm := computeDerivedMetrics(&p.all, roundsDenom)

		avgEquip := 0
		if vals := in.EquipValues[sid]; len(vals) > 0 {
			sum := 0
			for _, v := range vals {
				sum += v
			}
			avgEquip = int(math.Round(float64(sum) / float64(len(vals))))
		}

		clutchMap := make(map[string]clutchTierCount)
		for tier, tc := range p.all.clutchByTier {
			if tc != nil && tc.Attempts > 0 {
				clutchMap[fmt.Sprintf("1v%d", tier)] = *tc
			}
		}
		clutchJSONBytes, _ := json.Marshal(clutchMap)
		if len(clutchJSONBytes) == 0 {
			clutchJSONBytes = []byte("{}")
		}

		playersOut = append(playersOut, storage.MatchPlayer{
			SteamID64:           sid,
			PlayerName:          displayName,
			DisplayName:         displayName,
			TeamKey:             p.meta.TeamKey,
			SpecSlot:            p.meta.SpecSlot,
			Kills:               p.all.kills,
			Deaths:              p.all.deaths,
			Assists:             p.all.assists,
			KDRatio:             dm.kd,
			KPR:                 roundTo(dm.kpr, 3),
			DPR:                 roundTo(dm.dpr, 3),
			APR:                 roundTo(dm.apr, 3),
			TotalDamage:         p.all.totalDamage,
			ADR:                 dm.adr,
			UtilityDamage:       p.all.utilityDamage,
			UtilityADR:          dm.uadr,
			KASTPercent:         dm.kast,
			Headshots:           p.all.headshots,
			HSPercent:           dm.hsPct,
			FirstKills:          p.all.firstKills,
			FirstDeaths:         p.all.firstDeaths,
			OpeningDuelWinRate:  dm.openWR,
			TradeKills:          p.all.tradeKills,
			TradeDeaths:         p.all.tradeDeaths,
			SurvivalRate:        dm.survRate,
			OneKillRounds:       p.all.oneKillRounds,
			TwoKillRounds:       p.all.twoKillRounds,
			ThreeKillRounds:     p.all.threeKillRounds,
			FourKillRounds:      p.all.fourKillRounds,
			FiveKillRounds:      p.all.fiveKillRounds,
			MultiKillRounds:     p.all.multiKillRounds,
			ClutchAttempts:      p.all.clutchAttempts,
			ClutchWins:          p.all.clutchWins,
			AWPKills:            p.all.awpKills,
			AvgEquipmentValue:   avgEquip,
			ImpactScore:         roundTo(dm.impact, 3),
			StrikeyeRating:      dm.rating,
			FlashAssists:        p.all.flashAssists,
			EnemiesFlashed:      p.all.enemiesFlashed,
			TotalFlashSec:       roundTo(p.all.totalFlashSec, 1),
			RoundSwing:          dm.roundSwing,
			EcoAdjRating:        dm.ecoAdjRating,
			EcoAdjADR:           dm.ecoAdjADR,
			ClutchBreakdownJSON: string(clutchJSONBytes),
			TTKAvgMS:            avgTTKMS(p.all, tickRate),
			TTKSamples:          p.all.ttkSamples,
			TTDAvgMS:            avgTTDMS(p.all),
			TTDSamples:          p.all.ttdSamples,
		})

		for _, sideName := range []string{"CT", "T"} {
			sa := p.sides[sideName]
			if sa == nil || sa.roundsPlayed == 0 {
				continue
			}
			sdm := computeDerivedMetrics(sa, float64(sa.roundsPlayed))
			sidesOut = append(sidesOut, storage.MatchPlayerSide{
				SteamID64:          sid,
				Side:               sideName,
				RoundsPlayed:       sa.roundsPlayed,
				Kills:              sa.kills,
				Deaths:             sa.deaths,
				Assists:            sa.assists,
				KDRatio:            sdm.kd,
				KPR:                roundTo(sdm.kpr, 3),
				DPR:                roundTo(sdm.dpr, 3),
				APR:                roundTo(sdm.apr, 3),
				TotalDamage:        sa.totalDamage,
				ADR:                sdm.adr,
				UtilityDamage:      sa.utilityDamage,
				UtilityADR:         sdm.uadr,
				KASTPercent:        sdm.kast,
				Headshots:          sa.headshots,
				HSPercent:          sdm.hsPct,
				FirstKills:         sa.firstKills,
				FirstDeaths:        sa.firstDeaths,
				OpeningDuelWinRate: sdm.openWR,
				TradeKills:         sa.tradeKills,
				TradeDeaths:        sa.tradeDeaths,
				SurvivalRate:       sdm.survRate,
				MultiKillRounds:    sa.multiKillRounds,
				ClutchAttempts:     sa.clutchAttempts,
				ClutchWins:         sa.clutchWins,
				FlashAssists:       sa.flashAssists,
				EnemiesFlashed:     sa.enemiesFlashed,
				TotalFlashSec:      roundTo(sa.totalFlashSec, 1),
				ImpactScore:        roundTo(sdm.impact, 3),
				RoundSwing:         sdm.roundSwing,
				StrikeyeRating:     sdm.rating,
				EcoAdjRating:       sdm.ecoAdjRating,
				EcoAdjADR:          sdm.ecoAdjADR,
			})
		}
	}

	return MetricsResult{
		Players:  playersOut,
		Sides:    sidesOut,
		Clutches: clutches,
	}
}

func avgTTDMS(a statAcc) float64 {
	if a.ttdSamples == 0 {
		return 0
	}
	return roundTo(a.ttdMS/float64(a.ttdSamples), 1)
}

func avgTTKMS(a statAcc, tickRate float64) float64 {
	if a.ttkSamples == 0 || tickRate <= 0 {
		return 0
	}
	return roundTo(float64(a.ttkTicks)/tickRate*1000.0/float64(a.ttkSamples), 1)
}

func roundTo(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}
