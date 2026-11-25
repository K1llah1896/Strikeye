package spectator

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/storage"
	"strikeye/internal/ui/renderutil"
)

// TestRenderSpectatorView seeds one parsed demo so the picker, round list and
// player list branches all lay out.
func TestRenderSpectatorView(t *testing.T) {
	root := t.TempDir()
	db, err := storage.OpenDefault(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	mapName := "de_dust2"
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/spec.dem", Filename: "spec.dem", ContentSHA256: "spec",
		TeamAName: "Falcons", TeamBName: "FURIA", MapName: &mapName, ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, []storage.MatchPlayer{
		{DemoID: demo.ID, SteamID64: "76561198000000001", PlayerName: "KSCERATO", TeamKey: "team_b"},
	}); err != nil {
		t.Fatalf("players: %v", err)
	}
	v := NewView(db, nil, "", nil)
	if err := v.SelectDemo(ctx, demo.ID); err != nil {
		t.Logf("select demo: %v", err)
	}
	renderutil.AssertRendered(t, 30, func(c *ui.Context) { v.Render(c) })
}
