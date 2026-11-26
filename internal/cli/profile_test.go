package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func TestRunProfileSaveAndInspect(t *testing.T) {
	dataRoot := t.TempDir()
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	demo, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
		FilePath:      "/demos/p.dem",
		Filename:      "p.dem",
		ContentSHA256: strings.Repeat("d", 64),
		ParseStatus:   "parsed",
		TotalRounds:   16,
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPlayers(context.Background(), demo.ID, []storage.MatchPlayer{{
		SteamID64: "76561198000000002", PlayerName: "ZywOo", Kills: 15, Deaths: 8, TotalDamage: 1440, ADR: 90,
	}}); err != nil {
		t.Fatalf("seed player: %v", err)
	}
	db.Close()

	var out bytes.Buffer
	if err := runProfile(context.Background(), dataRoot, "76561198000000002", "", "", "clutch,awp", &out); err != nil {
		t.Fatalf("save: %v", err)
	}
	var saved struct {
		Saved   bool                  `json:"saved"`
		Profile storage.PlayerProfile `json:"profile"`
	}
	if err := json.Unmarshal(out.Bytes(), &saved); err != nil || !saved.Saved || len(saved.Profile.Tags) != 2 {
		t.Fatalf("save output = %s err=%v", out.String(), err)
	}

	out.Reset()
	if err := runProfile(context.Background(), dataRoot, "76561198000000002", "", "", "", &out); err != nil {
		t.Fatalf("inspect: %v", err)
	}
	var report struct {
		Aggregate struct {
			Kills int     `json:"kills"`
			ADR   float64 `json:"adr"`
		} `json:"aggregate"`
		Profile    storage.PlayerProfile `json:"profile"`
		HasProfile bool                  `json:"has_profile"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v (%s)", err, out.String())
	}
	if !report.HasProfile || report.Aggregate.Kills != 15 || report.Aggregate.ADR != 90 {
		t.Errorf("report = %+v", report)
	}
	if err := runProfile(context.Background(), dataRoot, "nobody", "", "", "", &out); err == nil {
		t.Error("expected error for unknown steamid")
	}
}
