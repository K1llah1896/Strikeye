package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func TestWriteMatchBundleHTMLIsSelfContainedAndEscaped(t *testing.T) {
	mapName := "de_dust2"
	date := "2026-11-20"
	bundle := MatchBundle{
		Demo: storage.Demo{
			ID: 7, TeamAName: "FURIA", TeamBName: "Falcons", TeamAScore: 16, TeamBScore: 14,
			MapName: &mapName, MatchDate: &date,
		},
		Players: []storage.MatchPlayer{
			{PlayerName: "KSCERATO", TeamKey: "team_a", Kills: 24, Deaths: 15, Assists: 3, ADR: 96.4, KASTPercent: 71, HSPercent: 55, StrikeyeRating: 1.31},
			{PlayerName: "<script>alert(1)</script>", TeamKey: "team_b", Kills: 9},
		},
	}
	dir := t.TempDir()
	written, err := WriteMatchBundle(bundle, dir, Formats{HTML: true})
	if err != nil {
		t.Fatalf("WriteMatchBundle: %v", err)
	}
	if len(written) != 1 || filepath.Ext(written[0]) != ".html" {
		t.Fatalf("written = %v", written)
	}
	raw, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, want := range []string{"<!doctype html>", "FURIA vs Falcons", "16 : 14", "de_dust2", "KSCERATO", "1.31", "<style>"} {
		if !strings.Contains(doc, want) {
			t.Errorf("html missing %q", want)
		}
	}
	if strings.Contains(doc, "<script>alert(1)</script>") {
		t.Error("player name was not HTML-escaped")
	}
	if !strings.Contains(doc, "&lt;script&gt;") {
		t.Error("escaped player name missing")
	}
	if strings.Contains(doc, "<link") || strings.Contains(doc, "<img") {
		t.Error("report must not reference external assets")
	}
}

func TestParseFormatsAcceptsHTML(t *testing.T) {
	f, err := ParseFormats("csv,html")
	if err != nil {
		t.Fatal(err)
	}
	if !f.HTML || !f.CSV {
		t.Fatalf("formats = %+v", f)
	}
	if _, err := ParseFormats("pdf"); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
