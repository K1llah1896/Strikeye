package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"strikeye/internal/export"
	"strikeye/internal/storage"
)

func TestRunExportBatchWritesIndexAndDirectories(t *testing.T) {
	dataRoot := t.TempDir()
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	seed := func(hash, mapName, teamA, teamB, status string) int64 {
		demo, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
			FilePath:      "/demos/" + hash + ".dem",
			Filename:      hash + ".dem",
			ContentSHA256: strings.Repeat(hash, 64),
			MapName:       &mapName,
			ParseStatus:   status,
			TotalRounds:   18,
			TeamAName:     teamA,
			TeamBName:     teamB,
			TeamAScore:    13,
			TeamBScore:    5,
		})
		if err != nil {
			t.Fatalf("seed %s: %v", hash, err)
		}
		if err := db.Analysis().ReplaceMatchPlayers(context.Background(), demo.ID, []storage.MatchPlayer{
			{SteamID64: "1", PlayerName: "p1", TeamKey: "team_a", Kills: 20, Deaths: 8, ADR: 96, KASTPercent: 74, StrikeyeRating: 1.35},
		}); err != nil {
			t.Fatalf("seed players %s: %v", hash, err)
		}
		return demo.ID
	}
	id1 := seed("a", "de_ancient", "FURIA", "Falcons", "parsed")
	id2 := seed("b", "de_dust2", "FURIA", "Falcons", "parsed")
	seed("c", "de_nuke", "NAVI", "G2", "parsed")
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	out := t.TempDir()
	var buf bytes.Buffer
	if err := runExportBatch(context.Background(), dataRoot, 0, "", out, "all",
		export.BatchFilter{Team: "furia"}, true, &buf); err != nil {
		t.Fatalf("runExportBatch: %v", err)
	}
	var res export.BatchResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v (%s)", err, buf.String())
	}
	if res.Exported != 2 || res.Failed != 0 {
		t.Fatalf("result = %+v", res)
	}
	for _, id := range []int64{id1, id2} {
		dir := filepath.Join(out, "demo_"+fmt.Sprint(id))
		for _, suffix := range []string{".json", "_players.csv", ".html", ".xlsx"} {
			if _, err := os.Stat(filepath.Join(dir, "match_"+fmt.Sprint(id)+suffix)); err != nil {
				t.Fatalf("demo %d missing %s: %v", id, suffix, err)
			}
		}
	}
	if _, err := os.Stat(res.Index); err != nil {
		t.Fatalf("index missing: %v", err)
	}

	// Combining --all with a single-match selector must be rejected.
	if err := runExportBatch(context.Background(), dataRoot, 1, "", out, "json",
		export.BatchFilter{}, true, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected conflict error for --all with --demo-id")
	}
}
