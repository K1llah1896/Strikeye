package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func TestRunSeriesGroupsSeededDemos(t *testing.T) {
	dataRoot := t.TempDir()
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	seed := func(hash, teamA, teamB, date string, aScore, bScore int) {
		_, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
			FilePath:      "/demos/" + hash + ".dem",
			Filename:      hash + ".dem",
			ContentSHA256: strings.Repeat(hash, 64),
			ParseStatus:   "parsed",
			TeamAName:     teamA,
			TeamBName:     teamB,
			MatchDate:     &date,
			TeamAScore:    aScore,
			TeamBScore:    bScore,
		})
		if err != nil {
			t.Fatalf("seed %s: %v", hash, err)
		}
	}
	seed("a", "FURIA", "Falcons", "2026-11-20", 13, 11)
	seed("b", "Falcons", "FURIA", "2026-11-20", 10, 13)
	seed("c", "NAVI", "G2", "2026-11-21", 2, 13)
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	var out bytes.Buffer
	if err := runSeries(context.Background(), dataRoot, 2, &out); err != nil {
		t.Fatalf("runSeries: %v", err)
	}
	var doc struct {
		Count  int `json:"count"`
		Series []struct {
			TeamA    string                   `json:"team_a"`
			TeamB    string                   `json:"team_b"`
			MapWinsA int                      `json:"map_wins_a"`
			MapWinsB int                      `json:"map_wins_b"`
			Maps     []struct{ DemoID int64 } `json:"maps"`
		} `json:"series"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v (%s)", err, out.String())
	}
	if doc.Count != 1 || len(doc.Series) != 1 {
		t.Fatalf("series = %+v", doc)
	}
	s := doc.Series[0]
	if s.TeamA != "Falcons" || s.MapWinsB != 2 || len(s.Maps) != 2 {
		t.Errorf("series content = %+v", s)
	}
}
