package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"

	"strikeye/internal/h2h"
	"strikeye/internal/profile"
	"strikeye/internal/storage"
)

// runHeadToHead prints the head-to-head record between two teams.
func runHeadToHead(ctx context.Context, dataRoot string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("h2h", flag.ContinueOnError)
	fs.SetOutput(out)
	teamA := fs.String("team-a", "", "first team name")
	teamB := fs.String("team-b", "", "second team name")
	format := fs.String("format", "json", "output format: json or csv")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	rep, err := h2h.Build(ctx, db, *teamA, *teamB)
	if err != nil {
		return err
	}
	if *format == "csv" {
		cw := csv.NewWriter(out)
		if err := cw.Write([]string{"kind", "name", "matches", "wins_a", "wins_b", "rounds_a", "rounds_b"}); err != nil {
			return err
		}
		if err := cw.Write([]string{"overall", rep.TeamA + " vs " + rep.TeamB,
			strconv.Itoa(rep.Matches), strconv.Itoa(rep.WinsA), strconv.Itoa(rep.WinsB),
			strconv.Itoa(rep.RoundsA), strconv.Itoa(rep.RoundsB)}); err != nil {
			return err
		}
		for _, m := range rep.Maps {
			if err := cw.Write([]string{"map", m.Map,
				strconv.Itoa(m.Matches), strconv.Itoa(m.WinsA), strconv.Itoa(m.WinsB),
				strconv.Itoa(m.RoundsA), strconv.Itoa(m.RoundsB)}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// runTrends prints one player's match-by-match history.
func runTrends(ctx context.Context, dataRoot string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("trends", flag.ContinueOnError)
	fs.SetOutput(out)
	steamID := fs.String("steamid", "", "SteamID64")
	format := fs.String("format", "json", "output format: json or csv")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *steamID == "" {
		return fmt.Errorf("--steamid is required")
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	points, err := profile.Trend(ctx, db, *steamID)
	if err != nil {
		return err
	}
	if len(points) == 0 {
		return fmt.Errorf("no parsed matches for steamid %s", *steamID)
	}
	if *format == "csv" {
		cw := csv.NewWriter(out)
		if err := cw.Write([]string{"demo_id", "date", "map", "opponent", "team_key", "rounds", "kills", "deaths", "assists", "adr", "kast", "hs_pct", "rating", "won"}); err != nil {
			return err
		}
		for _, p := range points {
			if err := cw.Write([]string{
				strconv.FormatInt(p.DemoID, 10), p.Date, p.Map, p.Opponent, p.TeamKey,
				strconv.Itoa(p.Rounds), strconv.Itoa(p.Kills), strconv.Itoa(p.Deaths), strconv.Itoa(p.Assists),
				strconv.FormatFloat(p.ADR, 'f', 1, 64), strconv.FormatFloat(p.KAST, 'f', 1, 64),
				strconv.FormatFloat(p.HSPercent, 'f', 1, 64), strconv.FormatFloat(p.Rating, 'f', 2, 64),
				strconv.FormatBool(p.Won),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(points)
}
