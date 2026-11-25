package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strikeye/internal/gsi"
)

func writeLiveState(t *testing.T, dataRoot string, st gsi.State, age time.Duration) {
	t.Helper()
	path := liveStatePath(dataRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body, err := json.Marshal(struct {
		gsi.State
	}{State: st})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stamp := time.Now().Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

func TestLoadLiveStateFreshness(t *testing.T) {
	root := t.TempDir()
	if _, ok := loadLiveState(root, time.Now(), 15*time.Second); ok {
		t.Fatal("missing mirror reported as fresh")
	}

	st := gsi.State{Map: "de_ancient", Round: 9, TeamScoreCT: 5, TeamScoreT: 4, Bomb: "planted"}
	writeLiveState(t, root, st, 0)
	got, ok := loadLiveState(root, time.Now(), 15*time.Second)
	if !ok {
		t.Fatal("fresh mirror not reported")
	}
	if got.Map != "de_ancient" || got.Round != 9 {
		t.Fatalf("unexpected state: %+v", got)
	}

	writeLiveState(t, root, st, 30*time.Second)
	if _, ok := loadLiveState(root, time.Now(), 15*time.Second); ok {
		t.Fatal("stale mirror reported as fresh")
	}
}

func TestAliveCountsIgnoresDeadAndSpectators(t *testing.T) {
	st := gsi.State{Players: []gsi.Player{
		{Side: "CT", Alive: true},
		{Side: "CT", Alive: true},
		{Side: "CT", Alive: false},
		{Side: "T", Alive: true},
		{Side: "", Alive: true},
	}}
	ct, tv := aliveCounts(st)
	if ct != 2 || tv != 1 {
		t.Fatalf("alive counts = %d v %d, want 2 v 1", ct, tv)
	}
}

func TestLiveHeadline(t *testing.T) {
	st := gsi.State{
		Map:         "de_dust2",
		Round:       3,
		TeamScoreCT: 2,
		TeamScoreT:  1,
		Phase:       "live",
		Bomb:        "planted",
		Players: []gsi.Player{
			{Side: "CT", Alive: true},
			{Side: "T", Alive: true},
			{Side: "T", Alive: true},
		},
	}
	got := liveHeadline(st)
	want := "de_dust2 · 回合 3 · 2-1 · live · 炸弹 planted · 存活 1v2"
	if got != want {
		t.Fatalf("headline = %q, want %q", got, want)
	}

	// Carried bomb is not worth surfacing in the banner.
	st.Bomb = "carried"
	if got := liveHeadline(st); got != "de_dust2 · 回合 3 · 2-1 · live · 存活 1v2" {
		t.Fatalf("carried bomb headline = %q", got)
	}

	// Round zero falls back to the bare map name plus score.
	if got := liveHeadline(gsi.State{Map: "de_mirage"}); got != "de_mirage · 0-0 · 存活 0v0" {
		t.Fatalf("empty headline = %q", got)
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 7: "7", 10: "10", 42: "42", -13: "-13", 1000: "1000"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}
