// Package series groups parsed demos into multi-map series (best-of-N).
package series

import (
	"sort"
	"strings"

	"strikeye/internal/storage"
)

// MapResult is one map inside a series, oriented to the series' TeamA/TeamB.
type MapResult struct {
	DemoID    int64  `json:"demo_id"`
	MapName   string `json:"map_name"`
	MatchDate string `json:"match_date,omitempty"`
	ScoreA    int    `json:"score_a"`
	ScoreB    int    `json:"score_b"`
	Winner    string `json:"winner,omitempty"`
	Swapped   bool   `json:"swapped"`
}

// Series is a set of >=2 demos between the same two teams on the same day.
type Series struct {
	Key      string      `json:"key"`
	TeamA    string      `json:"team_a"`
	TeamB    string      `json:"team_b"`
	Date     string      `json:"date,omitempty"`
	Maps     []MapResult `json:"maps"`
	MapWinsA int         `json:"map_wins_a"`
	MapWinsB int         `json:"map_wins_b"`
}

// Label renders "A 2-1 B" for headers.
func (s Series) Label() string {
	return strings.TrimSpace(s.TeamA) + " " + itoa(s.MapWinsA) + "-" + itoa(s.MapWinsB) + " " + strings.TrimSpace(s.TeamB)
}

// Group builds series from demos. Demos without two real team names, unparsed
// demos, and groups with fewer than two maps are skipped.
func Group(demos []storage.Demo) []Series {
	type bucket struct {
		maps               []MapResult
		teamA, teamB, date string
	}
	buckets := make(map[string]*bucket)
	for _, d := range demos {
		if d.ParseStatus != "parsed" {
			continue
		}
		a := strings.TrimSpace(d.TeamAName)
		b := strings.TrimSpace(d.TeamBName)
		if a == "" || b == "" || isPlaceholderTeam(a) || isPlaceholderTeam(b) || strings.EqualFold(a, b) {
			continue
		}
		date := dayOf(d)
		key := seriesKey(a, b, date)
		bk := buckets[key]
		if bk == nil {
			anchorA, anchorB := orderTeams(a, b)
			bk = &bucket{teamA: anchorA, teamB: anchorB, date: date}
			buckets[key] = bk
		}
		swapped := !strings.EqualFold(strings.TrimSpace(d.TeamAName), bk.teamA)
		scoreA, scoreB := d.TeamAScore, d.TeamBScore
		if swapped {
			scoreA, scoreB = scoreB, scoreA
		}
		m := MapResult{
			DemoID:    d.ID,
			MapName:   deref(d.MapName),
			MatchDate: deref(d.MatchDate),
			ScoreA:    scoreA,
			ScoreB:    scoreB,
			Swapped:   swapped,
		}
		switch {
		case scoreA > scoreB:
			m.Winner = "a"
		case scoreB > scoreA:
			m.Winner = "b"
		}
		bk.maps = append(bk.maps, m)
	}

	out := make([]Series, 0, len(buckets))
	for key, bk := range buckets {
		if len(bk.maps) < 2 {
			continue
		}
		sort.Slice(bk.maps, func(i, j int) bool {
			if bk.maps[i].MatchDate != bk.maps[j].MatchDate {
				return bk.maps[i].MatchDate < bk.maps[j].MatchDate
			}
			return bk.maps[i].DemoID < bk.maps[j].DemoID
		})
		s := Series{Key: key, TeamA: bk.teamA, TeamB: bk.teamB, Date: bk.date, Maps: bk.maps}
		for _, m := range bk.maps {
			if m.Winner == "a" {
				s.MapWinsA++
			} else if m.Winner == "b" {
				s.MapWinsB++
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// ByDemoID maps every demo ID that belongs to a multi-map series to that series.
func ByDemoID(series []Series) map[int64]Series {
	out := make(map[int64]Series)
	for _, s := range series {
		for _, m := range s.Maps {
			out[m.DemoID] = s
		}
	}
	return out
}

func seriesKey(a, b, date string) string {
	anchorA, anchorB := orderTeams(a, b)
	return strings.ToLower(anchorA) + "|" + strings.ToLower(anchorB) + "|" + date
}

// orderTeams returns a deterministic A/B orientation so a series where the
// demo's team_a/team_b columns swap between maps still aggregates correctly.
func orderTeams(a, b string) (string, string) {
	if strings.ToLower(a) <= strings.ToLower(b) {
		return a, b
	}
	return b, a
}

func dayOf(d storage.Demo) string {
	date := strings.TrimSpace(deref(d.MatchDate))
	if len(date) >= 10 {
		return date[:10]
	}
	if len(d.AddedAt) >= 10 {
		return d.AddedAt[:10]
	}
	return date
}

func isPlaceholderTeam(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "team a", "team b", "team ct", "team t", "unknown":
		return true
	}
	return false
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
