package ui

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
	"strikeye/internal/ui/views/analysis"
)

func TestVisualAuditOffscreenRender(t *testing.T) {
	theme.RegisterFonts()
	tmp := t.TempDir()
	db, err := storage.OpenDefault(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	mapName := "de_mirage"
	remark := "FACEIT 10 级复盘"
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:       "/demos/mirage_match.dem",
		Filename:       "faceit_mirage_2026.dem",
		FileSize:       1024000,
		ContentSHA256:  "abc123def456",
		SourcePlatform: "faceit",
		MapName:        &mapName,
		TickRate:       64,
		TotalRounds:    23,
		TeamAName:      "TEAM ALPHA",
		TeamBName:      "TEAM OMEGA",
		TeamAScore:     13,
		TeamBScore:     10,
		DurationSec:    2240,
		ParseStatus:    "ready",
		RenderStatus:   "idle",
		IsFavorite:     true,
		Remark:         &remark,
	})
	if err != nil {
		t.Fatal(err)
	}

	var players []storage.MatchPlayer
	for i := 0; i < 10; i++ {
		team := "CT"
		if i >= 5 {
			team = "T"
		}
		players = append(players, storage.MatchPlayer{
			DemoID:          demo.ID,
			SteamID64:       fmt.Sprintf("7656119800000000%d", i),
			PlayerName:      fmt.Sprintf("Player_%02d", i+1),
			DisplayName:     fmt.Sprintf("Player_%02d", i+1),
			TeamKey:         team,
			Kills:           24 - i,
			Deaths:          12 + i,
			Assists:         5,
			KDRatio:         float64(24-i) / float64(12+i),
			ADR:             96.5 - float64(i)*3.2,
			KASTPercent:     78 - float64(i)*2,
			HSPercent:       52 - float64(i)*2,
			FirstKills:      4 - (i % 3),
			FirstDeaths:     1 + (i % 3),
			TradeKills:      3,
			MultiKillRounds: 5 - (i % 4),
			ClutchWins:      2 - (i % 2),
			StrikeyeRating:  1.38 - float64(i)*0.05,
		})
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, players); err != nil {
		t.Fatal(err)
	}

	var rounds []storage.MatchRound
	for r := 1; r <= 23; r++ {
		win := "CT"
		reason := "elimination"
		if r%2 == 0 {
			win = "T"
			reason = "bomb_exploded"
		}
		rounds = append(rounds, storage.MatchRound{
			DemoID:           demo.ID,
			RoundNumber:      r,
			StartTick:        int64(r * 1000),
			FreezeEndTick:    int64(r*1000 + 200),
			EndTick:          int64(r*1000 + 800),
			WinnerTeamKey:    &win,
			WinReason:        &reason,
			TeamAEquipValue:  21500 - (r%4)*3500,
			TeamBEquipValue:  19800 - (r%3)*4200,
			TeamAEconomyType: "full_buy",
			TeamBEconomyType: "half_buy",
		})
	}
	if err := db.Analysis().ReplaceMatchRounds(ctx, demo.ID, rounds); err != nil {
		t.Fatal(err)
	}

	if err := db.Analysis().ReplaceHighlightClips(ctx, demo.ID, []storage.HighlightClip{
		{
			ID:              "hl-1",
			DemoID:          demo.ID,
			RoundNumber:     12,
			PlayerSteamID64: players[0].SteamID64,
			PlayerName:      players[0].PlayerName,
			ClipType:        "multi_kill",
			KillCount:       4,
			StartTick:       12200,
			EndTick:         12900,
			ImportanceScore: 95,
			TagsJSON:        `["4K","Headshot"]`,
		},
	}); err != nil {
		t.Fatal(err)
	}

	app := NewApp(AppOptions{
		DataRoot: tmp,
		DB:       db,
	})
	defer app.Close()

	auditDir := os.Getenv("STRIKEYE_AUDIT_DIR")
	savePNG := func(name string, view ViewID, modal bool) {
		app.NavigateTo(view, demo.ID)
		app.mu.Lock()
		app.showRenderModal = modal
		app.queuedCount = 2
		app.mu.Unlock()
		img := mygoui.Render(app.Render, 1360, 840, 1.0)
		if img == nil || img.Bounds().Dx() != 1360 || img.Bounds().Dy() != 840 {
			t.Fatalf("unexpected rendered image for %s", name)
		}
		if auditDir != "" {
			_ = os.MkdirAll(auditDir, 0o755)
			f, err := os.Create(filepath.Join(auditDir, name+".png"))
			if err == nil {
				_ = png.Encode(f, img)
				_ = f.Close()
			}
		}
	}

	savePNG("01_library", ViewLibrary, false)
	savePNG("02_spectator", ViewSpectator, false)
	savePNG("03_analysis", ViewAnalysis, false)
	savePNG("04_highlights", ViewHighlights, false)
	savePNG("05_settings", ViewSettings, false)
	savePNG("06_render_modal", ViewLibrary, true)

	// Also audit Analysis sub-tabs
	app.NavigateTo(ViewAnalysis, demo.ID)
	app.mu.Lock()
	app.showRenderModal = false
	app.mu.Unlock()
	for _, tab := range []struct {
		id   analysis.Tab
		name string
	}{
		{analysis.TabEconomy, "03b_economy"},
		{analysis.TabRadar, "03c_radar"},
		{analysis.TabReplay2D, "03d_replay2d"},
		{analysis.TabHeatmaps, "03e_heatmaps"},
	} {
		// Set activeTab via reflection-free helper? Let's just render analysis sub-tabs by exporting SetTab or testing directly.
		app.analysisView.SetActiveTab(tab.id)
		img := mygoui.Render(app.Render, 1360, 840, 1.0)
		if auditDir != "" {
			f, err := os.Create(filepath.Join(auditDir, tab.name+".png"))
			if err == nil {
				_ = png.Encode(f, img)
				_ = f.Close()
			}
		}
	}
}
