package library

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/gsi"
	"strikeye/internal/storage"
	"strikeye/internal/ui/renderutil"
)

// seedRenderLibrary seeds one parsed demo so the cards, filters and series
// headers all have data to lay out.
func seedRenderLibrary(t *testing.T) (*storage.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := storage.OpenDefault(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	for i, sha := range []string{"aa", "bb"} {
		mapName := "de_ancient"
		if i == 1 {
			mapName = "de_dust2"
		}
		remark := "默认双架"
		demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
			FilePath: "/demos/" + sha + ".dem", Filename: sha + ".dem", ContentSHA256: sha,
			TeamAName: "Falcons", TeamBName: "FURIA", MapName: &mapName, Remark: &remark,
			ParseStatus: "parsed", MatchDate: strPtr("2026-11-20"),
		})
		if err != nil {
			t.Fatalf("seed demo: %v", err)
		}
		if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, []storage.MatchPlayer{
			{DemoID: demo.ID, SteamID64: "76561198000000001", PlayerName: "KSCERATO", TeamKey: "team_b"},
		}); err != nil {
			t.Fatalf("players: %v", err)
		}
	}
	return db, root
}

func TestRenderLibraryDefault(t *testing.T) {
	db, root := seedRenderLibrary(t)
	v := NewView(db, nil, root, nil)
	if err := v.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	renderutil.AssertRendered(t, 60, func(c *ui.Context) { v.Render(c) })
}

// TestRenderLibraryPanels renders the download and API-key panels, which only
// appear after a click and would otherwise never be laid out in tests.
func TestRenderLibraryPanels(t *testing.T) {
	db, root := seedRenderLibrary(t)
	v := NewView(db, nil, root, nil)
	if err := v.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	v.showSteamDownload = true
	v.showAPIKeyPrompt = true
	v.steamCookieDraft = "steamLoginSecure=abc"
	v.sharecodeDraft = "CSGO-L9spZ-ihuov-cyhtE-kxbqa-FkBAA"
	v.apiKeyDraft = "0123456789ABCDEF"
	v.statusMsg = "示例状态"
	renderutil.AssertRendered(t, 60, func(c *ui.Context) { v.Render(c) })
}

// TestRenderLiveBanner renders a fresh GSI state so the banner branch is
// exercised (map/round/score/bomb/alive).
func TestRenderLiveBanner(t *testing.T) {
	db, root := seedRenderLibrary(t)
	v := NewView(db, nil, root, nil)
	if err := v.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	v.live = gsi.State{
		Map: "de_dust2", Round: 3, TeamScoreCT: 2, TeamScoreT: 1, RoundPhase: "live",
		Bomb: "planted", ReceivedAt: time.Now(),
		Players: []gsi.Player{{Health: 100}, {Health: 0}},
	}
	v.liveOK = true
	v.liveChecked = time.Now()
	renderutil.AssertRendered(t, 60, func(c *ui.Context) { v.Render(c) })
}
