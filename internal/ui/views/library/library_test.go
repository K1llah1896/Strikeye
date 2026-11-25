package library

import (
	"context"
	"strings"
	"testing"
	"time"

	"strikeye/internal/storage"
	"strikeye/internal/watcher"
)

func strPtr(s string) *string { return &s }

func TestFilterMatches(t *testing.T) {
	entries := []MatchEntry{
		{
			Demo: storage.Demo{
				ID:             1,
				Filename:       "faceit_mirage.dem",
				MapName:        strPtr("de_mirage"),
				SourcePlatform: "faceit",
				ParseStatus:    "parsed",
				IsFavorite:     true,
				Remark:         strPtr("donk 30 bombs"),
			},
			PlayerNames: []string{"donk", "sh1ro"},
		},
		{
			Demo: storage.Demo{
				ID:             2,
				Filename:       "valve_inferno.dem",
				MapName:        strPtr("de_inferno"),
				SourcePlatform: "valve",
				ParseStatus:    "pending",
				IsFavorite:     false,
			},
			PlayerNames: []string{"zywoo"},
		},
	}

	got := FilterMatches(entries, FilterCriteria{Keyword: "donk"})
	if len(got) != 1 || got[0].Demo.ID != 1 {
		t.Fatalf("expected keyword 'donk' to match demo 1, got %+v", got)
	}
	got = FilterMatches(entries, FilterCriteria{Map: "de_inferno"})
	if len(got) != 1 || got[0].Demo.ID != 2 {
		t.Fatalf("expected map de_inferno to match demo 2, got %+v", got)
	}
	got = FilterMatches(entries, FilterCriteria{FavoritesOnly: true})
	if len(got) != 1 || got[0].Demo.ID != 1 {
		t.Fatalf("expected favorites filter to match demo 1")
	}
}

func TestPlatformFilterOptionsCoverRegistry(t *testing.T) {
	opts := platformFilterOptions()
	if opts[0] != allPlatformsOption {
		t.Fatalf("first option must be %q, got %q", allPlatformsOption, opts[0])
	}
	seen := make(map[string]bool, len(opts))
	for _, o := range opts {
		seen[o] = true
	}
	for _, p := range watcher.AllPlatforms() {
		if !seen[p.ID] {
			t.Errorf("platform %q missing from filter options", p.ID)
		}
	}
}

func TestPlatformMatchesNormalizesLegacyLabels(t *testing.T) {
	cases := []struct {
		stored   string
		selected string
		want     bool
	}{
		{"perfectworld", watcher.PlatformPerfectWorld, true},
		{"perfect_world", watcher.PlatformPerfectWorld, true},
		{"5eplay", watcher.Platform5E, true},
		{"FACEIT", watcher.PlatformFACEIT, true},
		{"faceit", watcher.PlatformESL, false},
		{"unknown", watcher.PlatformUnknown, true},
	}
	for _, tc := range cases {
		if got := platformMatches(tc.stored, tc.selected); got != tc.want {
			t.Errorf("platformMatches(%q,%q) = %v want %v", tc.stored, tc.selected, got, tc.want)
		}
	}
}

func TestGroupFilteredKeepsSeriesContiguous(t *testing.T) {
	entries := []MatchEntry{
		{Demo: storage.Demo{ID: 1, TeamAName: "FURIA", TeamBName: "Falcons", ParseStatus: "parsed", MatchDate: strPtr("2026-11-20"), MapName: strPtr("de_ancient"), TeamAScore: 13, TeamBScore: 11}},
		{Demo: storage.Demo{ID: 2, TeamAName: "NAVI", TeamBName: "G2", ParseStatus: "parsed", MatchDate: strPtr("2026-11-21"), MapName: strPtr("de_mirage"), TeamAScore: 13, TeamBScore: 7}},
		{Demo: storage.Demo{ID: 3, TeamAName: "Falcons", TeamBName: "FURIA", ParseStatus: "parsed", MatchDate: strPtr("2026-11-20"), MapName: strPtr("de_dust2"), TeamAScore: 9, TeamBScore: 13}},
	}
	ordered, byID, visible := groupFiltered(entries)
	if len(ordered) != 3 {
		t.Fatalf("ordered len = %d", len(ordered))
	}
	if ordered[0].Demo.ID != 1 || ordered[1].Demo.ID != 3 {
		t.Errorf("series maps not contiguous: %v, %v", ordered[0].Demo.ID, ordered[1].Demo.ID)
	}
	if ordered[2].Demo.ID != 2 {
		t.Errorf("standalone demo should be last, got %d", ordered[2].Demo.ID)
	}
	s, ok := byID[3]
	if !ok || s.MapWinsB != 2 || visible[s.Key] != 2 {
		t.Errorf("series lookup = %+v, visible=%v", s, visible)
	}
}

func TestFilterMatchesSeriesToggleIsUIOnly(t *testing.T) {
	entries := []MatchEntry{
		{Demo: storage.Demo{ID: 1, ParseStatus: "parsed"}},
		{Demo: storage.Demo{ID: 2, ParseStatus: "parsed"}},
	}
	got := FilterMatches(entries, FilterCriteria{GroupBySeries: true})
	if len(got) != 2 {
		t.Fatalf("toggle changed filtering: %d", len(got))
	}
}

func TestFilterMatchesPlayerTags(t *testing.T) {
	entries := []MatchEntry{
		{Demo: storage.Demo{ID: 1, ParseStatus: "parsed"}, PlayerNames: []string{"donk"}, PlayerTags: []string{"clutch", "anchor"}},
		{Demo: storage.Demo{ID: 2, ParseStatus: "parsed"}, PlayerNames: []string{"zywoo"}},
	}
	got := FilterMatches(entries, FilterCriteria{Keyword: "clutch"})
	if len(got) != 1 || got[0].Demo.ID != 1 {
		t.Fatalf("tag keyword match = %+v", got)
	}
}

func TestRefreshLoadsPlayerTags(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/t.dem", Filename: "t.dem", ContentSHA256: strings.Repeat("e", 64),
		ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, []storage.MatchPlayer{
		{SteamID64: "1", PlayerName: "a"},
		{SteamID64: "2", PlayerName: "b"},
	}); err != nil {
		t.Fatalf("seed players: %v", err)
	}
	if _, err := db.PlayerProfiles().Upsert(ctx, storage.PlayerProfile{SteamID64: "1", Tags: []string{"star"}}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	v := NewView(db, nil, t.TempDir(), nil)
	if err := v.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(v.entries) != 1 || len(v.entries[0].PlayerTags) != 1 || v.entries[0].PlayerTags[0] != "star" {
		t.Fatalf("entries = %+v", v.entries)
	}
}

func TestRefreshCountsBannedPlayers(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/t.dem", Filename: "t.dem", ContentSHA256: strings.Repeat("f", 64),
		ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, []storage.MatchPlayer{
		{SteamID64: "1", PlayerName: "a"},
		{SteamID64: "2", PlayerName: "b"},
		{SteamID64: "3", PlayerName: "c"},
	}); err != nil {
		t.Fatalf("seed players: %v", err)
	}
	if err := db.PlayerBans().Upsert(ctx, storage.PlayerBans{SteamID64: "1", VACBanned: true, NumberOfVACBans: 1}); err != nil {
		t.Fatalf("seed ban: %v", err)
	}
	// Community ban counts too; a clean snapshot must not.
	if err := db.PlayerBans().Upsert(ctx, storage.PlayerBans{SteamID64: "2", CommunityBanned: true}); err != nil {
		t.Fatalf("seed ban: %v", err)
	}
	if err := db.PlayerBans().Upsert(ctx, storage.PlayerBans{SteamID64: "3"}); err != nil {
		t.Fatalf("seed clean: %v", err)
	}

	v := NewView(db, nil, t.TempDir(), nil)
	if err := v.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(v.entries) != 1 || v.entries[0].BannedPlayers != 2 {
		t.Fatalf("entries = %+v", v.entries)
	}
}

func TestFilterMatchesFullTextIDs(t *testing.T) {
	entries := []MatchEntry{
		{Demo: storage.Demo{ID: 1, Filename: "a.dem", ParseStatus: "parsed"}},
		{Demo: storage.Demo{ID: 2, Filename: "b.dem", ParseStatus: "parsed"}},
	}
	// The keyword matches nothing in memory but the FTS index reports demo 2,
	// e.g. because an annotation note contains it.
	got := FilterMatches(entries, FilterCriteria{Keyword: "longa", ExtraDemoIDs: map[int64]bool{2: true}})
	if len(got) != 1 || got[0].Demo.ID != 2 {
		t.Fatalf("expected only the FTS hit, got %+v", got)
	}
	// Without the FTS hint the same keyword filters everything out.
	if got := FilterMatches(entries, FilterCriteria{Keyword: "longa"}); len(got) != 0 {
		t.Fatalf("expected no in-memory match, got %+v", got)
	}
}

func TestEnsureFullTextSearchPopulatesIDs(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	mapName := "de_dust2"
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/demos/x.dem", Filename: "x.dem", ContentSHA256: "x",
		TeamAName: "FURIA", TeamBName: "Falcons", MapName: &mapName, ParseStatus: "parsed",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := db.MapAnnotations().Add(ctx, storage.MapAnnotation{
		DemoID: demo.ID, RoundNumber: 1, CalloutID: "LongA", Note: "夹 A",
	}); err != nil {
		t.Fatalf("annotation: %v", err)
	}

	v := NewView(db, nil, t.TempDir(), nil)
	if err := v.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	v.filter.Keyword = "longa"
	deadline := time.Now().Add(2 * time.Second)
	for {
		got := v.FilteredEntries()
		if len(got) == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected the FTS hit to surface, got %d entries", len(got))
		}
		time.Sleep(20 * time.Millisecond)
	}
}
