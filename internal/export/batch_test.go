package export

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func seedBatchDemo(t *testing.T, db *storage.DB, hash, mapName, teamA, teamB, status string) int64 {
	t.Helper()
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:      "/demos/" + hash + ".dem",
		Filename:      hash + ".dem",
		FileSize:      2048,
		ContentSHA256: strings.Repeat(hash[:1], 64),
		MapName:       strptr(mapName),
		ParseStatus:   status,
		TotalRounds:   20,
		TeamAName:     teamA,
		TeamBName:     teamB,
		TeamAScore:    13,
		TeamBScore:    7,
	})
	if err != nil {
		t.Fatalf("seed demo %s: %v", mapName, err)
	}
	players := []storage.MatchPlayer{
		{SteamID64: "1", PlayerName: "p1", TeamKey: "team_a", Kills: 18, Deaths: 10, ADR: 90, KASTPercent: 72, StrikeyeRating: 1.2},
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, players); err != nil {
		t.Fatalf("seed players: %v", err)
	}
	return demo.ID
}

func TestBatchExportsEachMatchIntoOwnDirectory(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	id1 := seedBatchDemo(t, db, "aaaa", "de_ancient", "FURIA", "Falcons", "parsed")
	id2 := seedBatchDemo(t, db, "bbbb", "de_dust2", "FURIA", "Falcons", "parsed")
	seedBatchDemo(t, db, "cccc", "de_nuke", "NAVI", "G2", "pending")

	out := t.TempDir()
	res, err := Batch(ctx, db, out, Formats{CSV: true, JSON: true}, BatchFilter{}, "both")
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if res.Exported != 2 || res.Failed != 0 {
		t.Fatalf("exported=%d failed=%d", res.Exported, res.Failed)
	}
	for _, id := range []int64{id1, id2} {
		dir := filepath.Join(out, "demo_"+itoa64(id))
		if _, err := os.Stat(filepath.Join(dir, "match_"+itoa64(id)+".json")); err != nil {
			t.Fatalf("demo %d json missing: %v", id, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "match_"+itoa64(id)+"_players.csv")); err != nil {
			t.Fatalf("demo %d csv missing: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "demo_3")); !os.IsNotExist(err) {
		t.Fatalf("pending match should be skipped, stat err = %v", err)
	}
	blob, err := os.ReadFile(res.Index)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var idx BatchResult
	if err := json.Unmarshal(blob, &idx); err != nil {
		t.Fatalf("decode index: %v", err)
	}
	if idx.Exported != 2 || len(idx.Items) != 2 || idx.Filter != "status=parsed" {
		t.Fatalf("index = %+v", idx)
	}
}

func TestBatchFiltersByTeamMapAndLimit(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	seedBatchDemo(t, db, "aaaa", "de_ancient", "FURIA", "Falcons", "parsed")
	seedBatchDemo(t, db, "bbbb", "de_dust2", "FURIA", "Falcons", "parsed")
	seedBatchDemo(t, db, "cccc", "de_nuke", "NAVI", "G2", "parsed")

	out := t.TempDir()
	res, err := Batch(ctx, db, out, Formats{JSON: true}, BatchFilter{Team: "navi"}, "json")
	if err != nil {
		t.Fatalf("batch team: %v", err)
	}
	if res.Exported != 1 || res.Items[0].Map != "de_nuke" {
		t.Fatalf("team filter = %+v", res.Items)
	}

	res, err = Batch(ctx, db, out, Formats{JSON: true}, BatchFilter{Map: "de_dust2"}, "json")
	if err != nil {
		t.Fatalf("batch map: %v", err)
	}
	if res.Exported != 1 || res.Items[0].Map != "de_dust2" {
		t.Fatalf("map filter = %+v", res.Items)
	}

	res, err = Batch(ctx, db, out, Formats{JSON: true}, BatchFilter{Limit: 1}, "json")
	if err != nil {
		t.Fatalf("batch limit: %v", err)
	}
	if res.Exported != 1 || len(res.Items) != 1 {
		t.Fatalf("limit filter = %+v", res.Items)
	}
	if res.Filter != "status=parsed,limit=1" {
		t.Fatalf("filter label = %q", res.Filter)
	}
}

func TestBatchReportsEmptySelection(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	seedBatchDemo(t, db, "aaaa", "de_ancient", "FURIA", "Falcons", "parsed")
	_, err = Batch(ctx, db, t.TempDir(), Formats{JSON: true}, BatchFilter{Team: "cloud9"}, "json")
	if err == nil || !strings.Contains(err.Error(), "no matches matched") {
		t.Fatalf("expected empty-selection error, got %v", err)
	}
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}
