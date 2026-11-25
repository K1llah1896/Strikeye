package callouts

import (
	"errors"
	"strings"
	"testing"

	"strikeye/internal/mapgeom"
)

const testCSV = `map,name,x1,y1,z1,x2,y2,z2
de_dust2,BombsiteA,1000,1000,0,1500,1500,100
de_dust2,MidDoors,1200,1200,0,1300,1300,50
de_dust2,LongA,2000,0,0,2600,600,80
de_dust2,,0,0,0,10,10,10
de_ancient,TSpawn,-500,-500,0,-300,-300,64
de_dust2,Broken,not,a,number,0,0,0
`

func loadTestTable(t *testing.T) *Table {
	t.Helper()
	table, err := ParseCSV(strings.NewReader(testCSV))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	return table
}

func TestParseCSVAndLookup(t *testing.T) {
	table := loadTestTable(t)
	if got := table.Maps(); len(got) != 2 || got[0] != "de_ancient" || got[1] != "de_dust2" {
		t.Fatalf("maps = %v", got)
	}
	set, err := table.For("DE_DUST2.dem")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if set.Len() != 3 {
		t.Fatalf("places = %d, want 3 (empty name and broken row skipped)", set.Len())
	}

	// Overlapping boxes: the smaller (Mid Doors) wins inside its area.
	place, ok := set.At(mapgeom.Vec3{1250, 1250, 0})
	if !ok || place.ID != "MidDoors" || place.Label != "Mid Doors" {
		t.Fatalf("overlap lookup = %+v, %v", place, ok)
	}
	place, ok = set.At(mapgeom.Vec3{1450, 1450, 40})
	if !ok || place.ID != "BombsiteA" || place.Label != "A Site" {
		t.Fatalf("A site lookup = %+v, %v", place, ok)
	}

	// Outside every box but close: nearest centre wins.
	place, ok = set.At(mapgeom.Vec3{2500, 700, 0})
	if !ok || place.ID != "LongA" {
		t.Fatalf("near lookup = %+v, %v", place, ok)
	}

	// Far from everything: no name.
	if place, ok := set.At(mapgeom.Vec3{9000, 9000, 0}); ok {
		t.Fatalf("far lookup = %+v, want none", place)
	}

	// Z outside the box by more than the tolerance is not a containment hit.
	if _, ok := set.At(mapgeom.Vec3{1250, 1250, 400}); !ok {
		t.Fatal("high point should still resolve by proximity")
	}
}

func TestForUnknownMap(t *testing.T) {
	table := loadTestTable(t)
	if _, err := table.For("de_nuke"); !errors.Is(err, ErrUnknownMap) {
		t.Fatalf("err = %v", err)
	}
}

func TestFindAndPlaces(t *testing.T) {
	set, err := loadTestTable(t).For("de_dust2")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Find("LongA"); !ok {
		t.Fatal("LongA missing")
	}
	places := set.Places()
	if len(places) != 3 || places[0].Label != "A Site" {
		t.Fatalf("sorted places = %+v", places)
	}
}

func TestLabel(t *testing.T) {
	cases := map[string]string{
		"BombsiteB":      "B Site",
		"TSpawn":         "T Spawn",
		"LongA":          "Long A",
		"OutsideTunnel":  "Outside Tunnel",
		"CTSideUpper":    "CT Side Upper",
		"SomeNewPlace":   "Some New Place",
		"UpperTunnel":    "Upper Tunnel",
		"AContainerYard": "A Container Yard",
	}
	for id, want := range cases {
		if got := Label(id); got != want {
			t.Errorf("Label(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestParseCSVRejectsMissingColumns(t *testing.T) {
	if _, err := ParseCSV(strings.NewReader("map,name\nx,y\n")); err == nil {
		t.Fatal("expected missing column error")
	}
}

func TestLookupReportsExactAndSnap(t *testing.T) {
	table := loadTestTable(t)
	set, err := table.For("de_dust2")
	if err != nil {
		t.Fatal(err)
	}
	inside, ok := set.Lookup(mapgeom.Vec3{1250, 1250, 10})
	if !ok || !inside.Exact || inside.Distance != 0 {
		t.Fatalf("inside = %+v ok=%v", inside, ok)
	}
	near, ok := set.Lookup(mapgeom.Vec3{1600, 1250, 10})
	if !ok || near.Exact {
		t.Fatalf("near = %+v ok=%v", near, ok)
	}
	if near.Distance <= 0 || near.Distance > 400 {
		t.Fatalf("near distance = %f", near.Distance)
	}
	if _, ok := set.Lookup(mapgeom.Vec3{5000, 5000, 0}); ok {
		t.Fatal("far point should not match")
	}
}
