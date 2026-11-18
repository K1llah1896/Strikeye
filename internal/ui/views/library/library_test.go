package library

import (
	"testing"

	"strikeye/internal/storage"
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
				ParseStatus:    "ready",
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
