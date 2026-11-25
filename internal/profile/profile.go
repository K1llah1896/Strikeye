// Package profile combines cross-match performance aggregates with the
// user-maintained player identity (display name, note, tags).
package profile

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"strikeye/internal/export"
	"strikeye/internal/storage"
)

// Report is the full player dossier returned to the CLI and UI layers.
type Report struct {
	Aggregate  export.PlayerAggregate `json:"aggregate"`
	Profile    storage.PlayerProfile  `json:"profile"`
	HasProfile bool                   `json:"has_profile"`
	// Bans is the cached Steam ban snapshot, when one has been fetched with
	// `strikeye-cli bans`. It is nil rather than zero-valued so the report can
	// distinguish "clean" from "never checked".
	Bans *storage.PlayerBans `json:"bans,omitempty"`
}

// Build loads the aggregated match rows plus the stored profile (if any).
// It returns an error when the SteamID64 has no parsed matches.
func Build(ctx context.Context, db *storage.DB, steamID string) (Report, error) {
	agg, err := export.AggregatePlayer(ctx, db, steamID)
	if err != nil {
		return Report{}, err
	}
	prof, found, err := db.PlayerProfiles().Get(ctx, agg.SteamID64)
	if err != nil {
		return Report{}, fmt.Errorf("load profile: %w", err)
	}
	if !found {
		prof = storage.PlayerProfile{SteamID64: agg.SteamID64, Tags: []string{}}
	}
	if prof.DisplayName != "" {
		agg.PlayerName = prof.DisplayName
	}
	rep := Report{Aggregate: agg, Profile: prof, HasProfile: found}
	if bans, ok, err := db.PlayerBans().Get(ctx, agg.SteamID64); err == nil && ok {
		rep.Bans = &bans
	}
	return rep, nil
}

// Save upserts the editable profile fields. Empty name/note keep their stored
// value unless replace is true; tags always replace when provided.
func Save(ctx context.Context, db *storage.DB, steamID, name, note string, tags []string) (storage.PlayerProfile, error) {
	steamID = strings.TrimSpace(steamID)
	if steamID == "" {
		return storage.PlayerProfile{}, fmt.Errorf("steamid is required")
	}
	existing, found, err := db.PlayerProfiles().Get(ctx, steamID)
	if err != nil {
		return storage.PlayerProfile{}, err
	}
	if !found {
		existing = storage.PlayerProfile{SteamID64: steamID}
	}
	if strings.TrimSpace(name) != "" {
		existing.DisplayName = strings.TrimSpace(name)
	}
	if strings.TrimSpace(note) != "" {
		existing.Note = strings.TrimSpace(note)
	}
	if tags != nil {
		existing.Tags = storage.NormalizeTags(tags)
	}
	return db.PlayerProfiles().Upsert(ctx, existing)
}

// TrendPoint is one match of a player's cross-match history.
type TrendPoint struct {
	DemoID    int64   `json:"demo_id"`
	Map       string  `json:"map"`
	Date      string  `json:"date"`
	Opponent  string  `json:"opponent"`
	TeamKey   string  `json:"team_key"`
	Rounds    int     `json:"rounds"`
	Kills     int     `json:"kills"`
	Deaths    int     `json:"deaths"`
	Assists   int     `json:"assists"`
	ADR       float64 `json:"adr"`
	KAST      float64 `json:"kast"`
	HSPercent float64 `json:"hs_percent"`
	Rating    float64 `json:"rating"`
	Won       bool    `json:"won"`
}

// Trend returns the player's matches in chronological order so callers can draw
// a rating/ADR curve instead of a single aggregate.
func Trend(ctx context.Context, db *storage.DB, steamID string) ([]TrendPoint, error) {
	steamID = strings.TrimSpace(steamID)
	if steamID == "" {
		return nil, fmt.Errorf("steamid is required")
	}
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list demos: %w", err)
	}
	var out []TrendPoint
	for _, demo := range demos {
		if demo.ParseStatus != "parsed" {
			continue
		}
		players, err := db.Analysis().ListMatchPlayers(ctx, demo.ID)
		if err != nil {
			return nil, fmt.Errorf("list players for demo %d: %w", demo.ID, err)
		}
		found := false
		for _, p := range players {
			if p.SteamID64 != steamID {
				continue
			}
			rounds := demo.TotalRounds
			if rounds <= 0 {
				if rs, err := db.Analysis().ListMatchRounds(ctx, demo.ID); err == nil {
					rounds = len(rs)
				}
			}
			point := TrendPoint{
				DemoID: demo.ID, TeamKey: p.TeamKey, Rounds: rounds,
				Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists,
				ADR: p.ADR, KAST: p.KASTPercent, HSPercent: p.HSPercent, Rating: p.StrikeyeRating,
			}
			if demo.MapName != nil {
				point.Map = *demo.MapName
			}
			if demo.MatchDate != nil {
				point.Date = *demo.MatchDate
			}
			switch p.TeamKey {
			case "team_a":
				point.Opponent = demo.TeamBName
				point.Won = demo.TeamAScore > demo.TeamBScore
			case "team_b":
				point.Opponent = demo.TeamAName
				point.Won = demo.TeamBScore > demo.TeamAScore
			}
			out = append(out, point)
			found = true
			break
		}
		if !found {
			continue
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].DemoID < out[j].DemoID
	})
	return out, nil
}
