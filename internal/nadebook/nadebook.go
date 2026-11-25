// Package nadebook clusters grenade throws into reusable line-ups: same side,
// same grenade type, same throw spot and same landing area. Each cluster keeps
// its usage, timing and follow-up damage so a team's utility playbook can be
// read off a demo.
package nadebook

import (
	"math"
	"sort"
	"strings"

	"strikeye/internal/callouts"
	"strikeye/internal/mapgeom"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

// Config tunes clustering.
type Config struct {
	// GridUnits is the cell size used to group throw origins and landings.
	GridUnits float64
	// FollowUpSeconds is how long after landing a kill still counts as
	// "set up by" the grenade.
	FollowUpSeconds float64
	// FollowUpRadius is the radius around the landing point for follow-up kills.
	FollowUpRadius float64
}

// DefaultConfig groups throws within 128 units and counts kills in the 8
// seconds after landing within 600 units.
func DefaultConfig() Config {
	return Config{GridUnits: 128, FollowUpSeconds: 8, FollowUpRadius: 600}
}

// Lineup is one aggregated utility pattern.
type Lineup struct {
	Type          string `json:"type"`
	Side          string `json:"side"`
	StartCallout  string `json:"start_callout"`
	LandCallout   string `json:"land_callout"`
	Throws        int    `json:"throws"`
	Rounds        int    `json:"rounds"`
	FollowUpKills int    `json:"follow_up_kills"`
	// Wins counts the rounds this line-up appeared in that its team won, and
	// Impact is the win-rate delta against the team's own baseline in points.
	Wins          int          `json:"wins"`
	WinRate       float64      `json:"win_rate"`
	Impact        float64      `json:"impact"`
	KillsPerThrow float64      `json:"kills_per_throw"`
	AvgTimeSec    float64      `json:"avg_time_sec"`
	Start         mapgeom.Vec3 `json:"start"`
	Land          mapgeom.Vec3 `json:"land"`
}

// Report is the utility book for one demo.
type Report struct {
	Map         string   `json:"map"`
	TotalThrows int      `json:"total_throws"`
	NamedThrows int      `json:"named_throws"`
	Config      Config   `json:"config"`
	Lineups     []Lineup `json:"lineups"`
}

type cluster struct {
	lineup   Lineup
	team     string
	roundSet map[int]struct{}
	winSet   map[int]struct{}
	sumTick  float64
	sumStart [3]float64
	sumLand  [3]float64
}

// Build aggregates every throw in the replay index.
func Build(replays map[int]parser.ReplayRoundData, rounds []storage.MatchRound, players []storage.MatchPlayer, teamA string, tickRate float64, set *callouts.Set, cfg Config) Report {
	if cfg.GridUnits <= 0 {
		cfg = DefaultConfig()
	}
	rep := Report{Config: cfg}

	roundSides := map[int]storage.MatchRound{}
	teamRounds := map[string]int{}
	teamWins := map[string]int{}
	for _, r := range rounds {
		roundSides[r.RoundNumber] = r
		if r.WinnerTeamKey == nil {
			continue
		}
		for _, team := range []string{"team_a", "team_b"} {
			side := strings.ToUpper(teamSide(r, team, teamA))
			key := team + "|" + side
			teamRounds[key]++
			if *r.WinnerTeamKey == team {
				teamWins[key]++
			}
		}
	}
	teamBySteam := map[string]string{}
	for _, p := range players {
		teamBySteam[p.SteamID64] = p.TeamKey
	}

	clusters := map[string]*cluster{}
	roundNums := make([]int, 0, len(replays))
	for rn := range replays {
		roundNums = append(roundNums, rn)
	}
	sort.Ints(roundNums)

	for _, rn := range roundNums {
		rd := replays[rn]
		base := rd.StartTick
		for _, r := range rounds {
			if r.RoundNumber == rn && r.FreezeEndTick > 0 {
				base = r.FreezeEndTick
			}
		}
		for _, g := range rd.Grenades {
			rep.TotalThrows++
			if len(g.Points) == 0 {
				continue
			}
			start := g.Points[0].Pos
			land := g.Points[len(g.Points)-1].Pos
			startV := mapgeom.Vec3{start.X, start.Y, start.Z}
			landV := mapgeom.Vec3{land.X, land.Y, land.Z}

			side := ""
			if team, ok := teamBySteam[g.ThrowerSteamID]; ok {
				side = teamSide(roundSides[rn], team, teamA)
			}
			startLabel, endLabel := "", ""
			if set != nil {
				if place, ok := set.At(startV); ok {
					startLabel = place.Label
				}
				if place, ok := set.At(landV); ok {
					endLabel = place.Label
				}
			}
			if startLabel != "" || endLabel != "" {
				rep.NamedThrows++
			}

			key := strings.Join([]string{
				normalizeType(g.GrenadeType), side,
				cell(startV, cfg.GridUnits), cell(landV, cfg.GridUnits),
			}, "|")
			cl := clusters[key]
			if cl == nil {
				cl = &cluster{
					lineup: Lineup{
						Type:         normalizeType(g.GrenadeType),
						Side:         side,
						StartCallout: startLabel,
						LandCallout:  endLabel,
					},
					roundSet: map[int]struct{}{},
					winSet:   map[int]struct{}{},
				}
				clusters[key] = cl
			}
			cl.lineup.Throws++
			if cl.team == "" {
				cl.team = teamBySteam[g.ThrowerSteamID]
			}
			cl.roundSet[rn] = struct{}{}
			if team, ok := teamBySteam[g.ThrowerSteamID]; ok {
				if r, ok := roundSides[rn]; ok && r.WinnerTeamKey != nil && *r.WinnerTeamKey == team {
					cl.winSet[rn] = struct{}{}
				}
			}
			if tickRate > 0 && g.ThrowTick > base {
				cl.sumTick += float64(g.ThrowTick-base) / tickRate
			}
			cl.sumStart[0] += float64(startV[0])
			cl.sumStart[1] += float64(startV[1])
			cl.sumStart[2] += float64(startV[2])
			cl.sumLand[0] += float64(landV[0])
			cl.sumLand[1] += float64(landV[1])
			cl.sumLand[2] += float64(landV[2])
			if cl.lineup.StartCallout == "" {
				cl.lineup.StartCallout = startLabel
			}
			if cl.lineup.LandCallout == "" {
				cl.lineup.LandCallout = endLabel
			}
			cl.lineup.FollowUpKills += followUpKills(rd, g, landV, tickRate, cfg)
		}
	}

	for _, cl := range clusters {
		n := float64(cl.lineup.Throws)
		cl.lineup.Rounds = len(cl.roundSet)
		cl.lineup.AvgTimeSec = cl.sumTick / n
		cl.lineup.Start = mapgeom.Vec3{
			float32(cl.sumStart[0] / n), float32(cl.sumStart[1] / n), float32(cl.sumStart[2] / n),
		}
		cl.lineup.Land = mapgeom.Vec3{
			float32(cl.sumLand[0] / n), float32(cl.sumLand[1] / n), float32(cl.sumLand[2] / n),
		}
		// Label the cluster by its mean position so a line-up is never named
		// after a single off-centre throw.
		if set != nil {
			if place, ok := set.At(cl.lineup.Start); ok {
				cl.lineup.StartCallout = place.Label
			}
			if place, ok := set.At(cl.lineup.Land); ok {
				cl.lineup.LandCallout = place.Label
			}
		}
		cl.lineup.Wins = len(cl.winSet)
		if cl.lineup.Rounds > 0 {
			cl.lineup.WinRate = roundOne(float64(cl.lineup.Wins) / float64(cl.lineup.Rounds) * 100)
		}
		if cl.lineup.Throws > 0 {
			cl.lineup.KillsPerThrow = roundOne(float64(cl.lineup.FollowUpKills) / float64(cl.lineup.Throws))
		}
		// Impact compares the line-up's rounds against the throwing team's own
		// baseline, so a strong team's every line-up does not look "great".
		baseKey := cl.team + "|" + strings.ToUpper(cl.lineup.Side)
		if cl.team != "" && teamRounds[baseKey] > 0 {
			baseline := float64(teamWins[baseKey]) / float64(teamRounds[baseKey]) * 100
			cl.lineup.Impact = roundOne(cl.lineup.WinRate - baseline)
		}
		rep.Lineups = append(rep.Lineups, cl.lineup)
	}
	// Frequent line-ups first, but within a similar usage band the win-rate delta
	// decides: that is the "which setup actually pays" question.
	sort.Slice(rep.Lineups, func(i, j int) bool {
		a, b := rep.Lineups[i], rep.Lineups[j]
		if a.Throws != b.Throws {
			return a.Throws > b.Throws
		}
		if a.Impact != b.Impact {
			return a.Impact > b.Impact
		}
		return a.FollowUpKills > b.FollowUpKills
	})
	return rep
}

// followUpKills counts kills shortly after the grenade lands, near its landing.
func followUpKills(rd parser.ReplayRoundData, g parser.GrenadeTrack, land mapgeom.Vec3, tickRate float64, cfg Config) int {
	if tickRate <= 0 || cfg.FollowUpSeconds <= 0 {
		return 0
	}
	landTick := g.EndTick
	if landTick <= 0 && len(g.Points) > 0 {
		landTick = g.Points[len(g.Points)-1].Tick
	}
	window := int64(cfg.FollowUpSeconds * tickRate)
	r2 := cfg.FollowUpRadius * cfg.FollowUpRadius
	count := 0
	for _, d := range rd.Deaths {
		if d.Tick <= landTick || d.Tick > landTick+window {
			continue
		}
		dx := float64(d.Victim.X - land[0])
		dy := float64(d.Victim.Y - land[1])
		if dx*dx+dy*dy <= r2 {
			count++
		}
	}
	return count
}

// teamSide resolves which side a player's team played in a round. Stored team
// keys are "team_a"/"team_b"; anything else falls back to comparing against the
// demo's team A name.
func teamSide(r storage.MatchRound, teamKey, teamA string) string {
	key := strings.ToLower(strings.TrimSpace(teamKey))
	switch key {
	case "team_a", "a":
		return normalizeSide(r.TeamASide)
	case "team_b", "b":
		return normalizeSide(r.TeamBSide)
	}
	if teamA != "" && key != "" && key == strings.ToLower(strings.TrimSpace(teamA)) {
		return normalizeSide(r.TeamASide)
	}
	if key != "" {
		return normalizeSide(r.TeamBSide)
	}
	return ""
}

func cell(v mapgeom.Vec3, grid float64) string {
	return strings.Join([]string{
		itoa(int(math.Round(float64(v[0]) / grid))),
		itoa(int(math.Round(float64(v[1]) / grid))),
	}, ",")
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func normalizeSide(side string) string {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "ct", "counter-terrorist", "counterterrorist", "2":
		return "ct"
	case "t", "terrorist", "1":
		return "t"
	}
	return strings.ToLower(strings.TrimSpace(side))
}

func normalizeType(t string) string {
	s := strings.ToLower(strings.TrimSpace(t))
	switch {
	case strings.Contains(s, "smoke"):
		return "smoke"
	case strings.Contains(s, "flash"):
		return "flash"
	case strings.Contains(s, "molotov"):
		return "molotov"
	case strings.Contains(s, "incendiary"):
		return "incendiary"
	case strings.Contains(s, "decoy"):
		return "decoy"
	case strings.Contains(s, "hegrenade"), strings.Contains(s, "he_grenade"), strings.Contains(s, "highexplosive"):
		return "he"
	}
	switch s {
	case "smoke", "smokegrenade", "smoke grenade":
		return "smoke"
	case "flash", "flashbang":
		return "flash"
	case "he", "hegrenade", "he grenade", "highexplosivegrenade":
		return "he"
	case "molotov":
		return "molotov"
	case "incendiary", "incgrenade", "incendiarygrenade":
		return "incendiary"
	case "decoy", "decoygrenade":
		return "decoy"
	}
	return s
}

// roundOne rounds to one decimal place, matching the precision used in exports.
func roundOne(v float64) float64 {
	return math.Round(v*10) / 10
}
