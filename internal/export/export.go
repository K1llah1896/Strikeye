// Package export serializes parsed match data into portable CSV/JSON documents.
package export

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"strikeye/internal/storage"
	"strikeye/internal/xlsx"
)

// SchemaVersion is embedded in every JSON document for forward compatibility.
const SchemaVersion = 1

// Formats selects which documents to write.
type Formats struct {
	CSV  bool
	JSON bool
	HTML bool
	XLSX bool
}

// ParseFormats parses "csv", "json", "both", or a comma-separated list.
func ParseFormats(raw string) (Formats, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "both":
		return Formats{CSV: true, JSON: true}, nil
	case "csv":
		return Formats{CSV: true}, nil
	case "json":
		return Formats{JSON: true}, nil
	case "html":
		return Formats{HTML: true}, nil
	case "xlsx":
		return Formats{XLSX: true}, nil
	case "all":
		return Formats{CSV: true, JSON: true, HTML: true, XLSX: true}, nil
	}
	var f Formats
	for _, part := range strings.Split(raw, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "csv":
			f.CSV = true
		case "json":
			f.JSON = true
		case "html":
			f.HTML = true
		case "xlsx":
			f.XLSX = true
		case "all":
			f.CSV, f.JSON, f.HTML, f.XLSX = true, true, true, true
		default:
			return Formats{}, fmt.Errorf("unsupported export format %q (want csv, json, html, xlsx, all, or both)", raw)
		}
	}
	if !f.CSV && !f.JSON && !f.HTML && !f.XLSX {
		return Formats{}, fmt.Errorf("no export format selected")
	}
	return f, nil
}

// MatchBundle is the read-only snapshot exported for one demo.
type MatchBundle struct {
	Demo    storage.Demo              `json:"demo"`
	Sides   []storage.MatchPlayerSide `json:"sides"`
	Players []storage.MatchPlayer     `json:"players"`
	Rounds  []storage.MatchRound      `json:"rounds"`
	Events  []storage.MatchEvent      `json:"events"`
	// Kills carries the stored per-kill geometry rows (see `killctx --store`).
	// It stays empty until that command has run for the demo.
	Kills []storage.KillContext `json:"kills"`
}

// LoadMatchBundle reads every analysis table for a demo.
func LoadMatchBundle(ctx context.Context, db *storage.DB, demoID int64) (MatchBundle, error) {
	demo, err := db.Demos().GetByID(ctx, demoID)
	if err != nil {
		return MatchBundle{}, fmt.Errorf("load demo %d: %w", demoID, err)
	}
	sides, err := db.Analysis().ListMatchPlayerSides(ctx, demoID)
	if err != nil {
		return MatchBundle{}, fmt.Errorf("load player sides: %w", err)
	}
	players, err := db.Analysis().ListMatchPlayers(ctx, demoID)
	if err != nil {
		return MatchBundle{}, fmt.Errorf("load players: %w", err)
	}
	rounds, err := db.Analysis().ListMatchRounds(ctx, demoID)
	if err != nil {
		return MatchBundle{}, fmt.Errorf("load rounds: %w", err)
	}
	events, err := db.Analysis().ListMatchEvents(ctx, demoID, nil)
	if err != nil {
		return MatchBundle{}, fmt.Errorf("load events: %w", err)
	}
	kills, err := db.KillContext().ListMatchKillContext(ctx, demoID)
	if err != nil {
		return MatchBundle{}, fmt.Errorf("load kill context: %w", err)
	}
	return MatchBundle{Demo: demo, Sides: sides, Players: players, Rounds: rounds, Events: events, Kills: kills}, nil
}

// WriteMatchBundle writes the selected formats into dir and returns written paths.
func WriteMatchBundle(bundle MatchBundle, dir string, formats Formats) ([]string, error) {
	id := bundle.Demo.ID
	if id <= 0 {
		id = bundle.Demo.ID
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create export dir: %w", err)
	}
	var written []string
	prefix := fmt.Sprintf("match_%d", id)
	if formats.CSV {
		files := []struct {
			suffix string
			write  func(string) error
		}{
			{"_players.csv", func(p string) error { return writePlayersCSV(p, bundle.Players) }},
			{"_rounds.csv", func(p string) error { return writeRoundsCSV(p, bundle.Rounds) }},
			{"_events.csv", func(p string) error { return writeEventsCSV(p, bundle.Events) }},
			{"_sides.csv", func(p string) error { return writeSidesCSV(p, bundle.Sides) }},
			{"_kills.csv", func(p string) error {
				header, rows := killsTable(bundle.Kills)
				return writeCSV(p, header, rows)
			}},
		}
		for _, f := range files {
			path := filepath.Join(dir, prefix+f.suffix)
			if err := f.write(path); err != nil {
				return written, err
			}
			written = append(written, path)
		}
	}
	if formats.XLSX {
		path := filepath.Join(dir, prefix+".xlsx")
		if err := writeMatchXLSX(path, bundle); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	if formats.HTML {
		path := filepath.Join(dir, prefix+".html")
		if err := writeMatchHTML(path, bundle); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	if formats.JSON {
		path := filepath.Join(dir, prefix+".json")
		doc := struct {
			SchemaVersion int                       `json:"schema_version"`
			Demo          storage.Demo              `json:"demo"`
			Sides         []storage.MatchPlayerSide `json:"sides"`
			Players       []storage.MatchPlayer     `json:"players"`
			Rounds        []storage.MatchRound      `json:"rounds"`
			Events        []storage.MatchEvent      `json:"events"`
			Kills         []storage.KillContext     `json:"kills"`
		}{SchemaVersion, bundle.Demo, bundle.Sides, bundle.Players, bundle.Rounds, bundle.Events, bundle.Kills}
		if err := writeJSON(path, doc); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// killsTable flattens the stored kill geometry rows for CSV and XLSX.
func killsTable(kills []storage.KillContext) ([]string, [][]string) {
	header := []string{
		"round_number", "tick", "time_sec", "killer_name", "victim_name", "weapon", "headshot",
		"distance", "visible", "geometry_checked", "smoke_blocked", "optics_checked",
		"aim_off_deg", "in_fov", "victim_sees_killer", "attacker_blind", "victim_blind",
		"attacker_zoom", "victim_zoom", "killer_x", "killer_y", "killer_z",
		"victim_x", "victim_y", "victim_z",
	}
	rows := make([][]string, 0, len(kills))
	for _, k := range kills {
		rows = append(rows, []string{
			strconv.Itoa(k.RoundNumber), strconv.FormatInt(k.Tick, 10), round2(k.TimeSec),
			k.KillerName, k.VictimName, k.Weapon, boolStr(k.Headshot), round2(k.Distance),
			boolStr(k.Visible), boolStr(k.GeometryChecked), boolStr(k.SmokeBlocked),
			boolStr(k.OpticsChecked), round2(k.AimOffDeg), boolStr(k.InFOV),
			boolStr(k.VictimSeesKiller), boolStr(k.AttackerBlind), boolStr(k.VictimBlind),
			strconv.Itoa(k.AttackerZoom), strconv.Itoa(k.VictimZoom),
			round2(k.KillerX), round2(k.KillerY), round2(k.KillerZ),
			round2(k.VictimX), round2(k.VictimY), round2(k.VictimZ),
		})
	}
	return header, rows
}

// PlayerMatchRow is one match line in a cross-match aggregate.
type PlayerMatchRow struct {
	DemoID      int64   `json:"demo_id"`
	MatchDate   string  `json:"match_date"`
	MapName     string  `json:"map_name"`
	TeamKey     string  `json:"team_key"`
	Rounds      int     `json:"rounds"`
	Kills       int     `json:"kills"`
	Deaths      int     `json:"deaths"`
	Assists     int     `json:"assists"`
	ADR         float64 `json:"adr"`
	KASTPercent float64 `json:"kast_percent"`
	HSPercent   float64 `json:"hs_percent"`
	Rating      float64 `json:"rating"`
}

// PlayerAggregate holds per-match rows plus career totals.
type PlayerAggregate struct {
	SteamID64   string           `json:"steam_id64"`
	PlayerName  string           `json:"player_name"`
	Matches     int              `json:"matches"`
	Rounds      int              `json:"rounds"`
	Kills       int              `json:"kills"`
	Deaths      int              `json:"deaths"`
	Assists     int              `json:"assists"`
	Damage      int              `json:"total_damage"`
	KDRatio     float64          `json:"kd_ratio"`
	KPR         float64          `json:"kpr"`
	DPR         float64          `json:"dpr"`
	ADR         float64          `json:"adr"`
	KASTPercent float64          `json:"kast_percent"`
	HSPercent   float64          `json:"hs_percent"`
	Rating      float64          `json:"rating"`
	Rows        []PlayerMatchRow `json:"rows"`
}

// AggregatePlayer aggregates one SteamID64 across every parsed demo.
func AggregatePlayer(ctx context.Context, db *storage.DB, steamID string) (PlayerAggregate, error) {
	steamID = strings.TrimSpace(steamID)
	if steamID == "" {
		return PlayerAggregate{}, fmt.Errorf("steamid is required")
	}
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return PlayerAggregate{}, fmt.Errorf("list demos: %w", err)
	}
	agg := PlayerAggregate{SteamID64: steamID}
	var weightedRating float64
	var weightedKAST float64
	var latestAdded string
	for _, demo := range demos {
		if demo.ParseStatus != "parsed" {
			continue
		}
		players, err := db.Analysis().ListMatchPlayers(ctx, demo.ID)
		if err != nil {
			return PlayerAggregate{}, fmt.Errorf("list players for demo %d: %w", demo.ID, err)
		}
		for _, p := range players {
			if p.SteamID64 != steamID {
				continue
			}
			rounds := demo.TotalRounds
			if rounds <= 0 {
				rounds = len(mustRounds(ctx, db, demo.ID))
			}
			row := PlayerMatchRow{
				DemoID:      demo.ID,
				MapName:     deref(demo.MapName),
				MatchDate:   deref(demo.MatchDate),
				TeamKey:     p.TeamKey,
				Rounds:      rounds,
				Kills:       p.Kills,
				Deaths:      p.Deaths,
				Assists:     p.Assists,
				ADR:         p.ADR,
				KASTPercent: p.KASTPercent,
				HSPercent:   p.HSPercent,
				Rating:      p.StrikeyeRating,
			}
			agg.Rows = append(agg.Rows, row)
			agg.Matches++
			agg.Rounds += rounds
			agg.Kills += p.Kills
			agg.Deaths += p.Deaths
			agg.Assists += p.Assists
			agg.Damage += p.TotalDamage
			weightedRating += p.StrikeyeRating * float64(rounds)
			weightedKAST += p.KASTPercent * float64(rounds)
			if added := demo.AddedAt; added > latestAdded {
				latestAdded = added
				if p.DisplayName != "" {
					agg.PlayerName = p.DisplayName
				} else {
					agg.PlayerName = p.PlayerName
				}
			}
		}
	}
	if agg.Matches == 0 {
		return PlayerAggregate{}, fmt.Errorf("no matches found for steamid %s", steamID)
	}
	agg.KDRatio = ratio(agg.Kills, maxInt(agg.Deaths, 1))
	agg.KPR = ratioFloat(agg.Kills, agg.Rounds)
	agg.DPR = ratioFloat(agg.Deaths, agg.Rounds)
	agg.ADR = ratioFloat(agg.Damage, agg.Rounds)
	if agg.Rounds > 0 {
		agg.Rating = weightedRating / float64(agg.Rounds)
		agg.KASTPercent = weightedKAST / float64(agg.Rounds)
	}
	if agg.Kills > 0 {
		var hs int
		for _, d := range demos {
			if d.ParseStatus != "parsed" {
				continue
			}
			players, _ := db.Analysis().ListMatchPlayers(ctx, d.ID)
			for _, p := range players {
				if p.SteamID64 == steamID {
					hs += p.Headshots
				}
			}
		}
		agg.HSPercent = float64(hs) / float64(agg.Kills) * 100
	}
	sort.Slice(agg.Rows, func(i, j int) bool { return agg.Rows[i].DemoID < agg.Rows[j].DemoID })
	return agg, nil
}

// WritePlayerAggregate writes the aggregate CSV/JSON and returns written paths.
func WritePlayerAggregate(agg PlayerAggregate, dir string, formats Formats) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create export dir: %w", err)
	}
	safeID := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, agg.SteamID64)
	prefix := "player_" + safeID
	var written []string
	if formats.CSV {
		path := filepath.Join(dir, prefix+"_matches.csv")
		if err := writeAggregateCSV(path, agg); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	if formats.XLSX {
		path := filepath.Join(dir, prefix+"_matches.xlsx")
		if err := writePlayerXLSX(path, agg); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	if formats.JSON {
		path := filepath.Join(dir, prefix+"_matches.json")
		doc := struct {
			SchemaVersion int             `json:"schema_version"`
			Aggregate     PlayerAggregate `json:"aggregate"`
		}{SchemaVersion, agg}
		if err := writeJSON(path, doc); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

func playersTable(players []storage.MatchPlayer) ([]string, [][]string) {
	header := []string{
		"steam_id64", "player_name", "display_name", "team_key", "kills", "deaths", "assists",
		"kd_ratio", "kpr", "dpr", "apr", "total_damage", "adr", "utility_damage", "utility_adr",
		"kast_percent", "headshots", "hs_percent", "first_kills", "first_deaths", "opening_win_rate",
		"trade_kills", "trade_deaths", "survival_rate", "one_kill_rounds", "two_kill_rounds",
		"three_kill_rounds", "four_kill_rounds", "five_kill_rounds", "multi_kill_rounds",
		"clutch_attempts", "clutch_wins", "awp_kills", "avg_equipment_value", "impact_score",
		"rating", "eco_adj_rating", "eco_adj_adr", "round_swing", "flash_assists",
		"ttk_avg_ms", "ttk_samples", "ttd_avg_ms", "ttd_samples",
	}
	rows := make([][]string, 0, len(players))
	for _, p := range players {
		rows = append(rows, []string{
			p.SteamID64, p.PlayerName, p.DisplayName, p.TeamKey,
			itoa(p.Kills), itoa(p.Deaths), itoa(p.Assists),
			ftoa(p.KDRatio), ftoa(p.KPR), ftoa(p.DPR), ftoa(p.APR),
			itoa(p.TotalDamage), ftoa(p.ADR), itoa(p.UtilityDamage), ftoa(p.UtilityADR),
			ftoa(p.KASTPercent), itoa(p.Headshots), ftoa(p.HSPercent),
			itoa(p.FirstKills), itoa(p.FirstDeaths), ftoa(p.OpeningDuelWinRate),
			itoa(p.TradeKills), itoa(p.TradeDeaths), ftoa(p.SurvivalRate),
			itoa(p.OneKillRounds), itoa(p.TwoKillRounds), itoa(p.ThreeKillRounds),
			itoa(p.FourKillRounds), itoa(p.FiveKillRounds), itoa(p.MultiKillRounds),
			itoa(p.ClutchAttempts), itoa(p.ClutchWins), itoa(p.AWPKills),
			itoa(p.AvgEquipmentValue), ftoa(p.ImpactScore), ftoa(p.StrikeyeRating),
			ftoa(p.EcoAdjRating), ftoa(p.EcoAdjADR), ftoa(p.RoundSwing), itoa(p.FlashAssists),
			ftoa(p.TTKAvgMS), itoa(p.TTKSamples), ftoa(p.TTDAvgMS), itoa(p.TTDSamples),
		})
	}
	return header, rows
}

func roundsTable(rounds []storage.MatchRound) ([]string, [][]string) {
	header := []string{
		"round_number", "start_tick", "freeze_end_tick", "end_tick", "duration_sec",
		"winner_team_key", "winner_side", "win_reason", "bomb_site",
		"team_a_side", "team_b_side", "team_a_score_before", "team_b_score_before",
		"team_a_score_after", "team_b_score_after", "team_a_equip_value", "team_b_equip_value",
		"team_a_economy_type", "team_b_economy_type", "headline", "tags_json",
	}
	rows := make([][]string, 0, len(rounds))
	for _, r := range rounds {
		rows = append(rows, []string{
			itoa(r.RoundNumber), i64toa(r.StartTick), i64toa(r.FreezeEndTick), i64toa(r.EndTick),
			itoa(r.DurationSec), deref(r.WinnerTeamKey), deref(r.WinnerSide), deref(r.WinReason),
			deref(r.BombSite), r.TeamASide, r.TeamBSide,
			itoa(r.TeamAScoreBefore), itoa(r.TeamBScoreBefore),
			itoa(r.TeamAScoreAfter), itoa(r.TeamBScoreAfter),
			itoa(r.TeamAEquipValue), itoa(r.TeamBEquipValue),
			r.TeamAEconomyType, r.TeamBEconomyType, deref(r.Headline), r.TagsJSON,
		})
	}
	return header, rows
}

func eventsTable(events []storage.MatchEvent) ([]string, [][]string) {
	header := []string{
		"round_number", "tick", "round_time_sec", "event_type",
		"actor_steam_id64", "actor_name", "actor_team_key",
		"target_steam_id64", "target_name", "target_team_key",
		"assister_steam_id64", "assister_name",
		"weapon", "headshot", "wallbang", "through_smoke", "noscope", "attacker_blind",
	}
	rows := make([][]string, 0, len(events))
	for _, e := range events {
		rows = append(rows, []string{
			itoa(e.RoundNumber), i64toa(e.Tick), ftoa(e.RoundTimeSec), e.EventType,
			deref(e.ActorSteamID64), deref(e.ActorName), deref(e.ActorTeamKey),
			deref(e.TargetSteamID64), deref(e.TargetName), deref(e.TargetTeamKey),
			deref(e.AssisterSteamID), deref(e.AssisterName),
			deref(e.Weapon), boolStr(e.IsHeadshot), boolStr(e.IsWallbang),
			boolStr(e.IsThroughSmoke), boolStr(e.IsNoscope), boolStr(e.IsAttackerBlind),
		})
	}
	return header, rows
}

func sidesTable(sides []storage.MatchPlayerSide) ([]string, [][]string) {
	header := []string{
		"steam_id64", "side", "rounds_played", "kills", "deaths", "assists",
		"kd_ratio", "kpr", "dpr", "adr", "kast_percent", "hs_percent",
	}
	rows := make([][]string, 0, len(sides))
	for _, s := range sides {
		rows = append(rows, []string{
			s.SteamID64, s.Side, itoa(s.RoundsPlayed), itoa(s.Kills), itoa(s.Deaths),
			itoa(s.Assists), ftoa(s.KDRatio), ftoa(s.KPR), ftoa(s.DPR), ftoa(s.ADR),
			ftoa(s.KASTPercent), ftoa(s.HSPercent),
		})
	}
	return header, rows
}

func aggregateTable(agg PlayerAggregate) ([]string, [][]string) {
	header := []string{
		"steam_id64", "player_name", "matches", "rounds", "kills", "deaths", "assists",
		"kd_ratio", "kpr", "dpr", "adr", "kast_percent", "hs_percent", "rating",
	}
	rows := make([][]string, 0, len(agg.Rows)+1)
	rows = append(rows, []string{
		agg.SteamID64, agg.PlayerName, itoa(agg.Matches), itoa(agg.Rounds),
		itoa(agg.Kills), itoa(agg.Deaths), itoa(agg.Assists), ftoa(agg.KDRatio),
		ftoa(agg.KPR), ftoa(agg.DPR), ftoa(agg.ADR), ftoa(agg.KASTPercent),
		ftoa(agg.HSPercent), ftoa(agg.Rating),
	})
	for _, r := range agg.Rows {
		rows = append(rows, []string{
			agg.SteamID64, agg.PlayerName, "1", itoa(r.Rounds), itoa(r.Kills), itoa(r.Deaths),
			itoa(r.Assists), "", "", "", ftoa(r.ADR), ftoa(r.KASTPercent),
			ftoa(r.HSPercent), ftoa(r.Rating),
		})
	}
	return header, rows
}

// writeMatchXLSX writes one sheet per bundle table into a single workbook.
func writeMatchXLSX(path string, bundle MatchBundle) error {
	playersHeader, playersRows := playersTable(bundle.Players)
	roundsHeader, roundsRows := roundsTable(bundle.Rounds)
	eventsHeader, eventsRows := eventsTable(bundle.Events)
	sidesHeader, sidesRows := sidesTable(bundle.Sides)
	killsHeader, killsRows := killsTable(bundle.Kills)
	return writeXLSX(path, []xlsx.Sheet{
		{Name: "players", Header: playersHeader, Rows: xlsxRows(playersRows)},
		{Name: "rounds", Header: roundsHeader, Rows: xlsxRows(roundsRows)},
		{Name: "events", Header: eventsHeader, Rows: xlsxRows(eventsRows)},
		{Name: "sides", Header: sidesHeader, Rows: xlsxRows(sidesRows)},
		{Name: "kills", Header: killsHeader, Rows: xlsxRows(killsRows)},
	})
}

// writePlayerXLSX writes the aggregate row plus per-match rows.
func writePlayerXLSX(path string, agg PlayerAggregate) error {
	header, rows := aggregateTable(agg)
	return writeXLSX(path, []xlsx.Sheet{
		{Name: "matches", Header: header, Rows: xlsxRows(rows)},
	})
}

// xlsxRows converts CSV-style rows into typed cells, keeping numbers numeric.
func xlsxRows(rows [][]string) [][]xlsx.Cell {
	out := make([][]xlsx.Cell, 0, len(rows))
	for _, row := range rows {
		cells := make([]xlsx.Cell, 0, len(row))
		for _, v := range row {
			if v == "" {
				cells = append(cells, xlsx.Text(""))
				continue
			}
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				cells = append(cells, xlsx.Number(n))
				continue
			}
			cells = append(cells, xlsx.Text(v))
		}
		out = append(out, cells)
	}
	return out
}

func writeXLSX(path string, sheets []xlsx.Sheet) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	if err := xlsx.Write(f, sheets); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writePlayersCSV(path string, players []storage.MatchPlayer) error {
	header, rows := playersTable(players)
	return writeCSV(path, header, rows)
}

func writeRoundsCSV(path string, rounds []storage.MatchRound) error {
	header, rows := roundsTable(rounds)
	return writeCSV(path, header, rows)
}

func writeEventsCSV(path string, events []storage.MatchEvent) error {
	header, rows := eventsTable(events)
	return writeCSV(path, header, rows)
}

func writeSidesCSV(path string, sides []storage.MatchPlayerSide) error {
	header, rows := sidesTable(sides)
	return writeCSV(path, header, rows)
}

func writeAggregateCSV(path string, agg PlayerAggregate) error {
	header, rows := aggregateTable(agg)
	return writeCSV(path, header, rows)
}

// round2 formats a float with two decimals, which keeps CSV/XLSX readable.
func round2(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

func writeCSV(path string, header []string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if err := w.WriteAll(rows); err != nil {
		return fmt.Errorf("write rows: %w", err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return f.Close()
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func mustRounds(ctx context.Context, db *storage.DB, demoID int64) []storage.MatchRound {
	rounds, err := db.Analysis().ListMatchRounds(ctx, demoID)
	if err != nil {
		return nil
	}
	return rounds
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func itoa(v int) string { return strconv.Itoa(v) }

func i64toa(v int64) string { return strconv.FormatInt(v, 10) }

func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func ratioFloat(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
