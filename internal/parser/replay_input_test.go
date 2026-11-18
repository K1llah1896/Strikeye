package parser

import (
	"path/filepath"
	"testing"
)

func TestInputCollectorCapabilityAndNoSyntheticMouse(t *testing.T) {
	icNone := NewInputCollector()
	icNone.ObserveButtons(1, 100, 1, 0)
	if icNone.Capability() != InputCapNone {
		t.Fatalf("expected none, got %s", icNone.Capability())
	}

	icEnt := NewInputCollector()
	icEnt.ObserveButtons(1, 100, 1, KeyW|KeyFire)
	icEnt.ObserveButtons(1, 101, 1, KeyW|KeyFire) // unchanged -> no duplicate edge
	icEnt.ObserveButtons(1, 102, 1, KeyW)         // release fire -> edge
	if icEnt.Capability() != InputCapEntityButtons {
		t.Fatalf("expected entity_buttons_only, got %s", icEnt.Capability())
	}
	if len(icEnt.RoundEdges(1)) != 2 {
		t.Fatalf("expected 2 button edges, got %d", len(icEnt.RoundEdges(1)))
	}

	icEnt.ObserveUserCmd()
	if icEnt.Capability() != InputCapFullUserCmd {
		t.Fatalf("expected full_usercmd, got %s", icEnt.Capability())
	}
}

func TestReplayBinaryRoundTripAndRandomAccess(t *testing.T) {
	tmp := t.TempDir()
	rplPath := filepath.Join(tmp, "cache", "replay_32hz", "sample.rpl")

	rounds := []ReplayRoundData{
		{
			RoundNumber: 1,
			StartTick:   100,
			EndTick:     500,
			Frames: []ReplayFrame{
				{
					Tick: 100,
					Players: []ReplayPlayerPose{
						{Slot: 1, Team: 2, Alive: true, Health: 100, Armor: 100, Money: 800, EquipValue: 750, X: 10.5, Y: -20.25, Z: 64, Yaw: 90, Pitch: -2, ActiveWeapon: "usp_silencer"},
					},
				},
			},
			Grenades: []GrenadeTrack{
				{EntityID: 42, RoundNumber: 1, ThrowerSteamID: "76561198000000001", GrenadeType: "smokegrenade", ThrowTick: 150, EndTick: 220, Points: []GrenadeSample{{Tick: 150, Pos: Point3D{1, 2, 3}}}},
			},
			Smokes: []SmokeEffect{
				{EntityID: 42, RoundNumber: 1, ThrowerSteamID: "76561198000000001", StartTick: 220, EndTick: 1372, Pos: Point3D{10, 20, 30}},
			},
			Infernos: []InfernoEffect{
				{UniqueID: 7, RoundNumber: 1, ThrowerSteamID: "76561198000000001", StartTick: 250, EndTick: 700, Hull: []Point2D{{0, 0}, {100, 0}, {50, 80}}},
			},
			InputEdges: []InputEdge{
				{Tick: 101, PlayerSlot: 1, Buttons: KeyW | KeyFire},
			},
		},
		{
			RoundNumber: 2,
			StartTick:   600,
			EndTick:     1200,
			InputEdges: []InputEdge{
				{Tick: 615, PlayerSlot: 2, Buttons: KeyA | KeyCrouch},
			},
		},
	}

	if err := WriteReplayFileAtomic(rplPath, 64.0, InputCapEntityButtons, rounds); err != nil {
		t.Fatalf("WriteReplayFileAtomic: %v", err)
	}

	hdr, entries, err := ReadReplayIndex(rplPath)
	if err != nil {
		t.Fatalf("ReadReplayIndex: %v", err)
	}
	if hdr.Magic != ReplayMagicBytes || hdr.RoundCount != 2 || InputCapString(hdr.InputCap) != InputCapEntityButtons {
		t.Fatalf("unexpected header: %+v", hdr)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 index entries, got %d", len(entries))
	}

	_, r1, err := ReadReplayRound(rplPath, 1)
	if err != nil {
		t.Fatalf("ReadReplayRound(1): %v", err)
	}
	if len(r1.Frames) != 1 || r1.Frames[0].Players[0].ActiveWeapon != "usp_silencer" {
		t.Fatalf("unexpected r1 frames: %+v", r1.Frames)
	}
	if len(r1.Grenades) != 1 || len(r1.Smokes) != 1 || len(r1.Infernos) != 1 || len(r1.Infernos[0].Hull) != 3 {
		t.Fatalf("unexpected r1 utility geometry: g=%d s=%d i=%d", len(r1.Grenades), len(r1.Smokes), len(r1.Infernos))
	}

	_, r2, err := ReadReplayRound(rplPath, 2)
	if err != nil {
		t.Fatalf("ReadReplayRound(2): %v", err)
	}
	if len(r2.InputEdges) != 1 || r2.InputEdges[0].Buttons != (KeyA|KeyCrouch) {
		t.Fatalf("unexpected r2 input edges: %+v", r2.InputEdges)
	}
}
