package analysis

import (
	"context"
	"fmt"
	"strings"

	"strikeye/internal/profile"
)

// loadTrend fetches a player's cross-match history for the drawer sparkline.
func (v *View) loadTrend(steamID string) {
	v.mu.Lock()
	if v.trends == nil {
		v.trends = map[string][]profile.TrendPoint{}
	}
	if v.trendLoading == nil {
		v.trendLoading = map[string]bool{}
	}
	if _, done := v.trends[steamID]; done || v.trendLoading[steamID] {
		v.mu.Unlock()
		return
	}
	v.trendLoading[steamID] = true
	db := v.db
	v.mu.Unlock()

	go func() {
		points, err := profile.Trend(context.Background(), db, steamID)
		v.mu.Lock()
		if err == nil {
			v.trends[steamID] = points
		} else {
			v.trends[steamID] = nil
		}
		delete(v.trendLoading, steamID)
		v.mu.Unlock()
		v.invalidate()
	}()
}

// trendFor returns the cached trend for a player.
func (v *View) trendFor(steamID string) []profile.TrendPoint {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.trends[steamID]
}

// trendSummary renders a one-line recap of the player's last matches.
func trendSummary(points []profile.TrendPoint, maxGames int) (string, string) {
	if maxGames > 0 && len(points) > maxGames {
		points = points[len(points)-maxGames:]
	}
	if len(points) == 0 {
		return "", ""
	}
	ratings := make([]float64, 0, len(points))
	adrs := make([]float64, 0, len(points))
	for _, p := range points {
		ratings = append(ratings, p.Rating)
		adrs = append(adrs, p.ADR)
	}
	first, last := points[0], points[len(points)-1]
	wins := 0
	for _, p := range points {
		if p.Won {
			wins++
		}
	}
	ratingLine := fmt.Sprintf("近 %d 场 Rating %s  %.2f → %.2f (%d 胜)",
		len(points), sparkline(ratings), first.Rating, last.Rating, wins)
	adrLine := fmt.Sprintf("ADR %s  %.1f → %.1f", sparkline(adrs), first.ADR, last.ADR)
	return ratingLine, adrLine
}

// trendMatchesText lists the last few opponents, newest last.
func trendMatchesText(points []profile.TrendPoint, maxGames int) string {
	if maxGames > 0 && len(points) > maxGames {
		points = points[len(points)-maxGames:]
	}
	parts := make([]string, 0, len(points))
	for _, p := range points {
		name := p.Opponent
		if name == "" {
			name = "未知对手"
		}
		mapName := p.Map
		if mapName == "" {
			mapName = "?"
		}
		result := "负"
		if p.Won {
			result = "胜"
		}
		parts = append(parts, fmt.Sprintf("%s %s %s %.2f", mapName, strings.ToUpper(name[:minInt(len(name), 12)]), result, p.Rating))
	}
	return strings.Join(parts, "  ·  ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
