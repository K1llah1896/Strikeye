// Package tendency builds an opponent-scouting profile from stored matches: how
// a team splits its rounds by side, economy, bomb site and opening duel, so a
// coach can prepare an anti-strat without scrubbing demos by hand.
package tendency

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"strikeye/internal/callouts"
	"strikeye/internal/mapgeom"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

// utilityWeapons is the set of grenades counted by the utility-per-round metric.
var utilityWeapons = map[string]bool{
	"Smoke Grenade":          true,
	"Flashbang":              true,
	"Molotov":                true,
	"Incendiary Grenade":     true,
	"High Explosive Grenade": true,
	"Decoy Grenade":          true,
}

// SideProfile summarises one (map, side) pairing.
type SideProfile struct {
	Map             string  `json:"map"`
	Side            string  `json:"side"`
	Rounds          int     `json:"rounds"`
	Wins            int     `json:"wins"`
	WinPct          float64 `json:"win_pct"`
	Pistols         int     `json:"pistols"`
	PistolWins      int     `json:"pistol_wins"`
	AvgEquip        int     `json:"avg_equip"`
	UtilityPerRound float64 `json:"utility_per_round"`

	utilitySum int
}

// EconomyStat is the team's record per buy type on one side.
type EconomyStat struct {
	Side   string  `json:"side"`
	Type   string  `json:"type"`
	Rounds int     `json:"rounds"`
	Wins   int     `json:"wins"`
	WinPct float64 `json:"win_pct"`
}

// SiteStat counts bomb plants and conversions per site.
type SiteStat struct {
	Map    string  `json:"map"`
	Site   string  `json:"site"`
	Plants int     `json:"plants"`
	Wins   int     `json:"wins"`
	WinPct float64 `json:"win_pct"`
}

// OpeningStat counts which callout the team's opening kills happen at.
type OpeningStat struct {
	Callout string  `json:"callout"`
	Kills   int     `json:"kills"`
	Share   float64 `json:"share"`
}

// Report is the scouting profile for one team.
type Report struct {
	Team          string        `json:"team"`
	Matches       int           `json:"matches"`
	Maps          []string      `json:"maps"`
	Rounds        int           `json:"rounds"`
	OpeningKills  int           `json:"opening_kills"`
	OpeningSecAvg float64       `json:"opening_sec_avg"`
	Sides         []SideProfile `json:"sides"`
	Economy       []EconomyStat `json:"economy"`
	Sites         []SiteStat    `json:"sites"`
	Opening       []OpeningStat `json:"opening"`
	CalloutsUsed  bool          `json:"callouts_used"`
}

// CalloutsMissing reports whether opening-duel areas could not be resolved
// because no callout table was supplied.
func (r Report) CalloutsMissing() bool { return !r.CalloutsUsed }

// Input bundles everything Build needs beyond the database handle.
type Input struct {
	Team     string
	Replays  map[int64][]parser.ReplayRoundData // demo id -> rounds
	Callouts *callouts.Table
	TopN     int
}

// Build assembles the scouting profile for team.
func Build(ctx context.Context, db *storage.DB, in Input) (Report, error) {
	team := strings.TrimSpace(in.Team)
	if team == "" {
		return Report{}, fmt.Errorf("team name is required")
	}
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("list demos: %w", err)
	}
	topN := in.TopN
	if topN <= 0 {
		topN = 8
	}

	rep := Report{Team: team, CalloutsUsed: in.Callouts != nil}
	sideKey := map[string]*SideProfile{}
	econKey := map[string]*EconomyStat{}
	siteKey := map[string]*SiteStat{}
	openingCount := map[string]int{}
	var openingSec float64
	mapSeen := map[string]bool{}

	for _, demo := range demos {
		if demo.ParseStatus != "parsed" {
			continue
		}
		teamKey, sideOf := teamSides(demo, team)
		if teamKey == "" {
			continue
		}
		rounds, err := db.Analysis().ListMatchRounds(ctx, demo.ID)
		if err != nil {
			return Report{}, fmt.Errorf("rounds for demo %d: %w", demo.ID, err)
		}
		players, err := db.Analysis().ListMatchPlayers(ctx, demo.ID)
		if err != nil {
			return Report{}, fmt.Errorf("players for demo %d: %w", demo.ID, err)
		}
		purchases, err := db.Analysis().ListMatchPurchases(ctx, demo.ID)
		if err != nil {
			return Report{}, fmt.Errorf("purchases for demo %d: %w", demo.ID, err)
		}
		steamTeam := map[string]string{}
		for _, p := range players {
			steamTeam[p.SteamID64] = p.TeamKey
		}
		mapName := ""
		if demo.MapName != nil {
			mapName = strings.TrimSpace(*demo.MapName)
		}
		if mapName != "" && !mapSeen[mapName] {
			mapSeen[mapName] = true
			rep.Maps = append(rep.Maps, mapName)
		}
		rep.Matches++

		utilByRound := map[int]int{}
		for _, p := range purchases {
			if !utilityWeapons[p.Weapon] || steamTeam[p.SteamID64] != teamKey {
				continue
			}
			utilByRound[p.RoundNumber]++
		}

		tickRate := demo.TickRate
		if tickRate <= 0 {
			tickRate = 64
		}
		deathsByRound := map[int][]parser.ReplayDeathEvent{}
		for _, rd := range in.Replays[demo.ID] {
			deathsByRound[int(rd.RoundNumber)] = rd.Deaths
		}

		for _, r := range rounds {
			side := sideOf(r)
			if side == "" {
				continue
			}
			rep.Rounds++
			won := r.WinnerTeamKey != nil && *r.WinnerTeamKey == teamKey
			equip := r.TeamAEquipValue
			econType := r.TeamAEconomyType
			if sideOf(r) == r.TeamBSide {
				equip = r.TeamBEquipValue
				econType = r.TeamBEconomyType
			}
			sp := sideKey[mapName+"|"+side]
			if sp == nil {
				sp = &SideProfile{Map: mapName, Side: side}
				sideKey[mapName+"|"+side] = sp
			}
			sp.Rounds++
			sp.AvgEquip += equip
			if won {
				sp.Wins++
			}
			if r.RoundNumber == 1 || r.RoundNumber == 13 {
				sp.Pistols++
				if won {
					sp.PistolWins++
				}
			}
			sp.utilitySum += utilByRound[r.RoundNumber]

			ek := side + "|" + econType
			if econType == "" {
				ek = side + "|unknown"
			}
			es := econKey[ek]
			if es == nil {
				es = &EconomyStat{Side: side, Type: econType}
				econKey[ek] = es
			}
			es.Rounds++
			if won {
				es.Wins++
			}

			// Only the team's own T-side plants describe their site preference.
			if side == "T" && r.BombSite != nil && *r.BombSite != "" {
				site := strings.ToUpper(strings.TrimSpace(*r.BombSite))
				sk := mapName + "|" + site
				ss := siteKey[sk]
				if ss == nil {
					ss = &SiteStat{Map: mapName, Site: site}
					siteKey[sk] = ss
				}
				ss.Plants++
				if won {
					ss.Wins++
				}
			}

			// Opening duel: earliest death of the round decides who got first blood.
			deaths := deathsByRound[r.RoundNumber]
			if len(deaths) == 0 {
				continue
			}
			sort.Slice(deaths, func(i, j int) bool { return deaths[i].Tick < deaths[j].Tick })
			first := deaths[0]
			killerSlot := int(first.KillerSlot)
			for _, p := range players {
				if p.SpecSlot == nil || *p.SpecSlot != killerSlot {
					continue
				}
				if p.TeamKey != teamKey {
					continue
				}
				rep.OpeningKills++
				if r.FreezeEndTick > 0 {
					openingSec += float64(first.Tick-r.FreezeEndTick) / tickRate
				}
				if in.Callouts != nil {
					if set, err := in.Callouts.For(mapName); err == nil && set != nil {
						pos := mapgeom.Vec3{first.Victim.X, first.Victim.Y, first.Victim.Z}
						if place, ok := set.At(pos); ok {
							openingCount[place.Label]++
						}
					}
				}
				break
			}
		}
	}

	for _, sp := range sideKey {
		sp.WinPct = pct(sp.Wins, sp.Rounds)
		if sp.Rounds > 0 {
			sp.AvgEquip /= sp.Rounds
			sp.UtilityPerRound = round2(float64(sp.utilitySum) / float64(sp.Rounds))
		}
		rep.Sides = append(rep.Sides, *sp)
	}
	sort.Slice(rep.Sides, func(i, j int) bool {
		if rep.Sides[i].Map != rep.Sides[j].Map {
			return rep.Sides[i].Map < rep.Sides[j].Map
		}
		return rep.Sides[i].Side < rep.Sides[j].Side
	})

	for _, es := range econKey {
		es.WinPct = pct(es.Wins, es.Rounds)
		rep.Economy = append(rep.Economy, *es)
	}
	sort.Slice(rep.Economy, func(i, j int) bool {
		if rep.Economy[i].Side != rep.Economy[j].Side {
			return rep.Economy[i].Side < rep.Economy[j].Side
		}
		if rep.Economy[i].Rounds != rep.Economy[j].Rounds {
			return rep.Economy[i].Rounds > rep.Economy[j].Rounds
		}
		return rep.Economy[i].Type < rep.Economy[j].Type
	})

	for _, ss := range siteKey {
		ss.WinPct = pct(ss.Wins, ss.Plants)
		rep.Sites = append(rep.Sites, *ss)
	}
	sort.Slice(rep.Sites, func(i, j int) bool {
		if rep.Sites[i].Map != rep.Sites[j].Map {
			return rep.Sites[i].Map < rep.Sites[j].Map
		}
		return rep.Sites[i].Plants > rep.Sites[j].Plants
	})

	for name, n := range openingCount {
		rep.Opening = append(rep.Opening, OpeningStat{Callout: name, Kills: n, Share: pct(n, rep.OpeningKills)})
	}
	sort.Slice(rep.Opening, func(i, j int) bool {
		if rep.Opening[i].Kills != rep.Opening[j].Kills {
			return rep.Opening[i].Kills > rep.Opening[j].Kills
		}
		return rep.Opening[i].Callout < rep.Opening[j].Callout
	})
	if len(rep.Opening) > topN {
		rep.Opening = rep.Opening[:topN]
	}
	if rep.OpeningKills > 0 {
		rep.OpeningSecAvg = round2(openingSec / float64(rep.OpeningKills))
	}
	sort.Strings(rep.Maps)
	return rep, nil
}

// teamSides resolves which TeamKey belongs to the team and a helper returning
// the side that team played on a given round.
func teamSides(demo storage.Demo, team string) (string, func(storage.MatchRound) string) {
	want := strings.ToLower(strings.TrimSpace(team))
	var key string
	switch {
	case demo.TeamAName != "" && strings.ToLower(demo.TeamAName) == want:
		key = "team_a"
	case demo.TeamBName != "" && strings.ToLower(demo.TeamBName) == want:
		key = "team_b"
	}
	if key == "" {
		return "", func(storage.MatchRound) string { return "" }
	}
	return key, func(r storage.MatchRound) string {
		if key == "team_a" {
			return r.TeamASide
		}
		return r.TeamBSide
	}
}

func pct(n, total int) float64 {
	if total <= 0 {
		return 0
	}
	return round2(float64(n) * 100 / float64(total))
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
