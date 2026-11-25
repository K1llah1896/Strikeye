package mapcontrol

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"strikeye/internal/callouts"
	"strikeye/internal/mapgeom"
	"strikeye/internal/navmesh"
	"strikeye/internal/parser"
)

func twoAreaMesh(t *testing.T) *navmesh.Mesh {
	t.Helper()
	var buf []byte
	u32 := func(v uint32) { buf = binary.LittleEndian.AppendUint32(buf, v) }
	vec3 := func(x, y, z float32) {
		f := func(v float32) { buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(v)) }
		f(x)
		f(y)
		f(z)
	}
	zeros := func(n int) {
		for i := 0; i < n; i++ {
			buf = append(buf, 0)
		}
	}
	u32(0xFEEDFACE)
	u32(31)
	u32(1)
	u32(1) // analyzed

	u32(6) // corners
	vec3(0, 0, 0)
	vec3(100, 0, 0)
	vec3(100, 100, 0)
	vec3(0, 100, 0)
	vec3(200, 0, 0)
	vec3(200, 100, 0)
	u32(2) // polygons
	buf = append(buf, 4)
	u32(0)
	u32(1)
	u32(2)
	u32(3)
	buf = append(buf, 4)
	u32(1)
	u32(4)
	u32(5)
	u32(2)

	u32(2) // areas
	writeArea := func(id, poly uint32, neighbor uint32) {
		u32(id)
		buf = binary.LittleEndian.AppendUint64(buf, 0)
		buf = append(buf, 0)
		u32(poly)
		u32(0)
		for i := 0; i < 4; i++ {
			if i == 1 {
				u32(1)
				u32(neighbor)
				u32(0)
			} else {
				u32(0)
			}
		}
		zeros(5)
		u32(0)
		u32(0)
	}
	writeArea(1, 0, 2)
	writeArea(2, 1, 1)

	mesh, err := navmesh.Parse(buf)
	if err != nil {
		t.Fatalf("parse nav: %v", err)
	}
	return mesh
}

func testCallouts(t *testing.T) *callouts.Set {
	t.Helper()
	table, err := callouts.ParseCSV(strings.NewReader(`map,name,x1,y1,z1,x2,y2,z2
de_test,Alpha,0,-10,-10,100,110,10
de_test,Bravo,100,-10,-10,200,110,10
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

func replay(players []parser.ReplayPlayerPose) map[int]parser.ReplayRoundData {
	return map[int]parser.ReplayRoundData{
		1: {
			RoundNumber: 1,
			StartTick:   0,
			Frames: []parser.ReplayFrame{
				{Tick: 0, Players: players},
				{Tick: 320, Players: players},
			},
		},
	}
}

func TestBuildSplitsControlBetweenTeams(t *testing.T) {
	mesh := twoAreaMesh(t)
	set := testCallouts(t)
	cfg := DefaultConfig()
	cfg.ContestedGap = 50
	cfg.ReachRadius = 1000
	players := []parser.ReplayPlayerPose{
		{Slot: 1, Team: 1, Alive: true, X: 50, Y: 50, Z: 0},
		{Slot: 2, Team: 2, Alive: true, X: 150, Y: 50, Z: 0},
	}
	res := Build(replay(players), nil, mesh, set, 64, cfg)
	if len(res.Frames) == 0 {
		t.Fatal("no frames")
	}
	f := res.Frames[0]
	if f.OwnedT != 1 || f.OwnedCT != 1 || f.Contested != 0 || f.Neutral != 0 {
		t.Fatalf("frame = %+v", f)
	}
	if f.Net != 0 {
		t.Fatalf("net = %f", f.Net)
	}
	if len(res.Callouts) != 2 {
		t.Fatalf("callouts = %+v", res.Callouts)
	}
	for _, c := range res.Callouts {
		switch c.Callout {
		case "Alpha":
			if c.AvgNet != -1 {
				t.Fatalf("alpha = %+v", c)
			}
		case "Bravo":
			if c.AvgNet != 1 {
				t.Fatalf("bravo = %+v", c)
			}
		}
	}
}

func TestBuildMarksContestedAndNeutral(t *testing.T) {
	mesh := twoAreaMesh(t)
	cfg := DefaultConfig()
	cfg.ContestedGap = 500
	cfg.ReachRadius = 1000
	players := []parser.ReplayPlayerPose{
		{Slot: 1, Team: 1, Alive: true, X: 50, Y: 50, Z: 0},
		{Slot: 2, Team: 2, Alive: true, X: 150, Y: 50, Z: 0},
	}
	res := Build(replay(players), nil, mesh, nil, 64, cfg)
	if got := res.Frames[0].Contested; got != 2 {
		t.Fatalf("contested = %d", got)
	}

	// A single T confined to its own square leaves the other square neutral
	// once the reach radius is small.
	cfg.ReachRadius = 50
	cfg.ContestedGap = 10
	onlyT := []parser.ReplayPlayerPose{{Slot: 1, Team: 1, Alive: true, X: 50, Y: 50, Z: 0}}
	res = Build(replay(onlyT), nil, mesh, nil, 64, cfg)
	f := res.Frames[0]
	if f.OwnedT != 1 || f.Neutral != 1 {
		t.Fatalf("frame = %+v", f)
	}
	if f.Net != -1 {
		t.Fatalf("net = %f", f.Net)
	}
}

func TestBuildWithoutMeshOrTickRate(t *testing.T) {
	if res := Build(replay(nil), nil, nil, nil, 64, DefaultConfig()); len(res.Frames) != 0 {
		t.Fatalf("frames = %+v", res.Frames)
	}
	if res := Build(replay(nil), nil, twoAreaMesh(t), nil, 0, DefaultConfig()); len(res.Frames) != 0 {
		t.Fatalf("frames = %+v", res.Frames)
	}
}

func TestSmokeHidesAreaFromOwnership(t *testing.T) {
	mesh := twoAreaMesh(t)
	set := testCallouts(t)
	cfg := DefaultConfig()
	cfg.ContestedGap = 50
	cfg.ReachRadius = 1000
	cfg.SmokeRadius = 60
	players := []parser.ReplayPlayerPose{
		{Slot: 1, Team: 1, Alive: true, X: 50, Y: 50, Z: 0},
		{Slot: 2, Team: 2, Alive: true, X: 150, Y: 50, Z: 0},
	}
	replays := replay(players)
	rd := replays[1]
	rd.Smokes = []parser.SmokeEffect{{StartTick: 0, EndTick: 500, Pos: parser.Point3D{X: 150, Y: 50, Z: 0}}}
	replays[1] = rd

	res := Build(replays, nil, mesh, set, 64, cfg)
	if len(res.Frames) == 0 {
		t.Fatal("no frames")
	}
	f := res.Frames[0]
	if f.Blocked != 1 {
		t.Fatalf("blocked = %d, frame = %+v", f.Blocked, f)
	}
	if f.OwnedCT != 0 || f.OwnedT != 1 {
		t.Fatalf("smoke area still owned: %+v", f)
	}
	for _, c := range res.Callouts {
		if c.Callout == "Bravo" {
			t.Fatalf("smoked callout should not be aggregated: %+v", c)
		}
	}
}

func TestFireCutsTraversal(t *testing.T) {
	mesh := twoAreaMesh(t)
	cfg := DefaultConfig()
	cfg.ContestedGap = 50
	cfg.ReachRadius = 1000
	players := []parser.ReplayPlayerPose{
		{Slot: 1, Team: 1, Alive: true, X: 50, Y: 50, Z: 0},
		{Slot: 2, Team: 2, Alive: true, X: 150, Y: 50, Z: 0},
	}
	replays := replay(players)
	rd := replays[1]
	rd.Infernos = []parser.InfernoEffect{{
		StartTick: 0,
		EndTick:   500,
		Hull: []parser.Point2D{
			{X: 90, Y: 0}, {X: 210, Y: 0}, {X: 210, Y: 100}, {X: 90, Y: 100},
		},
	}}
	replays[1] = rd

	res := Build(replays, nil, mesh, nil, 64, cfg)
	f := res.Frames[0]
	if f.Blocked != 1 || f.OwnedCT != 0 {
		t.Fatalf("fire did not cut the area: %+v", f)
	}
	if f.OwnedT != 1 {
		t.Fatalf("T side of the map should stay reachable: %+v", f)
	}

	cfg.ApplyEffects = false
	plain := Build(replays, nil, mesh, nil, 64, cfg)
	if plain.Frames[0].Blocked != 0 {
		t.Fatalf("effects disabled but blocked = %d", plain.Frames[0].Blocked)
	}
	if plain.Frames[0].OwnedCT == 0 {
		t.Fatalf("without effects CT should reach T's area: %+v", plain.Frames[0])
	}
}

func TestPointInHull(t *testing.T) {
	hull := []parser.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	cases := map[[2]float32]bool{
		{50, 50}:  true,
		{-5, 50}:  false,
		{150, 50}: false,
		{1, 50}:   true,
		{99, 99}:  true,
	}
	for in, want := range cases {
		if got := pointInHull(mapgeom.Vec3{in[0], in[1], 0}, hull); got != want {
			t.Fatalf("point %v -> %v, want %v", in, got, want)
		}
	}
}
