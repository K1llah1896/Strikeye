package nadebook

import (
	"strings"
	"testing"

	"strikeye/internal/callouts"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

func testSet(t *testing.T) *callouts.Set {
	t.Helper()
	table, err := callouts.ParseCSV(strings.NewReader(`map,name,x1,y1,z1,x2,y2,z2
de_test,TSpawn,0,0,0,100,100,100
de_test,BombsiteA,900,900,0,1100,1100,100
`))
	if err != nil {
		t.Fatal(err)
	}
	set, err := table.For("de_test")
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func throw(steam, kind string, tick int64, from, to [3]float32) parser.GrenadeTrack {
	return parser.GrenadeTrack{
		ThrowerSteamID: steam,
		GrenadeType:    kind,
		ThrowTick:      tick,
		EndTick:        tick + 64,
		Points: []parser.GrenadeSample{
			{Tick: tick, Pos: parser.Point3D{X: from[0], Y: from[1], Z: from[2]}},
			{Tick: tick + 64, Pos: parser.Point3D{X: to[0], Y: to[1], Z: to[2]}},
		},
	}
}

func TestBuildClustersAndCountsFollowUpKills(t *testing.T) {
	replays := map[int]parser.ReplayRoundData{
		1: {
			RoundNumber: 1,
			StartTick:   0,
			Grenades: []parser.GrenadeTrack{
				throw("76561198000000001", "CSmokeGrenadeProjectile", 128, [3]float32{10, 10, 0}, [3]float32{900, 900, 0}),
				throw("76561198000000001", "CSmokeGrenadeProjectile", 256, [3]float32{30, 20, 0}, [3]float32{905, 895, 0}),
				throw("76561198000000002", "CFlashbangProjectile", 300, [3]float32{10, 10, 0}, [3]float32{1000, 1000, 0}),
			},
			Deaths: []parser.ReplayDeathEvent{
				{Tick: 128 + 64 + 128, Victim: parser.Point3D{X: 960, Y: 950, Z: 0}},
				{Tick: 128 + 64 + 3000, Victim: parser.Point3D{X: 960, Y: 950, Z: 0}},  // outside 8s window
				{Tick: 128 + 64 + 128, Victim: parser.Point3D{X: 5000, Y: 5000, Z: 0}}, // too far
			},
		},
	}
	players := []storage.MatchPlayer{
		{SteamID64: "76561198000000001", TeamKey: "team_b"},
		{SteamID64: "76561198000000002", TeamKey: "team_a"},
	}
	rounds := []storage.MatchRound{{RoundNumber: 1, FreezeEndTick: 0, TeamASide: "ct", TeamBSide: "t"}}

	rep := Build(replays, rounds, players, "Alpha", 64, testSet(t), DefaultConfig())
	if rep.TotalThrows != 3 {
		t.Fatalf("total throws = %d", rep.TotalThrows)
	}
	if len(rep.Lineups) != 2 {
		t.Fatalf("lineups = %+v", rep.Lineups)
	}
	smoke := rep.Lineups[0]
	if smoke.Type != "smoke" || smoke.Side != "t" || smoke.Throws != 2 || smoke.Rounds != 1 {
		t.Fatalf("smoke lineup = %+v", smoke)
	}
	if smoke.StartCallout != "T Spawn" || smoke.LandCallout != "A Site" {
		t.Fatalf("callouts = %q -> %q", smoke.StartCallout, smoke.LandCallout)
	}
	if smoke.FollowUpKills != 1 {
		t.Fatalf("follow-up kills = %d", smoke.FollowUpKills)
	}
	if smoke.AvgTimeSec <= 0 {
		t.Fatalf("avg time = %f", smoke.AvgTimeSec)
	}
	if smoke.Start[0] < 15 || smoke.Start[0] > 25 {
		t.Fatalf("mean start = %+v", smoke.Start)
	}
	if rep.Lineups[1].Type != "flash" || rep.Lineups[1].Side != "ct" {
		t.Fatalf("flash lineup = %+v", rep.Lineups[1])
	}
}

func TestBuildWithoutCalloutsOrStructure(t *testing.T) {
	replays := map[int]parser.ReplayRoundData{
		1: {RoundNumber: 1, Grenades: []parser.GrenadeTrack{
			{ThrowerSteamID: "x", GrenadeType: "smoke", Points: nil}, // no trajectory
		}},
	}
	rep := Build(replays, nil, nil, "", 64, nil, Config{})
	if rep.TotalThrows != 1 || rep.NamedThrows != 0 || len(rep.Lineups) != 0 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Config.GridUnits == 0 {
		t.Fatalf("default config not applied: %+v", rep.Config)
	}
}

func TestNormalizeHelpers(t *testing.T) {
	cases := map[string]string{
		"CSmokeGrenadeProjectile": "smoke",
		"CFlashbangProjectile":    "flash",
		"CMolotovProjectile":      "molotov",
		"CIncendiaryGrenade":      "incendiary",
		"CHEGrenadeProjectile":    "he",
		"CDecoyProjectile":        "decoy",
	}
	for in, want := range cases {
		if got := normalizeType(in); got != want {
			t.Fatalf("%s -> %s, want %s", in, got, want)
		}
	}
	if normalizeSide("CT") != "ct" || normalizeSide("Terrorist") != "t" || normalizeSide("") != "" {
		t.Fatal("side normalization")
	}
}

func TestBuildWinRateAndImpact(t *testing.T) {
	// team_b (T side) throws the same smoke in three rounds and wins two of them;
	// the team's T-side baseline across all four rounds is 50%.
	replays := map[int]parser.ReplayRoundData{}
	for rn := 1; rn <= 4; rn++ {
		rd := parser.ReplayRoundData{RoundNumber: uint32(rn)}
		if rn <= 3 {
			rd.Grenades = []parser.GrenadeTrack{
				throw("76561198000000001", "CSmokeGrenadeProjectile", 128, [3]float32{10, 10, 0}, [3]float32{900, 900, 0}),
			}
		}
		replays[rn] = rd
	}
	players := []storage.MatchPlayer{{SteamID64: "76561198000000001", TeamKey: "team_b"}}
	winA, winB := "team_a", "team_b"
	rounds := []storage.MatchRound{
		{RoundNumber: 1, TeamASide: "ct", TeamBSide: "t", WinnerTeamKey: &winB},
		{RoundNumber: 2, TeamASide: "ct", TeamBSide: "t", WinnerTeamKey: &winB},
		{RoundNumber: 3, TeamASide: "ct", TeamBSide: "t", WinnerTeamKey: &winA},
		{RoundNumber: 4, TeamASide: "ct", TeamBSide: "t", WinnerTeamKey: &winA},
	}

	rep := Build(replays, rounds, players, "Alpha", 64, testSet(t), DefaultConfig())
	if len(rep.Lineups) != 1 {
		t.Fatalf("lineups = %+v", rep.Lineups)
	}
	l := rep.Lineups[0]
	if l.Rounds != 3 || l.Wins != 2 {
		t.Fatalf("rounds/wins = %d/%d", l.Rounds, l.Wins)
	}
	if l.WinRate != 66.7 {
		t.Fatalf("win rate = %f", l.WinRate)
	}
	// T-side baseline is 2/4 = 50%, so impact is +16.7 points.
	if l.Impact != 16.7 {
		t.Fatalf("impact = %f", l.Impact)
	}
	if l.KillsPerThrow != 0 {
		t.Fatalf("kills per throw = %f", l.KillsPerThrow)
	}
}
