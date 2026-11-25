// Package h2h summarises head-to-head history between two teams across every
// stored match: overall score, per-map record, and the most recent meetings.
package h2h

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"strikeye/internal/storage"
)

// MapRecord is the head-to-head record on one map.
type MapRecord struct {
	Map     string `json:"map"`
	Matches int    `json:"matches"`
	WinsA   int    `json:"wins_a"`
	WinsB   int    `json:"wins_b"`
	RoundsA int    `json:"rounds_a"`
	RoundsB int    `json:"rounds_b"`
}

// MatchRecord is one stored meeting.
type MatchRecord struct {
	DemoID int64  `json:"demo_id"`
	Date   string `json:"date"`
	Map    string `json:"map"`
	ScoreA int    `json:"score_a"`
	ScoreB int    `json:"score_b"`
	Winner string `json:"winner"`
}

// Report is the head-to-head summary for a pair of teams.
type Report struct {
	TeamA   string        `json:"team_a"`
	TeamB   string        `json:"team_b"`
	Matches int           `json:"matches"`
	WinsA   int           `json:"wins_a"`
	WinsB   int           `json:"wins_b"`
	RoundsA int           `json:"rounds_a"`
	RoundsB int           `json:"rounds_b"`
	Maps    []MapRecord   `json:"maps"`
	Recent  []MatchRecord `json:"recent"`
}

// Build collects every parsed match between the two teams.
func Build(ctx context.Context, db *storage.DB, teamA, teamB string) (Report, error) {
	a := strings.TrimSpace(teamA)
	b := strings.TrimSpace(teamB)
	if a == "" || b == "" {
		return Report{}, fmt.Errorf("both team names are required")
	}
	if strings.EqualFold(a, b) {
		return Report{}, fmt.Errorf("team names must differ")
	}
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("list demos: %w", err)
	}
	rep := Report{TeamA: a, TeamB: b}
	byMap := map[string]*MapRecord{}

	for _, demo := range demos {
		if demo.ParseStatus != "parsed" {
			continue
		}
		// Normalise orientation so A/B always refer to the requested teams,
		// whichever slot the demo stored them in.
		scoreA, scoreB := 0, 0
		switch {
		case strings.EqualFold(demo.TeamAName, a) && strings.EqualFold(demo.TeamBName, b):
			scoreA, scoreB = demo.TeamAScore, demo.TeamBScore
		case strings.EqualFold(demo.TeamAName, b) && strings.EqualFold(demo.TeamBName, a):
			scoreA, scoreB = demo.TeamBScore, demo.TeamAScore
		default:
			continue
		}
		mapName := ""
		if demo.MapName != nil {
			mapName = *demo.MapName
		}
		date := ""
		if demo.MatchDate != nil {
			date = *demo.MatchDate
		}
		rep.Matches++
		rep.RoundsA += scoreA
		rep.RoundsB += scoreB
		winner := "draw"
		switch {
		case scoreA > scoreB:
			rep.WinsA++
			winner = a
		case scoreB > scoreA:
			rep.WinsB++
			winner = b
		}
		if mapName != "" {
			m := byMap[mapName]
			if m == nil {
				m = &MapRecord{Map: mapName}
				byMap[mapName] = m
			}
			m.Matches++
			m.RoundsA += scoreA
			m.RoundsB += scoreB
			if scoreA > scoreB {
				m.WinsA++
			} else if scoreB > scoreA {
				m.WinsB++
			}
		}
		rep.Recent = append(rep.Recent, MatchRecord{
			DemoID: demo.ID, Date: date, Map: mapName,
			ScoreA: scoreA, ScoreB: scoreB, Winner: winner,
		})
	}

	for _, m := range byMap {
		rep.Maps = append(rep.Maps, *m)
	}
	sort.Slice(rep.Maps, func(i, j int) bool {
		if rep.Maps[i].Matches != rep.Maps[j].Matches {
			return rep.Maps[i].Matches > rep.Maps[j].Matches
		}
		return rep.Maps[i].Map < rep.Maps[j].Map
	})
	sort.SliceStable(rep.Recent, func(i, j int) bool {
		if rep.Recent[i].Date != rep.Recent[j].Date {
			return rep.Recent[i].Date > rep.Recent[j].Date
		}
		return rep.Recent[i].DemoID > rep.Recent[j].DemoID
	})
	return rep, nil
}
