package spectator

import (
	"os"
	"path/filepath"
	"testing"

	"strikeye/internal/storage"
)

func TestResolvePOVStatesStrictMediaSeparation(t *testing.T) {
	tmp := t.TempDir()
	validMP4 := filepath.Join(tmp, "r1_donk.mp4")
	if err := os.WriteFile(validMP4, []byte("mp4data"), 0o644); err != nil {
		t.Fatal(err)
	}

	rnd := 1
	p1 := "76561198000000001"
	p2 := "76561198000000002"
	p3 := "76561198000000003"

	players := []storage.MatchPlayer{
		{SteamID64: p1, PlayerName: "donk", TeamKey: "T"},
		{SteamID64: p2, PlayerName: "sh1ro", TeamKey: "T"},
		{SteamID64: p3, PlayerName: "chopper", TeamKey: "T"},
	}
	videos := []storage.MediaVideo{
		{RoundNumber: &rnd, Perspective: "pov", PlayerSteamID64: &p1, FilePath: validMP4},
	}
	tasks := []storage.RenderTask{
		{DemoID: 10, RoundNumber: &rnd, Perspective: "pov", TargetSteamID64: p2, Status: "queued"},
	}

	states := ResolvePOVStates(players, videos, tasks, nil, 10, 1)
	if len(states) != 4 {
		t.Fatalf("expected 3 players + 1 observer, got %d", len(states))
	}
	if states[0].State != "ready" || states[0].Video == nil {
		t.Fatalf("expected p1 ready, got %+v", states[0])
	}
	if states[1].State != "queued" {
		t.Fatalf("expected p2 queued, got %+v", states[1])
	}
	if states[2].State != "unrendered" || states[2].Video != nil {
		t.Fatalf("expected p3 unrendered without fallback video, got %+v", states[2])
	}
	if states[3].Key != "observer" || states[3].State != "unrendered" {
		t.Fatalf("expected observer unrendered, got %+v", states[3])
	}
}
