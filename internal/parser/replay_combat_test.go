package parser

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func combatRound() ReplayRoundData {
	return ReplayRoundData{
		RoundNumber: 3,
		StartTick:   100,
		EndTick:     900,
		Frames: []ReplayFrame{{Tick: 100, Players: []ReplayPlayerPose{
			{Slot: 1, Team: 1, Alive: true, Health: 100, X: 1, Y: 2, Z: 3},
		}}},
		Shots: []ReplayShotEvent{
			{Tick: 150, PlayerSlot: 1, Weapon: "ak47", Start: Point3D{10, 20, 64}, End: Point3D{510, 20, 64}},
			{Tick: 151, PlayerSlot: 2, Weapon: "awp", Start: Point3D{-10, -20, 64}, End: Point3D{-1010, -20, 40}},
		},
		Deaths: []ReplayDeathEvent{
			{Tick: 400, KillerSlot: 1, VictimSlot: 2, Headshot: true, Victim: Point3D{5, 6, 7}, Killer: Point3D{8, 9, 10}},
			{Tick: 500, KillerSlot: 0, VictimSlot: 3, Victim: Point3D{-1, -2, -3}, Killer: Point3D{-1, -2, -3}},
		},
		Bombs: []ReplayBombEvent{
			{Tick: 600, Kind: ReplayBombPlant, Pos: Point3D{100, 200, 0}},
			{Tick: 700, Kind: ReplayBombDefuse, Pos: Point3D{100, 200, 0}},
			{Tick: 800, Kind: ReplayBombExplode, Pos: Point3D{100, 200, 0}},
		},
	}
}

func TestReplayCombatChunkRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "combat.rpl")
	if err := WriteReplayFileAtomic(path, 64, InputCapEntityButtons, []ReplayRoundData{combatRound()}); err != nil {
		t.Fatalf("write: %v", err)
	}
	hdr, _, err := ReadReplayIndex(path)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if hdr.Version != ReplayFormatVersion {
		t.Fatalf("expected format version %d, got %d", ReplayFormatVersion, hdr.Version)
	}
	_, rd, err := ReadReplayRound(path, 3)
	if err != nil {
		t.Fatalf("read round: %v", err)
	}
	if len(rd.Shots) != 2 || rd.Shots[1].Weapon != "awp" || rd.Shots[1].End.X != -1010 {
		t.Fatalf("shots mismatch: %+v", rd.Shots)
	}
	if len(rd.Deaths) != 2 || !rd.Deaths[0].Headshot || rd.Deaths[0].KillerSlot != 1 || rd.Deaths[1].Victim.Z != -3 {
		t.Fatalf("deaths mismatch: %+v", rd.Deaths)
	}
	if len(rd.Bombs) != 3 || rd.Bombs[2].Kind != ReplayBombExplode || rd.Bombs[0].Pos.Y != 200 {
		t.Fatalf("bombs mismatch: %+v", rd.Bombs)
	}
}

func TestReplayV1CacheReadsWithEmptyCombatChunk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.rpl")
	prev := ReplayFormatVersion
	ReplayFormatVersion = 1
	err := WriteReplayFileAtomic(path, 64, InputCapEntityButtons, []ReplayRoundData{combatRound()})
	ReplayFormatVersion = prev
	if err != nil {
		t.Fatalf("write v1: %v", err)
	}

	hdr, _, err := ReadReplayIndex(path)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if hdr.Version != 1 {
		t.Fatalf("expected version 1, got %d", hdr.Version)
	}
	_, rd, err := ReadReplayRound(path, 3)
	if err != nil {
		t.Fatalf("read v1 round: %v", err)
	}
	if len(rd.Shots) != 0 || len(rd.Deaths) != 0 || len(rd.Bombs) != 0 {
		t.Fatalf("expected no combat events from v1 cache, got %d/%d/%d", len(rd.Shots), len(rd.Deaths), len(rd.Bombs))
	}
	if len(rd.Frames) != 1 {
		t.Fatalf("legacy frames lost: %d", len(rd.Frames))
	}
}

func TestReplayRejectsNewerFormatVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.rpl")
	if err := WriteReplayFileAtomic(path, 64, InputCapNone, []ReplayRoundData{combatRound()}); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint16(raw[8:10], 99)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadReplayIndex(path); !errors.Is(err, ErrUnsupportedReplayVer) {
		t.Fatalf("expected ErrUnsupportedReplayVer, got %v", err)
	}
}
