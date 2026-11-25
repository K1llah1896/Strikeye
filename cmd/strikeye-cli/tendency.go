package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"strikeye/internal/callouts"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
	"strikeye/internal/tendency"
)

// runTendencies prints the scouting profile for one team.
func runTendencies(ctx context.Context, dataRoot string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("tendencies", flag.ContinueOnError)
	fs.SetOutput(out)
	team := fs.String("team", "", "team name (as stored in demos)")
	top := fs.Int("top", 8, "how many opening callouts to report")
	format := fs.String("format", "json", "output format: json or csv")
	dest := fs.String("out", "", "write to this file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *team == "" {
		return fmt.Errorf("--team is required")
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	demos, err := db.Demos().List(ctx)
	if err != nil {
		return fmt.Errorf("list demos: %w", err)
	}
	replays := map[int64][]parser.ReplayRoundData{}
	for i := range demos {
		demo := &demos[i]
		if demo.ParseStatus != "parsed" {
			continue
		}
		rounds, err := loadReplayRounds(dataRoot, demo)
		if err != nil {
			continue
		}
		list := make([]parser.ReplayRoundData, 0, len(rounds))
		for _, rd := range rounds {
			list = append(list, rd)
		}
		replays[demo.ID] = list
	}

	var table *callouts.Table
	if t, err := callouts.LoadCSV(filepath.Join(dataRoot, "cache", "map_data", "callouts.csv")); err == nil {
		table = t
	}

	rep, err := tendency.Build(ctx, db, tendency.Input{Team: *team, Replays: replays, Callouts: table, TopN: *top})
	if err != nil {
		return err
	}
	if rep.Rounds == 0 {
		return fmt.Errorf("no parsed matches found for team %q", *team)
	}
	if rep.CalloutsMissing() {
		fmt.Fprintln(os.Stderr, "提示: 缺少 callout 数据，开场击杀分区为空（运行 strikeye-cli callouts-fetch 补齐）")
	}

	var sink io.Writer = out
	if *dest != "" {
		if err := os.MkdirAll(filepath.Dir(*dest), 0o755); err != nil && filepath.Dir(*dest) != "." {
			return err
		}
		f, err := os.Create(*dest)
		if err != nil {
			return err
		}
		defer f.Close()
		sink = f
	}
	switch *format {
	case "json", "":
		enc := json.NewEncoder(sink)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	case "csv":
		return writeTendencyCSV(sink, rep)
	default:
		return fmt.Errorf("unknown format %q (want json or csv)", *format)
	}
}

// writeTendencyCSV flattens the profile into one row per fact so spreadsheets
// can pivot on kind/side/name.
func writeTendencyCSV(w io.Writer, rep tendency.Report) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"kind", "map", "side", "name", "rounds", "wins", "win_pct", "extra"}); err != nil {
		return err
	}
	row := func(kind, mapName, side, name string, rounds, wins int, pct float64, extra string) error {
		return cw.Write([]string{kind, mapName, side, name, strconv.Itoa(rounds), strconv.Itoa(wins), strconv.FormatFloat(pct, 'f', 2, 64), extra})
	}
	for _, s := range rep.Sides {
		extra := fmt.Sprintf("equip=%d;utility_per_round=%s;pistols=%d/%d",
			s.AvgEquip, strconv.FormatFloat(s.UtilityPerRound, 'f', 2, 64), s.PistolWins, s.Pistols)
		if err := row("side", s.Map, s.Side, s.Side, s.Rounds, s.Wins, s.WinPct, extra); err != nil {
			return err
		}
	}
	for _, e := range rep.Economy {
		if err := row("economy", "", e.Side, e.Type, e.Rounds, e.Wins, e.WinPct, ""); err != nil {
			return err
		}
	}
	for _, s := range rep.Sites {
		if err := row("site", s.Map, "", s.Site, s.Plants, s.Wins, s.WinPct, ""); err != nil {
			return err
		}
	}
	for _, o := range rep.Opening {
		if err := row("opening", "", "", o.Callout, o.Kills, 0, o.Share, ""); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
