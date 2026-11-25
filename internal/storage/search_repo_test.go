package storage

import (
	"context"
	"testing"
)

func seedSearchDemo(t *testing.T, db *DB, sha, teamA, teamB, mapName, remark string) Demo {
	t.Helper()
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, Demo{
		FilePath: "/demos/" + sha + ".dem", Filename: sha + ".dem", ContentSHA256: sha,
		TeamAName: teamA, TeamBName: teamB, MapName: &mapName, Remark: &remark,
		ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	return demo
}

func TestSearchIndexPrefixAndRanking(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	dust2 := seedSearchDemo(t, db, "s1", "FURIA", "Falcons", "de_dust2", "春季赛 半决赛")
	ancient := seedSearchDemo(t, db, "s2", "Spirit", "Vitality", "de_ancient", "")

	if err := db.Analysis().ReplaceMatchPlayers(ctx, dust2.ID, []MatchPlayer{
		{DemoID: dust2.ID, SteamID64: "1", PlayerName: "KSCERATO", TeamKey: "team_a"},
		{DemoID: dust2.ID, SteamID64: "2", PlayerName: "NiKo", TeamKey: "team_b"},
	}); err != nil {
		t.Fatalf("players: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, ancient.ID, []MatchPlayer{
		{DemoID: ancient.ID, SteamID64: "3", PlayerName: "donk", TeamKey: "team_a"},
	}); err != nil {
		t.Fatalf("players: %v", err)
	}
	if _, err := db.MapAnnotations().Add(ctx, MapAnnotation{
		DemoID: ancient.ID, RoundNumber: 4, CalloutID: "LongA", Note: "夹 A 点",
	}); err != nil {
		t.Fatalf("annotation: %v", err)
	}

	rebuilt, err := db.Search().RebuildSearchIndex(ctx)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if rebuilt != 2 {
		t.Fatalf("rebuilt %d docs, want 2", rebuilt)
	}
	changed, err := db.Search().EnsureSearchIndex(ctx)
	if err != nil || changed {
		t.Fatalf("index should already be fresh: changed=%v err=%v", changed, err)
	}

	hits, err := db.Search().Query(ctx, "ksce", 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(hits) != 1 || hits[0].DemoID != dust2.ID {
		t.Fatalf("hits = %+v", hits)
	}

	if hits, _ := db.Search().Query(ctx, "dust2", 10); len(hits) != 1 || hits[0].DemoID != dust2.ID {
		t.Fatalf("map query hits = %+v", hits)
	}
	if hits, _ := db.Search().Query(ctx, "春季", 10); len(hits) != 1 || hits[0].DemoID != dust2.ID {
		t.Fatalf("remark query hits = %+v", hits)
	}
	// Annotation notes are indexed too.
	if hits, _ := db.Search().Query(ctx, "longa", 10); len(hits) != 1 || hits[0].DemoID != ancient.ID {
		t.Fatalf("annotation query hits = %+v", hits)
	}
	// FTS operators typed by a user must not break the query.
	if hits, err := db.Search().Query(ctx, `NiKo" OR NEAR(`, 10); err != nil || len(hits) != 1 {
		t.Fatalf("operator query hits = %+v err=%v", hits, err)
	}
	if hits, _ := db.Search().Query(ctx, "   ", 10); len(hits) != 0 {
		t.Fatalf("blank query should return nothing, got %+v", hits)
	}

	// A new demo makes the index stale and EnsureSearchIndex repairs it.
	seedSearchDemo(t, db, "s3", "G2", "NAVI", "de_mirage", "")
	if changed, err := db.Search().EnsureSearchIndex(ctx); err != nil || !changed {
		t.Fatalf("expected a rebuild, changed=%v err=%v", changed, err)
	}
	if hits, _ := db.Search().Query(ctx, "mirage", 10); len(hits) != 1 {
		t.Fatalf("mirage hits = %+v", hits)
	}
}

func TestSearchQueryLimit(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	for i, sha := range []string{"a", "b", "c"} {
		seedSearchDemo(t, db, sha, "Team", "Opponent", "de_nuke", "note"+string(rune('0'+i)))
	}
	if _, err := db.Search().RebuildSearchIndex(ctx); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	hits, err := db.Search().Query(ctx, "nuke", 2)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("limit not honoured: %d hits", len(hits))
	}
}

// TestQueryFindsCJKSubstrings guards the tokenizer gap: unicode61 keeps a whole
// Chinese note as one token, so "双架" only matches through the substring pass.
func TestQueryFindsCJKSubstrings(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demo := seedSearchDemo(t, db, "cn", "Falcons", "FURIA", "de_dust2", "LongA 默认双架")
	if _, err := db.Search().RebuildSearchIndex(ctx); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	hits, err := db.Search().Query(ctx, "双架", 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(hits) != 1 || hits[0].DemoID != demo.ID {
		t.Fatalf("hits = %+v", hits)
	}
	// Latin terms still go through FTS only.
	if hits, err := db.Search().Query(ctx, "furia", 10); err != nil || len(hits) != 1 {
		t.Fatalf("latin hits = %+v err=%v", hits, err)
	}
	// A CJK string that is absent must not match.
	if hits, err := db.Search().Query(ctx, "上把", 10); err != nil || len(hits) != 0 {
		t.Fatalf("absent hits = %+v err=%v", hits, err)
	}
}
