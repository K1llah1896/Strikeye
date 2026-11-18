package analysis

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

// EconomyKPIs summarizes team-level tactical and economy conversion rates.
type EconomyKPIs struct {
	PistolAWins, PistolTotal           int
	ConversionAWins, ConversionTotal   int
	FullVsFullAWins, FullVsFullTotal   int
	ForceVsFullWins, ForceVsFullTotal  int
	EcoVsFullWins, EcoVsFullTotal      int
	PostPlantWins, PostPlantTotal      int
	SiteAWins, SiteATotal              int
	SiteBWins, SiteBTotal              int
}

func isTeamAKey(key, teamAName string) bool {
	u := strings.ToUpper(strings.TrimSpace(key))
	if u == "TEAM_A" || u == "A" {
		return true
	}
	if teamAName != "" && u == strings.ToUpper(strings.TrimSpace(teamAName)) {
		return true
	}
	return false
}

func normalizeEcoTier(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pistol":
		return "pistol"
	case "full", "full_buy":
		return "full"
	case "force", "force_buy", "semi", "half_buy":
		return "force"
	case "eco":
		return "eco"
	default:
		return "full"
	}
}

// ComputeEconomyKPIs aggregates pistol, conversion, buy-tier matchup, and post-plant win rates.
func ComputeEconomyKPIs(rounds []storage.MatchRound, teamAName string) EconomyKPIs {
	var k EconomyKPIs
	for i, r := range rounds {
		if r.WinnerTeamKey == nil {
			continue
		}
		aWon := isTeamAKey(*r.WinnerTeamKey, teamAName)
		ecoA := normalizeEcoTier(r.TeamAEconomyType)
		ecoB := normalizeEcoTier(r.TeamBEconomyType)

		if ecoA == "pistol" || ecoB == "pistol" {
			k.PistolTotal++
			if aWon {
				k.PistolAWins++
			}
			// Check post-pistol conversion on next round if consecutive.
			if i+1 < len(rounds) && rounds[i+1].WinnerTeamKey != nil && rounds[i+1].RoundNumber == r.RoundNumber+1 {
				k.ConversionTotal++
				nextAWon := isTeamAKey(*rounds[i+1].WinnerTeamKey, teamAName)
				if nextAWon == aWon {
					k.ConversionAWins++
				}
			}
		}

		if ecoA == "full" && ecoB == "full" {
			k.FullVsFullTotal++
			if aWon {
				k.FullVsFullAWins++
			}
		} else if ecoA == "force" && ecoB == "full" {
			k.ForceVsFullTotal++
			if aWon {
				k.ForceVsFullWins++
			}
		} else if ecoB == "force" && ecoA == "full" {
			k.ForceVsFullTotal++
			if !aWon {
				k.ForceVsFullWins++
			}
		} else if ecoA == "eco" && ecoB == "full" {
			k.EcoVsFullTotal++
			if aWon {
				k.EcoVsFullWins++
			}
		} else if ecoB == "eco" && ecoA == "full" {
			k.EcoVsFullTotal++
			if !aWon {
				k.EcoVsFullWins++
			}
		}

		if r.BombSite != nil && strings.TrimSpace(*r.BombSite) != "" {
			k.PostPlantTotal++
			tWon := r.WinnerSide != nil && strings.EqualFold(*r.WinnerSide, "T")
			if tWon {
				k.PostPlantWins++
			}
			site := strings.ToUpper(strings.TrimSpace(*r.BombSite))
			if site == "A" {
				k.SiteATotal++
				if tWon {
					k.SiteAWins++
				}
			} else if site == "B" {
				k.SiteBTotal++
				if tWon {
					k.SiteBWins++
				}
			}
		}
	}
	return k
}

// ClutchTierSummary holds parsed 1v1..1v5 clutch counts for UI rendering.
type ClutchTierSummary struct {
	Tier     string
	Attempts int
	Wins     int
}

// ParseClutchBreakdown decodes ClutchBreakdownJSON into ordered 1v1..1v5 entries.
func ParseClutchBreakdown(raw string) []ClutchTierSummary {
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return nil
	}
	var m map[string]struct {
		Attempts int `json:"attempts"`
		Wins     int `json:"wins"`
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	order := []string{"1v1", "1v2", "1v3", "1v4", "1v5"}
	out := make([]ClutchTierSummary, 0, len(order))
	for _, k := range order {
		if v, ok := m[k]; ok && v.Attempts > 0 {
			out = append(out, ClutchTierSummary{Tier: k, Attempts: v.Attempts, Wins: v.Wins})
		}
	}
	return out
}

// ParsePlayerEconomy decodes MatchRound.PlayerEconomyJSON.
func ParsePlayerEconomy(raw string) []parser.PlayerRoundEconomy {
	if strings.TrimSpace(raw) == "" || raw == "[]" {
		return nil
	}
	var out []parser.PlayerRoundEconomy
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// H2HCell stores head-to-head kills and deaths between a Team A and Team B player.
type H2HCell struct {
	Kills  int
	Deaths int
}

// WeaponStat summarizes match-wide kills and headshots for one weapon.
type WeaponStat struct {
	Weapon    string
	Kills     int
	Headshots int
	SharePct  float64
}

// PerformanceMatrix holds 5v5 H2H, Opening Duel, and Weapon distribution data.
type PerformanceMatrix struct {
	TeamA      []storage.MatchPlayer
	TeamB      []storage.MatchPlayer
	H2H        map[string]map[string]H2HCell // teamA_sid -> teamB_sid -> cell
	OpeningH2H map[string]map[string]H2HCell // teamA_sid -> teamB_sid -> opening duel cell
	Weapons    []WeaponStat
	TotalKills int
}

// SplitPlayersByTeam partitions players into Team A and Team B groups.
func SplitPlayersByTeam(players []storage.MatchPlayer, teamAName string) ([]storage.MatchPlayer, []storage.MatchPlayer) {
	var groupA, groupB []storage.MatchPlayer
	for _, p := range players {
		tk := strings.ToUpper(strings.TrimSpace(p.TeamKey))
		if tk == "CT" || tk == "A" || tk == "TEAM_A" || (teamAName != "" && tk == strings.ToUpper(strings.TrimSpace(teamAName))) {
			groupA = append(groupA, p)
		} else {
			groupB = append(groupB, p)
		}
	}
	return groupA, groupB
}

// BuildPerformanceMatrix computes 5v5 H2H kill matrix, opening duel matrix, and weapon stats from match_events.
func BuildPerformanceMatrix(players []storage.MatchPlayer, events []storage.MatchEvent, teamAName string) PerformanceMatrix {
	teamA, teamB := SplitPlayersByTeam(players, teamAName)
	inA := make(map[string]bool, len(teamA))
	for _, p := range teamA {
		inA[p.SteamID64] = true
	}
	inB := make(map[string]bool, len(teamB))
	for _, p := range teamB {
		inB[p.SteamID64] = true
	}

	h2h := make(map[string]map[string]H2HCell)
	opening := make(map[string]map[string]H2HCell)
	for _, a := range teamA {
		h2h[a.SteamID64] = make(map[string]H2HCell)
		opening[a.SteamID64] = make(map[string]H2HCell)
	}

	type wepAcc struct {
		kills int
		hs    int
	}
	wepMap := make(map[string]*wepAcc)
	firstKillSeen := make(map[int]bool)
	totalKills := 0

	for _, ev := range events {
		if ev.EventType != "kill" || ev.ActorSteamID64 == nil || ev.TargetSteamID64 == nil {
			continue
		}
		att := *ev.ActorSteamID64
		vic := *ev.TargetSteamID64
		if att == "" || vic == "" || att == vic {
			continue
		}
		totalKills++

		wep := "unknown"
		if ev.Weapon != nil && strings.TrimSpace(*ev.Weapon) != "" {
			wep = strings.ToLower(strings.TrimSpace(*ev.Weapon))
		}
		wa := wepMap[wep]
		if wa == nil {
			wa = &wepAcc{}
			wepMap[wep] = wa
		}
		wa.kills++
		if ev.IsHeadshot {
			wa.hs++
		}

		isOpening := !firstKillSeen[ev.RoundNumber]
		firstKillSeen[ev.RoundNumber] = true

		if inA[att] && inB[vic] {
			c := h2h[att][vic]
			c.Kills++
			h2h[att][vic] = c
			if isOpening {
				oc := opening[att][vic]
				oc.Kills++
				opening[att][vic] = oc
			}
		} else if inB[att] && inA[vic] {
			c := h2h[vic][att]
			c.Deaths++
			h2h[vic][att] = c
			if isOpening {
				oc := opening[vic][att]
				oc.Deaths++
				opening[vic][att] = oc
			}
		}
	}

	weapons := make([]WeaponStat, 0, len(wepMap))
	for name, wa := range wepMap {
		share := 0.0
		if totalKills > 0 {
			share = float64(wa.kills) / float64(totalKills) * 100.0
		}
		weapons = append(weapons, WeaponStat{
			Weapon:    name,
			Kills:     wa.kills,
			Headshots: wa.hs,
			SharePct:  share,
		})
	}
	sort.Slice(weapons, func(i, j int) bool {
		if weapons[i].Kills != weapons[j].Kills {
			return weapons[i].Kills > weapons[j].Kills
		}
		return weapons[i].Weapon < weapons[j].Weapon
	})

	return PerformanceMatrix{
		TeamA:      teamA,
		TeamB:      teamB,
		H2H:        h2h,
		OpeningH2H: opening,
		Weapons:    weapons,
		TotalKills: totalKills,
	}
}

// MergeSideStats returns a copy of players with CT or T side metrics overlaid when sideFilter is "CT" or "T".
func MergeSideStats(players []storage.MatchPlayer, sides []storage.MatchPlayerSide, sideFilter string) ([]storage.MatchPlayer, bool) {
	if sideFilter != "CT" && sideFilter != "T" {
		return players, true
	}
	bySID := make(map[string]storage.MatchPlayerSide)
	for _, s := range sides {
		if strings.EqualFold(s.Side, sideFilter) {
			bySID[s.SteamID64] = s
		}
	}
	if len(bySID) == 0 {
		return players, false
	}
	out := make([]storage.MatchPlayer, 0, len(players))
	for _, p := range players {
		cp := p
		if s, ok := bySID[p.SteamID64]; ok {
			cp.Kills = s.Kills
			cp.Deaths = s.Deaths
			cp.Assists = s.Assists
			cp.KDRatio = s.KDRatio
			cp.KPR = s.KPR
			cp.DPR = s.DPR
			cp.APR = s.APR
			cp.TotalDamage = s.TotalDamage
			cp.ADR = s.ADR
			cp.UtilityDamage = s.UtilityDamage
			cp.UtilityADR = s.UtilityADR
			cp.KASTPercent = s.KASTPercent
			cp.Headshots = s.Headshots
			cp.HSPercent = s.HSPercent
			cp.FirstKills = s.FirstKills
			cp.FirstDeaths = s.FirstDeaths
			cp.OpeningDuelWinRate = s.OpeningDuelWinRate
			cp.TradeKills = s.TradeKills
			cp.TradeDeaths = s.TradeDeaths
			cp.SurvivalRate = s.SurvivalRate
			cp.MultiKillRounds = s.MultiKillRounds
			cp.ClutchAttempts = s.ClutchAttempts
			cp.ClutchWins = s.ClutchWins
			cp.FlashAssists = s.FlashAssists
			cp.EnemiesFlashed = s.EnemiesFlashed
			cp.TotalFlashSec = s.TotalFlashSec
			cp.ImpactScore = s.ImpactScore
			cp.RoundSwing = s.RoundSwing
			cp.StrikeyeRating = s.StrikeyeRating
			cp.EcoAdjRating = s.EcoAdjRating
			cp.EcoAdjADR = s.EcoAdjADR
		}
		out = append(out, cp)
	}
	return out, true
}

func fmtSwing(v float64) string {
	if v > 0 {
		return fmt.Sprintf("+%.2f%%", v)
	}
	return fmt.Sprintf("%.2f%%", v)
}
