package analysis

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/callouts"
	"strikeye/internal/storage"
)

const pinsCSV = `map,name,x1,y1,z1,x2,y2,z2
de_dust2,LongA,100,200,0,300,400,128
de_dust2,BombsiteA,-200,-400,0,0,-200,128
de_dust2,Mid,500,0,0,700,200,128
`

func pinsSet(t *testing.T) *callouts.Set {
	t.Helper()
	table, err := callouts.ParseCSV(strings.NewReader(pinsCSV))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	set, err := table.For("de_dust2")
	if err != nil {
		t.Fatalf("for: %v", err)
	}
	return set
}

func TestPinsFromAnnotations(t *testing.T) {
	set := pinsSet(t)
	pins := pinsFromAnnotations(set, []storage.MapAnnotation{
		{CalloutID: "LongA", Color: "red"},
		{CalloutID: "Mid", Color: "green"},
		{CalloutID: "Nope", Color: "amber"},
	})
	if len(pins) != 2 {
		t.Fatalf("pins = %+v", pins)
	}
	if pins[0].Label != "Long A" || pins[0].Color != "red" {
		t.Fatalf("pin0 = %+v", pins[0])
	}
	if pins[0].X != 200 || pins[0].Y != 300 {
		t.Fatalf("centroid = %v,%v", pins[0].X, pins[0].Y)
	}
	if pinsFromAnnotations(nil, []storage.MapAnnotation{{CalloutID: "LongA"}}) != nil {
		t.Fatal("nil set must yield no pins")
	}
	if pinsFromAnnotations(set, nil) != nil {
		t.Fatal("no annotations must yield no pins")
	}
}

func TestAnnotationColor(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range []string{"red", "green", "blue", "amber", ""} {
		col := annotationColor(c)
		if col == (ui.Color{}) {
			t.Fatalf("empty color for %q", c)
		}
		key := strings.Join([]string{fmt.Sprint(int(col.R)), fmt.Sprint(int(col.G)), fmt.Sprint(int(col.B))}, "-")
		seen[key] = true
	}
	if len(seen) < 4 {
		t.Fatalf("expected distinct colours, got %v", seen)
	}
}
