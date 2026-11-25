package killctx

import (
	"math"
	"testing"

	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

func slotPtr(v int) *int { return &v }

func testRound() map[int]parser.ReplayRoundData {
	return map[int]parser.ReplayRoundData{
		3: {
			RoundNumber: 3,
			Frames:      []parser.ReplayFrame{{Tick: 6000}},
			Deaths: []parser.ReplayDeathEvent{{
				Tick:       6200,
				KillerSlot: 1, VictimSlot: 2, Headshot: true,
				Killer: parser.Point3D{X: 0, Y: 0, Z: 0},
				Victim: parser.Point3D{X: 0, Y: 500, Z: 0},
			}},
			Smokes: []parser.SmokeEffect{{
				StartTick: 6100, EndTick: 7000,
				Pos: parser.Point3D{X: 0, Y: 250, Z: 0},
			}},
		},
	}
}

func testPlayers() []storage.MatchPlayer {
	return []storage.MatchPlayer{
		{PlayerName: "killer", SpecSlot: slotPtr(1)},
		{PlayerName: "victim", SpecSlot: slotPtr(2)},
	}
}

func TestBuildAnnotatesDistanceSmokeAndTime(t *testing.T) {
	rounds := []storage.MatchRound{{RoundNumber: 3, FreezeEndTick: 6000}}
	rep := Build(testRound(), rounds, testPlayers(), 64, nil, DefaultConfig())
	if len(rep.Kills) != 1 {
		t.Fatalf("kills = %d", len(rep.Kills))
	}
	k := rep.Kills[0]
	if k.Killer != "killer" || k.Victim != "victim" {
		t.Fatalf("names = %q/%q", k.Killer, k.Victim)
	}
	if !k.Headshot {
		t.Fatal("headshot lost")
	}
	if math.Abs(k.TimeSec-200.0/64.0) > 1e-6 {
		t.Fatalf("time = %f", k.TimeSec)
	}
	want := math.Sqrt(500*500 + 19*19)
	if math.Abs(k.Distance-want) > 0.01 {
		t.Fatalf("distance = %f want %f", k.Distance, want)
	}
	if !k.SmokeBlocked {
		t.Fatal("line through smoke not flagged")
	}
	if rep.GeometryUsed || k.Occluded || k.Visible {
		t.Fatalf("geometry should be absent: %+v", rep)
	}
	if math.Abs(rep.SmokePct-100) > 1e-6 {
		t.Fatalf("smoke pct = %f", rep.SmokePct)
	}
}

func TestBuildIgnoresExpiredSmoke(t *testing.T) {
	replays := testRound()
	rd := replays[3]
	rd.Smokes[0].EndTick = 6150
	replays[3] = rd
	rep := Build(replays, nil, testPlayers(), 64, nil, DefaultConfig())
	if rep.Kills[0].SmokeBlocked {
		t.Fatal("expired smoke counted")
	}
	if rep.SmokePct != 0 {
		t.Fatalf("smoke pct = %f", rep.SmokePct)
	}
}

func TestBuildFallsBackToFirstFrameForTimeBase(t *testing.T) {
	rep := Build(testRound(), nil, testPlayers(), 64, nil, DefaultConfig())
	if math.Abs(rep.Kills[0].TimeSec-200.0/64.0) > 1e-6 {
		t.Fatalf("time = %f", rep.Kills[0].TimeSec)
	}
}

func TestSegmentWithin(t *testing.T) {
	a := [3]float64{0, 0, 0}
	b := [3]float64{0, 100, 0}
	cases := []struct {
		name string
		pt   [3]float64
		rad  float64
		want bool
	}{
		{"on segment", [3]float64{0, 50, 0}, 10, true},
		{"near segment", [3]float64{5, 50, 0}, 10, true},
		{"outside radius", [3]float64{15, 50, 0}, 10, false},
		{"beyond end", [3]float64{0, 180, 0}, 10, false},
		{"behind start", [3]float64{0, -80, 0}, 10, false},
	}
	for _, c := range cases {
		if got := segmentWithin(a, b, c.pt, c.rad); got != c.want {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	if !segmentWithin(a, a, [3]float64{0, 5, 0}, 10) {
		t.Fatal("degenerate segment")
	}
}

func TestBuildSortsByRoundAndTick(t *testing.T) {
	replays := map[int]parser.ReplayRoundData{
		2: {Deaths: []parser.ReplayDeathEvent{{Tick: 20, KillerSlot: 1, VictimSlot: 2}, {Tick: 10, KillerSlot: 1, VictimSlot: 2}}},
		1: {Deaths: []parser.ReplayDeathEvent{{Tick: 99, KillerSlot: 1, VictimSlot: 2}}},
	}
	rep := Build(replays, nil, testPlayers(), 64, nil, DefaultConfig())
	if len(rep.Kills) != 3 {
		t.Fatalf("kills = %d", len(rep.Kills))
	}
	if rep.Kills[0].Round != 1 || rep.Kills[1].Tick != 10 || rep.Kills[2].Tick != 20 {
		t.Fatalf("order = %+v", rep.Kills)
	}
}

func opticsRound() map[int]parser.ReplayRoundData {
	return map[int]parser.ReplayRoundData{
		1: {
			Frames: []parser.ReplayFrame{
				{
					Tick: 100,
					Players: []parser.ReplayPlayerPose{
						{Slot: 1, Alive: true, Yaw: 0},   // killer looking +X
						{Slot: 2, Alive: true, Yaw: 180}, // victim looking -X, straight at the killer
					},
				},
				{
					Tick: 115,
					Players: []parser.ReplayPlayerPose{
						{Slot: 1, Alive: true, Yaw: 180}, // killer turned away
						{Slot: 2, Alive: true, Yaw: 0},   // victim looking away from the killer
					},
				},
			},
			Deaths: []parser.ReplayDeathEvent{
				{Tick: 110, KillerSlot: 1, VictimSlot: 2, Killer: parser.Point3D{X: 0, Y: 0}, Victim: parser.Point3D{X: 500, Y: 0}},
				{Tick: 120, KillerSlot: 1, VictimSlot: 2, Killer: parser.Point3D{X: 0, Y: 0}, Victim: parser.Point3D{X: 400, Y: 0}},
			},
		},
	}
}

func TestBuildOpticsFOVAndAmbush(t *testing.T) {
	rep := Build(opticsRound(), nil, testPlayers(), 64, nil, DefaultConfig())
	if !rep.OpticsUsed || len(rep.Kills) != 2 {
		t.Fatalf("optics=%v kills=%d", rep.OpticsUsed, len(rep.Kills))
	}
	first := rep.Kills[0]
	if !first.Optics || !first.InFOV {
		t.Fatalf("killer aiming at victim should be in FOV: %+v", first)
	}
	if first.AimOffDeg > 0.5 {
		t.Fatalf("aim off = %f", first.AimOffDeg)
	}
	if !first.VictimSeesKiller {
		t.Fatal("victim facing killer should see them")
	}
	second := rep.Kills[1]
	if second.InFOV {
		t.Fatalf("victim behind the killer must be outside FOV: %+v", second)
	}
	if math.Abs(second.AimOffDeg-180) > 0.5 {
		t.Fatalf("aim off = %f", second.AimOffDeg)
	}
	if rep.FOVPct != 50 || rep.AmbushPct != 50 {
		t.Fatalf("fov=%f ambush=%f", rep.FOVPct, rep.AmbushPct)
	}
}

func TestBuildOpticsFlash(t *testing.T) {
	rd := opticsRound()
	round := rd[1]
	round.Frames[0].Players[0].FlashDur = 1.4
	round.Frames[1].Players[0].FlashDur = 1.4
	rd[1] = round
	rep := Build(rd, nil, testPlayers(), 64, nil, DefaultConfig())
	if !rep.Kills[0].AttackerBlind {
		t.Fatal("flashed killer not flagged")
	}
	if rep.BlindKills != 2 {
		t.Fatalf("blind kills = %d", rep.BlindKills)
	}
}

func TestBuildWithoutFramesLeavesOpticsUnchecked(t *testing.T) {
	rep := Build(testRound(), nil, testPlayers(), 64, nil, DefaultConfig())
	if rep.OpticsUsed {
		t.Fatal("optics should not be reported without frames")
	}
	if rep.Kills[0].Optics {
		t.Fatal("kill should be marked unchecked")
	}
}

// TestBuildOpticsRespectsZoom checks that a scoped attacker needs the victim
// inside a much narrower cone to count as in-FOV.
func TestBuildOpticsRespectsZoom(t *testing.T) {
	build := func(zoom uint8) Kill {
		rd := map[int]parser.ReplayRoundData{
			1: {
				RoundNumber: 1,
				// Victim 15° off the killer's view: inside the 53° hip cone and
				// the 20° half-zoom cone, outside the 7.5° full-zoom cone.
				Frames: []parser.ReplayFrame{{
					Tick: 100,
					Players: []parser.ReplayPlayerPose{
						{Slot: 1, Alive: true, Yaw: 15, Zoom: zoom},
						{Slot: 2, Alive: true, Yaw: 180},
					},
				}},
				Deaths: []parser.ReplayDeathEvent{{
					Tick: 100, KillerSlot: 1, VictimSlot: 2,
					Killer: parser.Point3D{X: 0, Y: 0}, Victim: parser.Point3D{X: 500, Y: 0},
				}},
			},
		}
		rep := Build(rd, nil, testPlayers(), 64, nil, DefaultConfig())
		if len(rep.Kills) != 1 {
			t.Fatalf("kills = %d", len(rep.Kills))
		}
		return rep.Kills[0]
	}

	hip := build(0)
	if !hip.Optics || !hip.InFOV || hip.AttackerZoom != 0 {
		t.Fatalf("hip fire should be in FOV: %+v", hip)
	}
	half := build(1)
	if !half.InFOV || half.AttackerZoom != 1 {
		t.Fatalf("half zoom should still cover 15°: %+v", half)
	}
	full := build(2)
	if full.InFOV || full.AttackerZoom != 2 {
		t.Fatalf("full zoom must not cover 15°: %+v", full)
	}
	// The victim's zoom narrows their own cone, so a scoped victim facing the
	// killer 30° off no longer counts as seeing them.
	rd := map[int]parser.ReplayRoundData{
		1: {
			RoundNumber: 1,
			Frames: []parser.ReplayFrame{{
				Tick: 100,
				Players: []parser.ReplayPlayerPose{
					{Slot: 1, Alive: true, Yaw: 0},
					{Slot: 2, Alive: true, Yaw: 210, Zoom: 2},
				},
			}},
			Deaths: []parser.ReplayDeathEvent{{
				Tick: 100, KillerSlot: 1, VictimSlot: 2,
				Killer: parser.Point3D{X: 0, Y: 0}, Victim: parser.Point3D{X: 500, Y: 0},
			}},
		},
	}
	k := Build(rd, nil, testPlayers(), 64, nil, DefaultConfig()).Kills[0]
	if k.VictimSeesKiller || k.VictimZoom != 2 {
		t.Fatalf("scoped victim should not see a killer 30° off: %+v", k)
	}
}
