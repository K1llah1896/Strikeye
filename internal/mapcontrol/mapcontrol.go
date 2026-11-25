// Package mapcontrol turns navigation data plus player frames into a control
// timeline: which team could reach each piece of the map first, aggregated per
// sample and per callout.
package mapcontrol

import (
	"container/heap"
	"math"
	"sort"

	"strikeye/internal/callouts"
	"strikeye/internal/mapgeom"
	"strikeye/internal/navmesh"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

// Config tunes the control model.
type Config struct {
	// ReachRadius bounds how far a team is considered able to project control
	// from its current positions, in world units.
	ReachRadius float64
	// ContestedGap is the distance difference under which an area counts as
	// contested rather than owned.
	ContestedGap float64
	// StepSeconds is the sampling interval.
	StepSeconds float64
	// ApplyEffects removes areas covered by live smoke or fire from the control
	// classification, and stops fire from being traversed.
	ApplyEffects bool
	// SmokeRadius is the smoke sphere radius in world units.
	SmokeRadius float64
}

// DefaultConfig matches the feel of a 128-tick round: control reach of roughly
// 4.5 seconds of running, sampled every 5 seconds.
func DefaultConfig() Config {
	return Config{ReachRadius: 1100, ContestedGap: 200, StepSeconds: 5, ApplyEffects: true, SmokeRadius: 144}
}

// Frame is one sampled control state.
type Frame struct {
	Round     int     `json:"round"`
	Tick      int64   `json:"tick"`
	TimeSec   float64 `json:"time_sec"`
	OwnedT    int     `json:"owned_t"`
	OwnedCT   int     `json:"owned_ct"`
	Contested int     `json:"contested"`
	Neutral   int     `json:"neutral"`
	// Blocked counts areas hidden by smoke or cut by fire at this sample.
	Blocked int `json:"blocked"`
	// Net is (CT - T) / classified areas, in [-1, 1]; positive favours CT.
	Net float64 `json:"net"`
}

// CalloutControl summarises net control for one named area across a round.
type CalloutControl struct {
	Callout string  `json:"callout"`
	Samples int     `json:"samples"`
	AvgNet  float64 `json:"avg_net"`
	MinNet  float64 `json:"min_net"`
	MaxNet  float64 `json:"max_net"`
}

// Result is the control timeline for one demo.
type Result struct {
	Map      string           `json:"map"`
	Config   Config           `json:"config"`
	Frames   []Frame          `json:"frames"`
	Callouts []CalloutControl `json:"callouts"`
}

// Build computes control for every round in the replay index.
// Build computes control for every round in the replay index. Sample times are
// measured from freeze time, and sampling starts there so warmup ticks do not
// dominate the curve.
func Build(replays map[int]parser.ReplayRoundData, rounds []storage.MatchRound, mesh *navmesh.Mesh, set *callouts.Set, tickRate float64, cfg Config) Result {
	res := Result{Config: cfg}
	if cfg.StepSeconds <= 0 || cfg.SmokeRadius <= 0 {
		defaults := DefaultConfig()
		if cfg.StepSeconds <= 0 {
			cfg.StepSeconds = defaults.StepSeconds
		}
		if cfg.SmokeRadius <= 0 {
			cfg.SmokeRadius = defaults.SmokeRadius
		}
		res.Config = cfg
	}
	if mesh == nil || tickRate <= 0 {
		return res
	}
	stepTicks := int64(cfg.StepSeconds * tickRate)
	if stepTicks <= 0 {
		stepTicks = 1
	}

	freezeEnd := make(map[int]int64, len(rounds))
	for _, r := range rounds {
		freezeEnd[r.RoundNumber] = r.FreezeEndTick
	}

	roundNums := make([]int, 0, len(replays))
	for rn := range replays {
		roundNums = append(roundNums, rn)
	}
	sort.Ints(roundNums)

	perCallout := map[string]*CalloutControl{}
	type calloutAgg struct {
		sum, min, max float64
		n             int
	}
	aggs := map[string]*calloutAgg{}

	areaCallout := make(map[uint32]string, len(mesh.Areas))
	centroids := make(map[uint32]mapgeom.Vec3, len(mesh.Areas))
	for _, area := range mesh.Areas {
		c := area.Centroid()
		centroids[area.ID] = c
		if set != nil {
			if place, ok := set.At(c); ok {
				areaCallout[area.ID] = place.Label
			}
		}
	}

	for _, rn := range roundNums {
		rd := replays[rn]
		if len(rd.Frames) == 0 {
			continue
		}
		base := rd.Frames[0].Tick
		if fe := freezeEnd[rn]; fe > 0 {
			base = fe
		}
		for tick := base; tick <= rd.Frames[len(rd.Frames)-1].Tick; tick += stepTicks {
			frame := frameAt(rd.Frames, tick)
			blocked, cut := activeEffects(rd, frame.Tick, centroids, cfg)
			distT, distCT := teamDistances(mesh, frame.Players, cfg.ReachRadius, cut)
			fr := Frame{Round: rn, Tick: frame.Tick, TimeSec: float64(frame.Tick-base) / tickRate}
			for _, area := range mesh.Areas {
				if blocked[area.ID] {
					fr.Blocked++
					continue
				}
				dT, okT := distT[area.ID]
				dCT, okCT := distCT[area.ID]
				owner := ""
				switch {
				case okT && okCT:
					if math.Abs(dT-dCT) < cfg.ContestedGap {
						fr.Contested++
						owner = "contested"
					} else if dT < dCT {
						fr.OwnedT++
						owner = "t"
					} else {
						fr.OwnedCT++
						owner = "ct"
					}
				case okT:
					fr.OwnedT++
					owner = "t"
				case okCT:
					fr.OwnedCT++
					owner = "ct"
				default:
					fr.Neutral++
				}
				if label, ok := areaCallout[area.ID]; ok && owner != "" {
					agg := aggs[label]
					if agg == nil {
						agg = &calloutAgg{min: 1, max: -1}
						aggs[label] = agg
					}
					value := 0.0
					switch owner {
					case "ct":
						value = 1
					case "t":
						value = -1
					}
					agg.sum += value
					agg.n++
					agg.min = math.Min(agg.min, value)
					agg.max = math.Max(agg.max, value)
				}
			}
			if classified := fr.OwnedT + fr.OwnedCT + fr.Contested; classified > 0 {
				fr.Net = float64(fr.OwnedCT-fr.OwnedT) / float64(classified)
			}
			res.Frames = append(res.Frames, fr)
		}
	}

	for label, agg := range aggs {
		perCallout[label] = &CalloutControl{
			Callout: label,
			Samples: agg.n,
			AvgNet:  agg.sum / float64(agg.n),
			MinNet:  agg.min,
			MaxNet:  agg.max,
		}
	}
	for _, c := range perCallout {
		res.Callouts = append(res.Callouts, *c)
	}
	sort.Slice(res.Callouts, func(i, j int) bool {
		if res.Callouts[i].Samples != res.Callouts[j].Samples {
			return res.Callouts[i].Samples > res.Callouts[j].Samples
		}
		return res.Callouts[i].Callout < res.Callouts[j].Callout
	})
	return res
}

// teamDistances runs one multi-source Dijkstra per team, truncated at radius.
func teamDistances(mesh *navmesh.Mesh, players []parser.ReplayPlayerPose, radius float64, cut map[uint32]bool) (map[uint32]float64, map[uint32]float64) {
	var tSeeds, ctSeeds []uint32
	for _, p := range players {
		if !p.Alive {
			continue
		}
		area, ok := mesh.FindArea(mapgeom.Vec3{p.X, p.Y, p.Z})
		if !ok {
			area, ok = nearestArea(mesh, mapgeom.Vec3{p.X, p.Y, p.Z}, 400)
			if !ok {
				continue
			}
		}
		switch p.Team {
		case 1:
			tSeeds = append(tSeeds, area)
		case 2:
			ctSeeds = append(ctSeeds, area)
		}
	}
	return dijkstra(mesh, tSeeds, radius, cut), dijkstra(mesh, ctSeeds, radius, cut)
}

// activeEffects returns the areas hidden by live smoke and the areas cut off by
// live fire at a given tick. Smoke only blinds (ownership is skipped), fire also
// stops movement, so it is returned separately.
func activeEffects(rd parser.ReplayRoundData, tick int64, centroids map[uint32]mapgeom.Vec3, cfg Config) (map[uint32]bool, map[uint32]bool) {
	blocked := map[uint32]bool{}
	cut := map[uint32]bool{}
	if !cfg.ApplyEffects {
		return blocked, cut
	}
	r2 := cfg.SmokeRadius * cfg.SmokeRadius
	for _, smoke := range rd.Smokes {
		if tick < smoke.StartTick || (smoke.EndTick > 0 && tick > smoke.EndTick) {
			continue
		}
		for id, c := range centroids {
			dx := float64(c[0] - smoke.Pos.X)
			dy := float64(c[1] - smoke.Pos.Y)
			dz := float64(c[2] - smoke.Pos.Z)
			if dx*dx+dy*dy <= r2 && math.Abs(dz) <= cfg.SmokeRadius {
				blocked[id] = true
			}
		}
	}
	for _, inf := range rd.Infernos {
		if tick < inf.StartTick || (inf.EndTick > 0 && tick > inf.EndTick) {
			continue
		}
		if len(inf.Hull) < 3 {
			continue
		}
		for id, c := range centroids {
			if pointInHull(c, inf.Hull) {
				blocked[id] = true
				cut[id] = true
			}
		}
	}
	return blocked, cut
}

// pointInHull does a 2D ray cast of the area centroid against the fire hull.
func pointInHull(c mapgeom.Vec3, hull []parser.Point2D) bool {
	inside := false
	x, y := c[0], c[1]
	j := len(hull) - 1
	for i := 0; i < len(hull); i++ {
		xi, yi := hull[i].X, hull[i].Y
		xj, yj := hull[j].X, hull[j].Y
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}

func nearestArea(mesh *navmesh.Mesh, p mapgeom.Vec3, maxDist float64) (uint32, bool) {
	best := uint32(0)
	bestD := maxDist * maxDist
	for _, area := range mesh.Areas {
		c := area.Centroid()
		dx := float64(p[0] - c[0])
		dy := float64(p[1] - c[1])
		dz := float64(p[2] - c[2])
		d := dx*dx + dy*dy + dz*dz
		if d <= bestD {
			best, bestD = area.ID, d
		}
	}
	return best, best > 0
}

type hop struct {
	id   uint32
	cost float64
}

type costQueue []hop

func (q costQueue) Len() int           { return len(q) }
func (q costQueue) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q costQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *costQueue) Push(x any)        { *q = append(*q, x.(hop)) }
func (q *costQueue) Pop() any {
	old := *q
	item := old[len(old)-1]
	*q = old[:len(old)-1]
	return item
}

func dijkstra(mesh *navmesh.Mesh, seeds []uint32, radius float64, cut map[uint32]bool) map[uint32]float64 {
	out := make(map[uint32]float64)
	if len(seeds) == 0 {
		return out
	}
	q := &costQueue{}
	for _, id := range seeds {
		if _, ok := out[id]; ok {
			continue
		}
		out[id] = 0
		*q = append(*q, hop{id: id, cost: 0})
	}
	heap.Init(q)
	for q.Len() > 0 {
		cur := heap.Pop(q).(hop)
		if cur.cost > out[cur.id] {
			continue
		}
		src, ok := mesh.Area(cur.id)
		if !ok {
			continue
		}
		c := src.Centroid()
		for _, next := range src.Neighbors {
			if cut[next] {
				continue
			}
			dst, ok := mesh.Area(next)
			if !ok {
				continue
			}
			n := dst.Centroid()
			step := math.Sqrt(
				sq(float64(n[0]-c[0])) + sq(float64(n[1]-c[1])) + sq(float64(n[2]-c[2])),
			)
			nextCost := cur.cost + step
			if nextCost > radius {
				continue
			}
			if prev, ok := out[next]; ok && prev <= nextCost {
				continue
			}
			out[next] = nextCost
			heap.Push(q, hop{id: next, cost: nextCost})
		}
	}
	return out
}

func sq(v float64) float64 { return v * v }

func frameAt(frames []parser.ReplayFrame, tick int64) parser.ReplayFrame {
	if len(frames) == 0 {
		return parser.ReplayFrame{}
	}
	idx := sort.Search(len(frames), func(i int) bool { return frames[i].Tick >= tick })
	if idx >= len(frames) {
		return frames[len(frames)-1]
	}
	if idx > 0 && tick-frames[idx-1].Tick < frames[idx].Tick-tick {
		return frames[idx-1]
	}
	return frames[idx]
}
