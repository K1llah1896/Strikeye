package export

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func seedMatch(t *testing.T, db *storage.DB) int64 {
	t.Helper()
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:      "/demos/furia.dem",
		Filename:      "furia.dem",
		FileSize:      1024,
		ContentSHA256: strings.Repeat("a", 64),
		MapName:       strptr("de_ancient"),
		MatchDate:     strptr("2026-11-20"),
		ParseStatus:   "parsed",
		TotalRounds:   24,
		TeamAName:     "FURIA",
		TeamBName:     "Falcons",
		TeamAScore:    13,
		TeamBScore:    11,
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	players := []storage.MatchPlayer{
		{SteamID64: "1", PlayerName: "fallen", TeamKey: "team_a", Kills: 20, Deaths: 14, Assists: 5,
			TotalDamage: 2400, ADR: 100, KASTPercent: 75, Headshots: 8, HSPercent: 40,
			StrikeyeRating: 1.30, EcoAdjRating: 1.25, EcoAdjADR: 98, RoundSwing: 1.1, FlashAssists: 2,
			ClutchWins: 1, ClutchAttempts: 2, MultiKillRounds: 3, AWPKills: 4, ImpactScore: 1.2},
		{SteamID64: "2", PlayerName: "kye", TeamKey: "team_b", Kills: 12, Deaths: 18, Assists: 3,
			TotalDamage: 1500, ADR: 62.5, KASTPercent: 60, Headshots: 6, HSPercent: 50,
			StrikeyeRating: 0.85},
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, players); err != nil {
		t.Fatalf("seed players: %v", err)
	}
	sides := []storage.MatchPlayerSide{
		{SteamID64: "1", Side: "ct", RoundsPlayed: 12, Kills: 12, Deaths: 6, ADR: 110, KASTPercent: 80},
		{SteamID64: "1", Side: "t", RoundsPlayed: 12, Kills: 8, Deaths: 8, ADR: 90, KASTPercent: 70},
	}
	if err := db.Analysis().ReplaceMatchPlayerSides(ctx, demo.ID, sides); err != nil {
		t.Fatalf("seed sides: %v", err)
	}
	rounds := []storage.MatchRound{
		{RoundNumber: 1, StartTick: 100, EndTick: 5000, DurationSec: 115, TeamASide: "t", TeamBSide: "ct",
			TeamAScoreAfter: 1, WinnerTeamKey: strptr("team_a"), WinReason: strptr("elimination"), BombSite: strptr("a")},
		{RoundNumber: 2, StartTick: 5000, EndTick: 9000, DurationSec: 100, TeamASide: "t", TeamBSide: "ct",
			TeamAScoreAfter: 1, TeamBScoreAfter: 1, WinnerTeamKey: strptr("team_b"), WinReason: strptr("bomb")},
	}
	if err := db.Analysis().ReplaceMatchRounds(ctx, demo.ID, rounds); err != nil {
		t.Fatalf("seed rounds: %v", err)
	}
	events := []storage.MatchEvent{
		{RoundNumber: 1, Tick: 2000, RoundTimeSec: 30, EventType: "kill",
			ActorSteamID64: strptr("1"), ActorName: strptr("fallen"), TargetSteamID64: strptr("2"),
			TargetName: strptr("kye"), Weapon: strptr("ak47"), IsHeadshot: true},
	}
	if err := db.Analysis().ReplaceMatchEvents(ctx, demo.ID, events); err != nil {
		t.Fatalf("seed events: %v", err)
	}
	return demo.ID
}

func TestWriteMatchBundleCSVAndJSON(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demoID := seedMatch(t, db)

	bundle, err := LoadMatchBundle(ctx, db, demoID)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	dir := t.TempDir()
	files, err := WriteMatchBundle(bundle, dir, Formats{CSV: true, JSON: true})
	if err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	if len(files) != 6 {
		t.Fatalf("expected 6 files, got %d: %v", len(files), files)
	}
	for _, want := range []string{"_players.csv", "_rounds.csv", "_events.csv", "_sides.csv", "_kills.csv", ".json"} {
		found := false
		for _, f := range files {
			if strings.HasSuffix(f, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing output %s in %v", want, files)
		}
	}

	playersCSV := filepath.Join(dir, "match_1_players.csv")
	rows := readCSV(t, playersCSV)
	if len(rows) != 3 {
		t.Fatalf("players rows = %d, want header+2", len(rows))
	}
	if rows[1][0] != "1" || rows[1][1] != "fallen" {
		t.Errorf("first player row = %v", rows[1])
	}

	eventsCSV := filepath.Join(dir, "match_1_events.csv")
	erows := readCSV(t, eventsCSV)
	if len(erows) != 2 || erows[1][3] != "kill" || erows[1][13] != "true" {
		t.Errorf("event rows = %v", erows)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "match_1.json"))
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var doc struct {
		SchemaVersion int                       `json:"schema_version"`
		Demo          struct{ ID int64 }        `json:"demo"`
		Players       []storage.MatchPlayer     `json:"players"`
		Rounds        []storage.MatchRound      `json:"rounds"`
		Events        []storage.MatchEvent      `json:"events"`
		Sides         []storage.MatchPlayerSide `json:"sides"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if doc.SchemaVersion != SchemaVersion || doc.Demo.ID != demoID {
		t.Errorf("json meta = %+v", doc)
	}
	if len(doc.Players) != 2 || len(doc.Rounds) != 2 || len(doc.Events) != 1 || len(doc.Sides) != 2 {
		t.Errorf("json counts players=%d rounds=%d events=%d sides=%d",
			len(doc.Players), len(doc.Rounds), len(doc.Events), len(doc.Sides))
	}
}

func TestAggregatePlayerWeightedAndCSV(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	seedMatch(t, db)

	agg, err := AggregatePlayer(ctx, db, "1")
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if agg.Matches != 1 || agg.Rounds != 24 || agg.Kills != 20 || agg.PlayerName != "fallen" {
		t.Errorf("aggregate totals = %+v", agg)
	}
	if agg.ADR < 99 || agg.ADR > 101 {
		t.Errorf("aggregate ADR = %v, want 100", agg.ADR)
	}
	if agg.Rating < 1.29 || agg.Rating > 1.31 {
		t.Errorf("aggregate rating = %v, want 1.3", agg.Rating)
	}
	if agg.HSPercent < 39.9 || agg.HSPercent > 40.1 {
		t.Errorf("aggregate hs%% = %v, want 40", agg.HSPercent)
	}

	dir := t.TempDir()
	files, err := WritePlayerAggregate(agg, dir, Formats{CSV: true, JSON: true})
	if err != nil {
		t.Fatalf("write aggregate: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("aggregate files = %v", files)
	}
	rows := readCSV(t, filepath.Join(dir, "player_1_matches.csv"))
	if len(rows) != 3 {
		t.Fatalf("aggregate csv rows = %d, want header+summary+1", len(rows))
	}
	if rows[1][2] != "1" {
		t.Errorf("summary row = %v", rows[1])
	}

	if _, err := AggregatePlayer(ctx, db, "missing"); err == nil {
		t.Fatal("expected error for unknown steamid")
	}
}

func TestParseFormats(t *testing.T) {
	cases := map[string]Formats{
		"":         {CSV: true, JSON: true},
		"both":     {CSV: true, JSON: true},
		"csv":      {CSV: true},
		"JSON":     {JSON: true},
		"csv,json": {CSV: true, JSON: true},
		"all":      {CSV: true, JSON: true, HTML: true, XLSX: true},
		"csv,xlsx": {CSV: true, XLSX: true},
	}
	for raw, want := range cases {
		got, err := ParseFormats(raw)
		if err != nil || got != want {
			t.Errorf("ParseFormats(%q) = %+v, %v; want %+v", raw, got, err, want)
		}
	}
	if _, err := ParseFormats("pdf"); err == nil {
		t.Error("expected error for pdf")
	}
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return rows
}

func strptr(s string) *string { return &s }

func xlsxTestBundle() MatchBundle {
	name := "KSCERATO"
	site := "A"
	return MatchBundle{
		Demo:    storage.Demo{ID: 42, MapName: strptr("de_dust2"), ParseStatus: "parsed", TickRate: 64},
		Players: []storage.MatchPlayer{{SteamID64: "76561198000000001", PlayerName: name, TeamKey: "team_a", Kills: 20, ADR: 88.5}},
		Rounds:  []storage.MatchRound{{RoundNumber: 1, BombSite: &site, TeamASide: "t", TeamBSide: "ct"}},
		Events:  []storage.MatchEvent{{RoundNumber: 1, Tick: 100, EventType: "kill", IsHeadshot: true}},
		Sides:   []storage.MatchPlayerSide{{SteamID64: "76561198000000001", Side: "t", RoundsPlayed: 15, ADR: 90.1}},
	}
}

func TestWriteMatchBundleXLSX(t *testing.T) {
	bundle := xlsxTestBundle()
	dir := t.TempDir()
	files, err := WriteMatchBundle(bundle, dir, Formats{XLSX: true})
	if err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	if len(files) != 1 || !strings.HasSuffix(files[0], ".xlsx") {
		t.Fatalf("files = %v", files)
	}
	zr, err := zip.OpenReader(files[0])
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer zr.Close()
	seen := map[string]bool{}
	for _, f := range zr.File {
		seen[f.Name] = true
	}
	for _, want := range []string{
		"xl/workbook.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml",
		"xl/worksheets/sheet3.xml", "xl/worksheets/sheet4.xml", "xl/styles.xml",
	} {
		if !seen[want] {
			t.Fatalf("missing part %s (have %v)", want, seen)
		}
	}
}

func TestWritePlayerAggregateXLSX(t *testing.T) {
	agg := PlayerAggregate{SteamID64: "76561198000000001", PlayerName: "KSCERATO", Matches: 1, Rounds: 30, Kills: 20, Rating: 1.1}
	dir := t.TempDir()
	files, err := WritePlayerAggregate(agg, dir, Formats{XLSX: true})
	if err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	if len(files) != 1 || !strings.HasSuffix(files[0], "_matches.xlsx") {
		t.Fatalf("files = %v", files)
	}
}

func TestParseFormatsXLSX(t *testing.T) {
	f, err := ParseFormats("xlsx")
	if err != nil || !f.XLSX {
		t.Fatalf("formats = %+v err = %v", f, err)
	}
	f, err = ParseFormats("csv,xlsx")
	if err != nil || !f.CSV || !f.XLSX || f.JSON {
		t.Fatalf("formats = %+v err = %v", f, err)
	}
	if _, err := ParseFormats("xlsx,pdf"); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

// TestKillContextExportRoundTrip stores kill geometry rows and checks they reach
// the CSV and JSON bundle.
func TestKillContextExportRoundTrip(t *testing.T) {
	root := t.TempDir()
	db, err := storage.OpenDefault(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demoID := seedMatch(t, db)
	rows := []storage.KillContext{
		{DemoID: demoID, RoundNumber: 1, Tick: 6000, TimeSec: 12.5, KillerSlot: 4, VictimSlot: 8,
			KillerName: "kscerato", VictimName: "niko", Weapon: "ak47", Headshot: true,
			Distance: 812.345, Visible: true, GeometryChecked: true, SmokeBlocked: true,
			OpticsChecked: true, AimOffDeg: 2.5, InFOV: true, AttackerZoom: 1, VictimZoom: 0,
			KillerX: 1200, KillerY: -300, KillerZ: 64, VictimX: 400, VictimY: -900, VictimZ: 60},
	}
	if err := db.KillContext().ReplaceMatchKillContext(ctx, demoID, rows); err != nil {
		t.Fatalf("store: %v", err)
	}
	bundle, err := LoadMatchBundle(ctx, db, demoID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(bundle.Kills) != 1 || bundle.Kills[0].Weapon != "ak47" {
		t.Fatalf("bundle kills = %+v", bundle.Kills)
	}
	dir := t.TempDir()
	if _, err := WriteMatchBundle(bundle, dir, Formats{CSV: true, JSON: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	kills := readCSV(t, filepath.Join(dir, fmt.Sprintf("match_%d_kills.csv", demoID)))
	if len(kills) != 2 {
		t.Fatalf("kills csv rows = %d", len(kills))
	}
	if kills[1][5] != "ak47" || kills[1][7] != "812.35" {
		t.Fatalf("kills row = %v", kills[1])
	}
	raw, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("match_%d.json", demoID)))
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	if !strings.Contains(string(raw), `"kills"`) || !strings.Contains(string(raw), `"ak47"`) {
		t.Fatal("json bundle missing kills")
	}
	totals, err := db.KillContext().Totals(ctx, demoID)
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if totals.Kills != 1 || totals.SmokePct != 100 {
		t.Fatalf("totals = %+v", totals)
	}
}
