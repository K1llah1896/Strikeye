package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"strikeye/internal/storage"
)

func TestParseAndPersistCorruptDemoMarksFailed(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	dataRoot := filepath.Join(tmp, "data")
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	badDemo := filepath.Join(tmp, "corrupt.dem")
	buf := make([]byte, 4096)
	copy(buf[:8], "PBDEMS2\x00")
	copy(buf[8:], "corrupt_stream_payload")
	if err := os.WriteFile(badDemo, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = ParseAndPersist(ctx, db, ParseOptions{
		DataRoot: dataRoot,
		FilePath: badDemo,
	})
	if err == nil {
		t.Fatalf("expected error parsing corrupt demo stream")
	}

	demos, listErr := db.Demos().List(ctx)
	if listErr != nil || len(demos) != 1 {
		t.Fatalf("expected 1 demo record, got %d (err=%v)", len(demos), listErr)
	}
	if demos[0].ParseStatus != "failed" || demos[0].ParseError == nil || *demos[0].ParseError == "" {
		t.Fatalf("expected parse_status=failed with non-empty parse_error, got status=%s err=%v", demos[0].ParseStatus, demos[0].ParseError)
	}

	players, _ := db.Analysis().ListMatchPlayers(ctx, demos[0].ID)
	rounds, _ := db.Analysis().ListMatchRounds(ctx, demos[0].ID)
	if len(players) != 0 || len(rounds) != 0 {
		t.Fatalf("expected zero partial rows on failed parse, got players=%d rounds=%d", len(players), len(rounds))
	}
}

func TestParseAndPersistReusesParsedCache(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	dataRoot := filepath.Join(tmp, "data")
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	rplPath := filepath.Join(dataRoot, "cache", "replay_32hz", "deadbeef.rpl")
	if err := WriteReplayFileAtomic(rplPath, 64.0, InputCapNone, []ReplayRoundData{{RoundNumber: 1, StartTick: 10, EndTick: 20}}); err != nil {
		t.Fatal(err)
	}

	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:      filepath.Join(tmp, "nonexistent_raw.dem"),
		Filename:      "nonexistent_raw.dem",
		FileSize:      2048,
		ContentSHA256: "deadbeef",
	})
	if err != nil {
		t.Fatal(err)
	}
	demo.ParseStatus = "parsed"
	demo.ReplayCachePath = &rplPath
	demo.InputDataCapability = InputCapNone
	if err := db.Demos().UpdateParseResult(ctx, demo); err != nil {
		t.Fatal(err)
	}

	sum, err := ParseAndPersist(ctx, db, ParseOptions{
		DataRoot: dataRoot,
		DemoID:   demo.ID,
	})
	if err != nil {
		t.Fatalf("expected cached reuse without opening raw demo, got: %v", err)
	}
	if !sum.ReusedCache {
		t.Fatalf("expected ReusedCache=true")
	}
}
