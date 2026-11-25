package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"strikeye/internal/tendency"
)

func TestWriteTendencyCSVFlattensSections(t *testing.T) {
	rep := tendency.Report{
		Team:   "FURIA",
		Rounds: 4,
		Sides: []tendency.SideProfile{
			{Map: "de_dust2", Side: "T", Rounds: 4, Wins: 3, WinPct: 75, Pistols: 2, PistolWins: 1, AvgEquip: 12000, UtilityPerRound: 2.5},
		},
		Economy: []tendency.EconomyStat{{Side: "T", Type: "full", Rounds: 3, Wins: 3, WinPct: 100}},
		Sites:   []tendency.SiteStat{{Map: "de_dust2", Site: "B", Plants: 2, Wins: 1, WinPct: 50}},
		Opening: []tendency.OpeningStat{{Callout: "Long A", Kills: 2, Share: 50}},
	}
	var buf bytes.Buffer
	if err := writeTendencyCSV(&buf, rep); err != nil {
		t.Fatalf("csv: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected header + 4 rows, got %d:\n%s", len(lines), buf.String())
	}
	if lines[0] != "kind,map,side,name,rounds,wins,win_pct,extra" {
		t.Fatalf("header = %q", lines[0])
	}
	if !strings.Contains(lines[1], "side,de_dust2,T,T,4,3,75.00") {
		t.Fatalf("side row = %q", lines[1])
	}
	if !strings.Contains(lines[1], "utility_per_round=2.50") || !strings.Contains(lines[1], "pistols=1/2") {
		t.Fatalf("side extras = %q", lines[1])
	}
	if lines[2] != "economy,,T,full,3,3,100.00," {
		t.Fatalf("economy row = %q", lines[2])
	}
	if lines[3] != "site,de_dust2,,B,2,1,50.00," {
		t.Fatalf("site row = %q", lines[3])
	}
	if lines[4] != "opening,,,Long A,2,0,50.00," {
		t.Fatalf("opening row = %q", lines[4])
	}
}

func TestRunTendenciesRequiresTeam(t *testing.T) {
	var out bytes.Buffer
	if err := runTendencies(context.Background(), t.TempDir(), []string{"-format", "json"}, &out); err == nil {
		t.Fatal("expected an error when --team is missing")
	}
}

func TestRunHeadToHeadRequiresBothTeams(t *testing.T) {
	var out bytes.Buffer
	if err := runHeadToHead(context.Background(), t.TempDir(), []string{"-team-a", "FURIA"}, &out); err == nil {
		t.Fatal("expected an error when --team-b is missing")
	}
}

func TestRunTrendsRequiresSteamID(t *testing.T) {
	var out bytes.Buffer
	if err := runTrends(context.Background(), t.TempDir(), []string{"-format", "csv"}, &out); err == nil {
		t.Fatal("expected an error when --steamid is missing")
	}
}
