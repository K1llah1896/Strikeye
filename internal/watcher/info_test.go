package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSidecarTolerantVariants(t *testing.T) {
	meta, err := ParseSidecar([]byte(`{
		"Source": "Perfect World",
		"Team1Name": "Spirit",
		"team_2": "FaZe",
		"score1": "2",
		"score2": 1,
		"matchDate": "2026-10-01 12:00:00",
		"match_id": 987654
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if meta.SourcePlatform != PlatformPerfectWorld {
		t.Errorf("source = %q, want %q", meta.SourcePlatform, PlatformPerfectWorld)
	}
	if meta.TeamAName != "Spirit" || meta.TeamBName != "FaZe" {
		t.Errorf("teams = %q/%q", meta.TeamAName, meta.TeamBName)
	}
	if !meta.HasScore || meta.TeamAScore != 2 || meta.TeamBScore != 1 {
		t.Errorf("score = %v %d:%d", meta.HasScore, meta.TeamAScore, meta.TeamBScore)
	}
	if meta.MatchDate != "2026-10-01T12:00:00Z" {
		t.Errorf("date = %q", meta.MatchDate)
	}
	if meta.MatchID != "987654" {
		t.Errorf("match id = %q", meta.MatchID)
	}
}

func TestParseSidecarUnixAndUnknownFields(t *testing.T) {
	meta, err := ParseSidecar([]byte(`{"timestamp": 1767225600000, "unexpected": {"nested": true}, "platform": "faceit"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if meta.MatchDate != "2026-01-01T00:00:00Z" {
		t.Errorf("date = %q", meta.MatchDate)
	}
	if meta.SourcePlatform != PlatformFACEIT {
		t.Errorf("platform = %q", meta.SourcePlatform)
	}
}

func TestParseSidecarRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseSidecar([]byte(`{"team1":`)); err == nil {
		t.Fatalf("expected error for malformed json")
	}
}

func TestReadSidecarPathPreference(t *testing.T) {
	tmp := t.TempDir()
	demo := filepath.Join(tmp, "match.dem")
	if err := os.WriteFile(demo, demoPayload(2048), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, ok, err := ReadSidecar(demo)
	if err != nil || ok {
		t.Fatalf("expected no sidecar, got ok=%v err=%v", ok, err)
	}
	if meta.IsZero() != true {
		t.Fatalf("expected zero metadata")
	}

	if err := os.WriteFile(demo+".info", []byte(`{"source":"esl"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, ok, err = ReadSidecar(demo)
	if err != nil || !ok || meta.SourcePlatform != PlatformESL {
		t.Fatalf("demo.info not read: ok=%v err=%v meta=%+v", ok, err, meta)
	}

	// Extension-replaced sibling is used when "<demo>.info" is absent.
	demo2 := filepath.Join(tmp, "other.dem")
	if err := os.WriteFile(demo2, demoPayload(2048), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "other.info"), []byte(`{"date":"2026-02-03"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if meta, ok, err := ReadSidecar(demo2); err != nil || !ok || meta.MatchDate != "2026-02-03T00:00:00Z" {
		t.Fatalf("other.info not read: ok=%v err=%v meta=%+v", ok, err, meta)
	}
}

func TestNormalizeInfoDateFallsBackToRaw(t *testing.T) {
	if got := normalizeInfoDate("not a date"); got != "not a date" {
		t.Fatalf("expected raw fallback, got %q", got)
	}
	if got := normalizeInfoDate("2026-01-02"); got != "2026-01-02T00:00:00Z" {
		t.Fatalf("unexpected date: %q", got)
	}
	if got := normalizeInfoDate("1767225600"); got != time.Unix(1767225600, 0).UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected unix seconds: %q", got)
	}
}
