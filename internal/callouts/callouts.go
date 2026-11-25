// Package callouts resolves CS2 world positions to in-game place names.
//
// CS2 no longer stores place names inside .nav files (CS:GO did); they live in
// env_cs_place entities. Strikeye therefore reads the community-compiled world
// table published by sink3x/cs2-server-admin-callouts (MIT), which lists one
// axis-aligned box per place name per map.
package callouts

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"strikeye/internal/mapgeom"
)

// ErrUnknownMap reports a lookup for a map the table does not cover.
var ErrUnknownMap = errors.New("callout table has no data for map")

// Place is one named region of a map.
type Place struct {
	ID    string
	Label string
	Min   mapgeom.Vec3
	Max   mapgeom.Vec3
	Area  float64
}

// Centroid returns the box centre.
func (p Place) Centroid() mapgeom.Vec3 {
	return mapgeom.Vec3{
		(p.Min[0] + p.Max[0]) / 2,
		(p.Min[1] + p.Max[1]) / 2,
		(p.Min[2] + p.Max[2]) / 2,
	}
}

// contains 判断 point 是否落在盒内；Z 允许 48 单位容差，因为站位常略高于地面。
func (p Place) contains(point mapgeom.Vec3) bool {
	const zTolerance float32 = 48
	return point[0] >= p.Min[0] && point[0] <= p.Max[0] &&
		point[1] >= p.Min[1] && point[1] <= p.Max[1] &&
		point[2] >= p.Min[2]-zTolerance && point[2] <= p.Max[2]+zTolerance
}

// Set is the callout table of one map.
type Set struct {
	MapName string
	places  []Place
}

// Table holds the callout sets of every map in the dataset.
type Table struct {
	sets map[string]*Set
}

// LoadCSV reads the nav_places.csv table (columns map,name,x1,y1,z1,x2,y2,z2).
func LoadCSV(path string) (*Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	t, err := ParseCSV(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return t, nil
}

// ParseCSV reads the callout table from r.
func ParseCSV(r io.Reader) (*Table, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	index := map[string]int{}
	for i, name := range header {
		index[strings.ToLower(strings.TrimSpace(name))] = i
	}
	required := []string{"map", "name", "x1", "y1", "z1", "x2", "y2", "z2"}
	cols := make([]int, len(required))
	for i, name := range required {
		col, ok := index[name]
		if !ok {
			return nil, fmt.Errorf("missing column %q", name)
		}
		cols[i] = col
	}
	t := &Table{sets: map[string]*Set{}}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		field := func(i int) string {
			if cols[i] >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[cols[i]])
		}
		mapName := strings.ToLower(field(0))
		id := field(1)
		if mapName == "" || id == "" {
			continue
		}
		var v [6]float64
		ok := true
		for i := 0; i < 6; i++ {
			n, err := parseFloat(field(2 + i))
			if err != nil {
				ok = false
				break
			}
			v[i] = n
		}
		if !ok {
			continue
		}
		min := mapgeom.Vec3{float32(math.Min(v[0], v[3])), float32(math.Min(v[1], v[4])), float32(math.Min(v[2], v[5]))}
		max := mapgeom.Vec3{float32(math.Max(v[0], v[3])), float32(math.Max(v[1], v[4])), float32(math.Max(v[2], v[5]))}
		set := t.sets[mapName]
		if set == nil {
			set = &Set{MapName: mapName}
			t.sets[mapName] = set
		}
		set.places = append(set.places, Place{
			ID:    id,
			Label: Label(id),
			Min:   min,
			Max:   max,
			Area:  float64(max[0]-min[0]) * float64(max[1]-min[1]),
		})
	}
	return t, nil
}

func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	return strconv.ParseFloat(s, 64)
}

// For returns the callout set for a map (case-insensitive, .dem/.nav suffixes
// tolerated), or ErrUnknownMap.
func (t *Table) For(mapName string) (*Set, error) {
	name := strings.ToLower(strings.TrimSpace(mapName))
	for _, suffix := range []string{".dem", ".nav"} {
		name = strings.TrimSuffix(name, suffix)
	}
	if set, ok := t.sets[name]; ok {
		return set, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownMap, mapName)
}

// Maps lists the covered maps, sorted.
func (t *Table) Maps() []string {
	names := make([]string, 0, len(t.sets))
	for name := range t.sets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Places returns a copy of the set's places sorted by label.
func (s *Set) Places() []Place {
	out := append([]Place(nil), s.places...)
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// Len returns the number of places.
func (s *Set) Len() int { return len(s.places) }

// At resolves a world position to a place. The smallest box containing the
// point wins (overlapping coarse areas contain finer ones); when no box
// contains it, the nearest box centre within 512 units is used so points in
// doorways still get a name.
// snapRadius bounds how far a point may be from a callout centre and still be
// attributed to it.
const snapRadius = 512

func (s *Set) At(point mapgeom.Vec3) (Place, bool) {
	match, ok := s.Lookup(point)
	return match.Place, ok
}

// Match reports how a point was attributed: whether it fell inside a callout
// box or was snapped to the nearest box centre, and the distance in units.
type Match struct {
	Place    Place
	Exact    bool
	Distance float64
}

// Lookup attributes a point and reports whether the attribution is exact.
// Exact hits are direct box containment; everything else is a snap within
// snapRadius, which callers can treat as lower confidence.
func (s *Set) Lookup(point mapgeom.Vec3) (Match, bool) {
	var best Place
	bestArea := math.Inf(1)
	for _, p := range s.places {
		if p.contains(point) && p.Area < bestArea {
			best, bestArea = p, p.Area
		}
	}
	if !math.IsInf(bestArea, 1) {
		return Match{Place: best, Exact: true}, true
	}
	bestDist := math.Inf(1)
	for _, p := range s.places {
		c := p.Centroid()
		dx := float64(point[0] - c[0])
		dy := float64(point[1] - c[1])
		dz := float64(point[2] - c[2])
		d := dx*dx + dy*dy + dz*dz
		if d < bestDist {
			best, bestDist = p, d
		}
	}
	if bestDist <= snapRadius*snapRadius {
		return Match{Place: best, Distance: math.Sqrt(bestDist)}, true
	}
	return Match{}, false
}

// Find returns the place with the given ID.
func (s *Set) Find(id string) (Place, bool) {
	for _, p := range s.places {
		if p.ID == id {
			return p, true
		}
	}
	return Place{}, false
}

// specialLabels maps the engine's camel-case place IDs to how players say them.
var specialLabels = map[string]string{
	"BombsiteA":     "A Site",
	"BombsiteB":     "B Site",
	"CTSpawn":       "CT Spawn",
	"TSpawn":        "T Spawn",
	"BackofA":       "Back of A",
	"BackofB":       "Back of B",
	"TopofMid":      "Top of Mid",
	"TunnelsA":      "Tunnels",
	"ARamp":         "A Ramp",
	"BDoors":        "B Doors",
	"ExtendedA":     "Extended A",
	"APlatform":     "A Platform",
	"BPlatform":     "B Platform",
	"CTSideUpper":   "CT Side Upper",
	"TRamp":         "T Ramp",
	"OutsideTunnel": "Outside Tunnel",
	"LongDoors":     "Long Doors",
	"MidDoors":      "Mid Doors",
	"ShortStairs":   "Short Stairs",
	"LowerTunnel":   "Lower Tunnel",
	"UpperTunnel":   "Upper Tunnel",
	"UnderA":        "Under A",
	"UnderB":        "Under B",
	"OutsideLong":   "Outside Long",
	"LongA":         "Long A",
	"LongB":         "Long B",
	"Hole":          "Hole",
}

// Label turns an engine place ID into a display name.
func Label(id string) string {
	if label, ok := specialLabels[id]; ok {
		return label
	}
	var b strings.Builder
	runes := []rune(id)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			if prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9' || (prev >= 'A' && prev <= 'Z' && nextLower) {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
