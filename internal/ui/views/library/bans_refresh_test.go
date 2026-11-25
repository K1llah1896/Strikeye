package library

import (
	"context"
	"os"
	"testing"
	"time"

	"strikeye/internal/storage"
)

func TestSaveAndLoadSteamAPIKey(t *testing.T) {
	root := t.TempDir()
	if err := saveSteamAPIKey(root, "  ABC123  "); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(steamAPIKeyPath(root))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
	// The environment wins over the file so a key never has to touch disk.
	t.Setenv("STRIKEYE_STEAM_API_KEY", "ENVKEY")
	if got := loadSteamAPIKey(root); got != "ENVKEY" {
		t.Fatalf("env key not preferred: %q", got)
	}
	t.Setenv("STRIKEYE_STEAM_API_KEY", "")
	if got := loadSteamAPIKey(root); got != "ABC123" {
		t.Fatalf("file key = %q", got)
	}
	if err := saveSteamAPIKey(root, "   "); err == nil {
		t.Fatal("expected an error for an empty key")
	}
}

func TestRefreshBansWithoutPlayers(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := refreshBans(context.Background(), db, "", true); err == nil {
		t.Fatal("expected an error when the library has no players")
	}
	if _, err := refreshBans(context.Background(), nil, "", true); err == nil {
		t.Fatal("expected an error without storage")
	}
}

// TestRefreshBansUsesCachedRows seeds one player with a cached ban row and no API
// key, mirroring an offline refresh.
func TestRefreshBansUsesCachedRows(t *testing.T) {
	root := t.TempDir()
	db, err := storage.OpenDefault(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/d/x.dem", Filename: "x.dem", ContentSHA256: "x",
		TeamAName: "A", TeamBName: "B", ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	players := []storage.MatchPlayer{
		{DemoID: demo.ID, SteamID64: "76561198000000001", PlayerName: "clean", TeamKey: "team_a"},
		{DemoID: demo.ID, SteamID64: "76561198000000002", PlayerName: "banned", TeamKey: "team_b"},
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, players); err != nil {
		t.Fatalf("players: %v", err)
	}
	if err := db.PlayerBans().Upsert(ctx, storage.PlayerBans{
		SteamID64: "76561198000000002", VACBanned: true, NumberOfVACBans: 1,
		EconomyBan: "none", CheckedAt: time.Now().Add(-24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed ban: %v", err)
	}

	sum, err := refreshBans(ctx, db, "", false)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if sum.Checked != 2 {
		t.Fatalf("checked = %d", sum.Checked)
	}
	if len(sum.Banned) != 1 {
		t.Fatalf("banned = %v", sum.Banned)
	}
	// The refresh must not have written a key file on its own.
	if _, err := os.Stat(steamAPIKeyPath(root)); !os.IsNotExist(err) {
		t.Fatalf("key file should not exist: %v", err)
	}
}
