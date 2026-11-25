package zones

import (
	"strings"
	"testing"

	"strikeye/internal/callouts"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

const zoneCSV = `map,name,x1,y1,z1,x2,y2,z2
de_dust2,BombsiteA,0,0,0,100,100,50
de_dust2,BombsiteB,200,200,0,300,300,50
`

func testSet(t *testing.T) *callouts.Set {
	t.Helper()
	table, err := callouts.ParseCSV(strings.NewReader(zoneCSV))
	if err != nil {
		t.Fatal(err)
	}
	set, err := table.For("de_dust2")
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestSetupBinsPlayersByCalloutAtTwentySeconds(t *testing.T) {
	set := testSet(t)
	rd := parser.ReplayRoundData{
		RoundNumber: 1,
		StartTick:   500,
		Frames: []parser.ReplayFrame{
			{Tick: 1000, Players: []parser.ReplayPlayerPose{
				{Slot: 1, Team: 1, Alive: true, X: 10, Y: 10, Z: 0}, // A, before freeze end
			}},
			{Tick: 2280, Players: []parser.ReplayPlayerPose{
				{Slot: 1, Team: 1, Alive: true, X: 20, Y: 20, Z: 0},   // A Site
				{Slot: 2, Team: 2, Alive: true, X: 250, Y: 250, Z: 0}, // B Site
				{Slot: 3, Team: 2, Alive: false, X: 250, Y: 250, Z: 0},
				{Slot: 4, Team: 1, Alive: true, X: 9000, Y: 9000, Z: 0}, // no callout
			}},
		},
	}
	rounds := []storage.MatchRound{{RoundNumber: 1, FreezeEndTick: 1000}}
	totals := Setup(map[int]parser.ReplayRoundData{1: rd}, rounds, 64, set)
	if totals.Rounds != 1 || totals.Sampled != 3 {
		t.Fatalf("rounds=%d sampled=%d", totals.Rounds, totals.Sampled)
	}
	if totals.All["A Site"] != 1 || totals.T["A Site"] != 1 || totals.CT["B Site"] != 1 {
		t.Fatalf("totals = %+v", totals)
	}
	if totals.Unnamed != 1 {
		t.Fatalf("unnamed = %d", totals.Unnamed)
	}
	rows := totals.Rows()
	if len(rows) != 2 || rows[0].Total != 1 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestSetupWithoutCalloutsOrRounds(t *testing.T) {
	if totals := Setup(nil, nil, 64, testSet(t)); totals.Rounds != 0 || totals.Sampled != 0 {
		t.Fatalf("totals = %+v", totals)
	}
	rd := map[int]parser.ReplayRoundData{1: {RoundNumber: 1}}
	if totals := Setup(rd, nil, 64, nil); totals.Sampled != 0 {
		t.Fatalf("nil set sampled = %d", totals.Sampled)
	}
}

func TestDeathsResolveVictimCallout(t *testing.T) {
	set := testSet(t)
	rd := parser.ReplayRoundData{
		RoundNumber: 3,
		StartTick:   64 * 100,
		Deaths: []parser.ReplayDeathEvent{
			{Tick: 64 * 140, KillerSlot: 1, VictimSlot: 2, Headshot: true, Victim: parser.Point3D{X: 50, Y: 50, Z: 0}},
			{Tick: 64 * 150, KillerSlot: 2, VictimSlot: 3, Victim: parser.Point3D{X: 9000, Y: 9000, Z: 0}},
		},
	}
	names := map[uint8]string{1: "s1mple", 2: "ZywOo", 3: "NiKo"}
	rounds := []storage.MatchRound{{RoundNumber: 3, StartTick: 64 * 100, FreezeEndTick: 64 * 100}}
	deaths := Deaths(map[int]parser.ReplayRoundData{3: rd}, rounds, 64, set, names)
	if len(deaths) != 2 {
		t.Fatalf("deaths = %+v", deaths)
	}
	first := deaths[0]
	if first.Round != 3 || first.Killer != "s1mple" || first.Victim != "ZywOo" || !first.Headshot {
		t.Fatalf("first = %+v", first)
	}
	if first.Callout != "A Site" || !first.Found {
		t.Fatalf("callout = %q found=%v", first.Callout, first.Found)
	}
	if first.TimeSec != 40 {
		t.Fatalf("time = %f", first.TimeSec)
	}
	if deaths[1].Found {
		t.Fatalf("second = %+v, want unnamed", deaths[1])
	}
}
