// Package zones turns decoded replay frames into callout-level summaries: where
// each team stands during the default setup, and which named area each death
// happened in.
package zones

import (
	"sort"

	"strikeye/internal/callouts"
	"strikeye/internal/mapgeom"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

// SetupSampleSeconds is how long after freeze time the default setup is sampled.
const SetupSampleSeconds = 20.0

// SetupTotals aggregates setup positions across rounds.
type SetupTotals struct {
	Rounds  int
	Sampled int // player positions resolved this round set
	// T, CT and All count sampled players per callout label.
	T   map[string]int
	CT  map[string]int
	All map[string]int
	// Unnamed counts sampled players that fell outside every callout box.
	Unnamed int
	// Exact counts samples inside a callout box; Snapped counts samples that
	// needed the nearest-centre fallback (lower confidence).
	Exact       int
	Snapped     int
	SnapDistSum float64
	// IDs maps a callout label to the first place id seen for it, so callers can
	// attach annotations to labels.
	IDs map[string]string
}

// Row is one callout's setup counts, used for table rendering.
type Row struct {
	Callout string
	PlaceID string
	T       int
	CT      int
	Total   int
}

// Rows returns the per-callout counts sorted by total then label.
func (s SetupTotals) Rows() []Row {
	rows := make([]Row, 0, len(s.All))
	for name, total := range s.All {
		rows = append(rows, Row{Callout: name, PlaceID: s.IDs[name], T: s.T[name], CT: s.CT[name], Total: total})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Total != rows[j].Total {
			return rows[i].Total > rows[j].Total
		}
		return rows[i].Callout < rows[j].Callout
	})
	return rows
}

// Death is one kill located by callout.
type Death struct {
	Round    int
	Tick     int64
	TimeSec  float64
	Killer   string
	Victim   string
	Headshot bool
	Callout  string
	Found    bool
	// Exact is false when the victim was outside callout boxes and snapped to
	// the nearest callout centre.
	Exact bool
}

// Setup samples every round at freeze time + 20s and bins alive players by
// callout. Round replays missing from the index are skipped.
func Setup(replays map[int]parser.ReplayRoundData, rounds []storage.MatchRound, tickRate float64, set *callouts.Set) SetupTotals {
	totals := SetupTotals{T: map[string]int{}, CT: map[string]int{}, All: map[string]int{}, IDs: map[string]string{}}
	if set == nil || tickRate <= 0 || len(replays) == 0 {
		return totals
	}
	sampleTicks := int64(SetupSampleSeconds * tickRate)
	freezeEnd := make(map[int]int64, len(rounds))
	for _, r := range rounds {
		freezeEnd[r.RoundNumber] = r.FreezeEndTick
	}
	roundNums := make([]int, 0, len(replays))
	for rn := range replays {
		roundNums = append(roundNums, rn)
	}
	sort.Ints(roundNums)
	for _, rn := range roundNums {
		rd := replays[rn]
		if len(rd.Frames) == 0 {
			continue
		}
		target := rd.Frames[0].Tick + sampleTicks
		if fe := freezeEnd[rn]; fe > 0 {
			target = fe + sampleTicks
		}
		frame := frameAt(rd.Frames, target)
		roundSampled := false
		for _, p := range frame.Players {
			if !p.Alive || (p.Team != 1 && p.Team != 2) {
				continue
			}
			totals.Sampled++
			roundSampled = true
			match, ok := set.Lookup(parserVec(p))
			if !ok {
				totals.Unnamed++
				continue
			}
			if _, seen := totals.IDs[match.Place.Label]; !seen && match.Place.ID != "" {
				totals.IDs[match.Place.Label] = match.Place.ID
			}
			place := match.Place
			if match.Exact {
				totals.Exact++
			} else {
				totals.Snapped++
				totals.SnapDistSum += match.Distance
			}
			totals.All[place.Label]++
			if p.Team == 1 {
				totals.T[place.Label]++
			} else {
				totals.CT[place.Label]++
			}
		}
		if roundSampled {
			totals.Rounds++
		}
	}
	return totals
}

// Deaths resolves every recorded kill to the callout its victim fell in. Kill
// times are measured from freeze time, matching the in-game round clock.
func Deaths(replays map[int]parser.ReplayRoundData, rounds []storage.MatchRound, tickRate float64, set *callouts.Set, slotNames map[uint8]string) []Death {
	if len(replays) == 0 {
		return nil
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
	var out []Death
	for _, rn := range roundNums {
		rd := replays[rn]
		deaths := append([]parser.ReplayDeathEvent(nil), rd.Deaths...)
		sort.Slice(deaths, func(i, j int) bool { return deaths[i].Tick < deaths[j].Tick })
		for _, d := range deaths {
			entry := Death{
				Round:    rn,
				Tick:     d.Tick,
				Killer:   slotNames[d.KillerSlot],
				Victim:   slotNames[d.VictimSlot],
				Headshot: d.Headshot,
			}
			base := rd.StartTick
			if fe := freezeEnd[rn]; fe > 0 {
				base = fe
			}
			if tickRate > 0 && d.Tick > base {
				entry.TimeSec = float64(d.Tick-base) / tickRate
			}
			if set != nil {
				if match, ok := set.Lookup(mapgeom.Vec3{d.Victim.X, d.Victim.Y, d.Victim.Z}); ok {
					entry.Callout, entry.Found, entry.Exact = match.Place.Label, true, match.Exact
				}
			}
			out = append(out, entry)
		}
	}
	return out
}

// frameAt returns the frame closest to tick.
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

func parserVec(p parser.ReplayPlayerPose) mapgeom.Vec3 {
	return mapgeom.Vec3{p.X, p.Y, p.Z}
}
