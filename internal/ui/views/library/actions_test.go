package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func seedExportDemo(t *testing.T, db *storage.DB, ctx context.Context) storage.Demo {
	t.Helper()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/export.dem", Filename: "export.dem", ContentSHA256: "export-demo",
		MapName: stringPtr("de_dust2"), TickRate: 64, TotalRounds: 1,
		TeamAName: "FURIA", TeamBName: "Falcons", TeamAScore: 1, TeamBScore: 0,
		ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	players := []storage.MatchPlayer{{DemoID: demo.ID, SteamID64: "1", PlayerName: "KSCERATO", TeamKey: "team_a", Kills: 1}}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, players); err != nil {
		t.Fatalf("players: %v", err)
	}
	rounds := []storage.MatchRound{{DemoID: demo.ID, RoundNumber: 1, StartTick: 1, FreezeEndTick: 2, EndTick: 3000, WinnerSide: stringPtr("CT")}}
	if err := db.Analysis().ReplaceMatchRounds(ctx, demo.ID, rounds); err != nil {
		t.Fatalf("rounds: %v", err)
	}
	return demo
}

func stringPtr(s string) *string { return &s }

func TestExportDemoWritesEveryFormat(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demo := seedExportDemo(t, db, ctx)
	dataRoot := t.TempDir()

	dir, count, err := exportDemo(ctx, db, dataRoot, demo.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if count < 4 {
		t.Fatalf("expected at least 4 files, got %d", count)
	}
	for _, ext := range []string{".csv", ".json", ".html", ".xlsx"} {
		matches, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
		if len(matches) == 0 {
			t.Fatalf("missing %s in %s", ext, dir)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) < 4 {
		t.Fatalf("dir entries = %d", len(entries))
	}
	if !strings.HasPrefix(dir, dataRoot) {
		t.Fatalf("export dir %q should live under the DataRoot", dir)
	}
}

func TestExportDemoWithoutDB(t *testing.T) {
	if _, _, err := exportDemo(context.Background(), nil, t.TempDir(), 1); err == nil {
		t.Fatal("expected an error without storage")
	}
}

func TestOpenDemoInCS2BuildsSteamURL(t *testing.T) {
	var got string
	demo := storage.Demo{FilePath: `/home/u/demos/a b.dem`}
	if err := openDemoInCS2(demo, func(url string) error { got = url; return nil }); err != nil {
		t.Fatalf("open: %v", err)
	}
	if !strings.HasPrefix(got, "steam://rungameid/730//+playdemo%20") {
		t.Fatalf("url = %q", got)
	}
	if !strings.Contains(got, "a%20b.dem") {
		t.Fatalf("path not escaped: %q", got)
	}

	// A demo row without a path must not reach the handler.
	if err := openDemoInCS2(storage.Demo{}, func(string) error {
		t.Fatal("handler should not be called")
		return nil
	}); err == nil {
		t.Fatal("expected an error for an empty path")
	}

	sentinel := errors.New("boom")
	if err := openDemoInCS2(demo, func(string) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("handler error not propagated: %v", err)
	}
}
