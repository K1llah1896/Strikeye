// Package team aggregates cross-match results and rosters for one team name.
package team

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"strikeye/internal/storage"
)

// MapResult is one map played by the team.
type MapResult struct {
	DemoID        int64   `json:"demo_id"`
	MapName       string  `json:"map_name"`
	MatchDate     string  `json:"match_date"`
	Opponent      string  `json:"opponent"`
	TeamScore     int     `json:"team_score"`
	OpponentScore int     `json:"opponent_score"`
	Won           bool    `json:"won"`
	RoundsPlayed  int     `json:"rounds_played"`
	Rating        float64 `json:"rating"`
}

// RosterEntry aggregates one player across the team's parsed maps.
type RosterEntry struct {
	SteamID64  string  `json:"steam_id64"`
	PlayerName string  `json:"player_name"`
	Maps       int     `json:"maps"`
	Rounds     int     `json:"rounds"`
	Kills      int     `json:"kills"`
	Deaths     int     `json:"deaths"`
	Assists    int     `json:"assists"`
	ADR        float64 `json:"adr"`
	KAST       float64 `json:"kast_percent"`
	Rating     float64 `json:"rating"`
}

// Report is the cross-match dossier for one team name.
type Report struct {
	Team       string        `json:"team"`
	Maps       []MapResult   `json:"maps"`
	MapWins    int           `json:"map_wins"`
	MapLosses  int           `json:"map_losses"`
	RoundsWon  int           `json:"rounds_won"`
	RoundsLost int           `json:"rounds_lost"`
	Roster     []RosterEntry `json:"roster"`
	MatchDates []string      `json:"match_dates"`
}

type rosterAcc struct {
	entry       RosterEntry
	damage      int
	kastRounds  float64
	ratingRound float64
}

// Build aggregates every parsed demo whose team_a or team_b name matches name
// case-insensitively.
func Build(ctx context.Context, db *storage.DB, name string) (Report, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	rep := Report{Team: strings.TrimSpace(name)}
	if want == "" {
		return rep, fmt.Errorf("team name is required")
	}
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return rep, fmt.Errorf("list demos: %w", err)
	}
	roster := map[string]*rosterAcc{}
	dates := map[string]bool{}
	for _, d := range demos {
		if d.ParseStatus != "parsed" {
			continue
		}
		isA := strings.ToLower(strings.TrimSpace(d.TeamAName)) == want
		isB := strings.ToLower(strings.TrimSpace(d.TeamBName)) == want
		if !isA && !isB {
			continue
		}
		oppName, teamScore, oppScore := d.TeamBName, d.TeamAScore, d.TeamBScore
		if isB {
			oppName, teamScore, oppScore = d.TeamAName, d.TeamBScore, d.TeamAScore
		}
		rounds := d.TotalRounds
		if rounds <= 0 {
			rounds = teamScore + oppScore
		}
		mapName := ""
		if d.MapName != nil {
			mapName = *d.MapName
		}
		date := ""
		if d.MatchDate != nil {
			date = *d.MatchDate
			dates[date] = true
		}
		players, err := db.Analysis().ListMatchPlayers(ctx, d.ID)
		if err != nil {
			return rep, fmt.Errorf("load players for demo %d: %w", d.ID, err)
		}
		mapRating, mapRounds := 0.0, 0
		for _, p := range players {
			if !teamSideMatches(p.TeamKey, isA) {
				continue
			}
			acc := roster[p.SteamID64]
			if acc == nil {
				acc = &rosterAcc{entry: RosterEntry{SteamID64: p.SteamID64, PlayerName: p.PlayerName}}
				roster[p.SteamID64] = acc
			}
			if acc.entry.PlayerName == "" {
				acc.entry.PlayerName = p.PlayerName
			}
			acc.entry.Maps++
			acc.entry.Rounds += rounds
			acc.entry.Kills += p.Kills
			acc.entry.Deaths += p.Deaths
			acc.entry.Assists += p.Assists
			acc.damage += p.TotalDamage
			acc.kastRounds += p.KASTPercent / 100 * float64(rounds)
			acc.ratingRound += p.StrikeyeRating * float64(rounds)
			mapRating += p.StrikeyeRating * float64(rounds)
			mapRounds += rounds
		}
		mr := MapResult{
			DemoID: d.ID, MapName: mapName, MatchDate: date, Opponent: oppName,
			TeamScore: teamScore, OpponentScore: oppScore, Won: teamScore > oppScore,
			RoundsPlayed: rounds,
		}
		if mapRounds > 0 {
			mr.Rating = round1(mapRating / float64(mapRounds))
		}
		rep.Maps = append(rep.Maps, mr)
		if mr.Won {
			rep.MapWins++
		} else {
			rep.MapLosses++
		}
		rep.RoundsWon += teamScore
		rep.RoundsLost += oppScore
	}
	for _, acc := range roster {
		e := acc.entry
		if e.Rounds > 0 {
			e.ADR = round1(float64(acc.damage) / float64(e.Rounds))
			e.KAST = round1(acc.kastRounds / float64(e.Rounds) * 100)
			e.Rating = round2(acc.ratingRound / float64(e.Rounds))
		}
		rep.Roster = append(rep.Roster, e)
	}
	sort.Slice(rep.Roster, func(i, j int) bool {
		if rep.Roster[i].Rounds != rep.Roster[j].Rounds {
			return rep.Roster[i].Rounds > rep.Roster[j].Rounds
		}
		return rep.Roster[i].Rating > rep.Roster[j].Rating
	})
	for d := range dates {
		rep.MatchDates = append(rep.MatchDates, d)
	}
	sort.Strings(rep.MatchDates)
	return rep, nil
}

func teamSideMatches(teamKey string, isTeamA bool) bool {
	k := strings.ToLower(strings.TrimSpace(teamKey))
	if isTeamA {
		return k == "team_a" || k == "a"
	}
	return k == "team_b" || k == "b"
}

func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }
func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
