package corpus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiffDetectsMetricAndCountChanges(t *testing.T) {
	want := Snapshot{
		Source: "x.dem", SHA256: "abc", Map: "de_dust2", TeamA: "A", TeamB: "B",
		ScoreA: 13, ScoreB: 7, TickRate: 64, ParseStatus: "parsed",
		RoundCount: 20, EventCount: 10, Highlights: 3, InputCap: "full_usercmd",
		ReplayRounds: 20, ReplayShots: 100, ReplayDeaths: 40, Purchases: 500,
		Players: []PlayerEntry{{Name: "p1", Kills: 20, Deaths: 15, ADR: 90.5, Rating: 1.2,
			TTDSamples: 3, TTDAvgMS: 400}},
	}
	// identical snapshot has no diffs
	if got := Diff(want, want); len(got) != 0 {
		t.Fatalf("expected no diffs, got %v", got)
	}
	got := want
	got.ScoreB = 8
	got.ReplayShots = 101
	got.Players = []PlayerEntry{{Name: "p1", Kills: 21, Deaths: 15, ADR: 91.0, Rating: 1.2,
		TTDSamples: 3, TTDAvgMS: 400}}
	diffs := Diff(want, got)
	if len(diffs) < 3 {
		t.Fatalf("expected at least 3 diffs, got %v", diffs)
	}
	// floating point noise below tolerance must not fail
	want.Players[0].ADR = 90.5
	got2 := want
	got2.Players = []PlayerEntry{want.Players[0]}
	got2.Players[0].ADR = 90.56
	if diffs := Diff(want, got2); len(diffs) != 0 {
		t.Fatalf("tolerance not applied: %v", diffs)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "snap.json")
	snap := Snapshot{Source: "a.dem", Map: "de_mirage", Players: []PlayerEntry{{Name: "p", Kills: 1}}}
	if err := WriteSnapshot(path, snap); err != nil {
		t.Fatalf("write: %v", err)
	}
	back, err := ReadSnapshot(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if back.Map != snap.Map || len(back.Players) != 1 || back.Players[0].Kills != 1 {
		t.Fatalf("round trip mismatch: %+v", back)
	}
	if _, err := ReadSnapshot(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

// TestFixtureCorpus is opt-in because parsing two 350MB demos takes ~30s:
// run with STRIKEYE_CORPUS=1 go test ./internal/corpus/ -run TestFixtureCorpus.
func TestFixtureCorpus(t *testing.T) {
	if os.Getenv("STRIKEYE_CORPUS") == "" {
		t.Skip("set STRIKEYE_CORPUS=1 to verify fixture golden metrics")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	goldenDir := filepath.Join(root, "tests", "fixtures", "golden")
	geometryDir := filepath.Join(root, "dist", "cache", "map_geometry")
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("golden dir: %v", err)
	}
	ctx := context.Background()
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			want, err := ReadSnapshot(filepath.Join(goldenDir, name))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			demoPath := filepath.Join(root, "tests", want.Source)
			if _, err := os.Stat(demoPath); err != nil {
				t.Skipf("fixture %s not present", want.Source)
			}
			got, err := BuildWithGeometry(ctx, demoPath, "", geometryDir)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if diffs := Diff(want, got); len(diffs) > 0 {
				for _, d := range diffs {
					t.Errorf("diff: %s", d)
				}
			}
		})
	}
}
