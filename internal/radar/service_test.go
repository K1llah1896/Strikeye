package radar

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"strikeye/internal/config"
	"strikeye/internal/storage"
)

func TestServiceGenerateMatchCardsAndAppearanceSQLite(t *testing.T) {
	tmp := t.TempDir()
	layout, err := config.EnsureDataLayout(tmp)
	if err != nil {
		t.Fatalf("EnsureDataLayout: %v", err)
	}
	dbPath := filepath.Join(tmp, "strikeye.db")
	db, err := storage.Open(dbPath, layout.BackupDBSnapshots)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:      filepath.Join(tmp, "match.dem"),
		Filename:      "match.dem",
		ContentSHA256: "abc123",
		FileSize:      1024,
	})
	if err != nil {
		t.Fatalf("UpsertDiscovered: %v", err)
	}

	err = db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, []storage.MatchPlayer{
		{
			SteamID64:       "76561198000000001",
			PlayerName:      "PlayerOne",
			DisplayName:     "PlayerOne",
			TeamKey:         "CT",
			Kills:           22,
			Deaths:          14,
			Assists:         4,
			KPR:             0.91,
			ADR:             92.3,
			KASTPercent:     79.2,
			SurvivalRate:    45.8,
			MultiKillRounds: 6,
			StrikeyeRating:  1.31,
		},
		{
			SteamID64:       "76561198000000002",
			PlayerName:      "PlayerTwo",
			DisplayName:     "PlayerTwo",
			TeamKey:         "T",
			Kills:           15,
			Deaths:          18,
			Assists:         3,
			KPR:             0.62,
			ADR:             68.0,
			KASTPercent:     66.7,
			SurvivalRate:    25.0,
			MultiKillRounds: 2,
			StrikeyeRating:  0.88,
		},
	})
	if err != nil {
		t.Fatalf("ReplaceMatchPlayers: %v", err)
	}

	svc := NewService(db.Analysis(), layout)
	cards, err := svc.GenerateMatchCards(ctx, demo.ID)
	if err != nil {
		t.Fatalf("GenerateMatchCards: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}
	for _, c := range cards {
		if _, err := os.Stat(c.StaticImagePath); err != nil {
			t.Fatalf("missing static image %s: %v", c.StaticImagePath, err)
		}
	}

	portrait := filepath.Join(tmp, "p1.png")
	writeSolidPNG(t, portrait, 64, 64, color.RGBA{10, 200, 100, 255})
	updated, err := svc.UpdateCardAppearance(ctx, demo.ID, "76561198000000001", &portrait, nil, RenderAnimOptions{})
	if err != nil {
		t.Fatalf("UpdateCardAppearance: %v", err)
	}
	if updated.CustomPortraitPath == nil || *updated.CustomPortraitPath != portrait {
		t.Fatalf("expected updated CustomPortraitPath=%s, got %v", portrait, updated.CustomPortraitPath)
	}
}
