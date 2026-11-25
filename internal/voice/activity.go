package voice

import "sort"

// Activity is one continuous stretch of speech by a single speaker, in demo
// ticks. Frames closer than the join gap are merged so a sentence reads as one
// block instead of a hundred 30ms slivers.
type Activity struct {
	XUID      uint64 `json:"xuid"`
	StartTick int64  `json:"start_tick"`
	EndTick   int64  `json:"end_tick"`
	Frames    int    `json:"frames"`
}

// DefaultActivityJoinTicks is the gap (at 64 tick) below which two bursts are
// treated as the same utterance: 0.5s of silence ends a sentence.
const DefaultActivityJoinTicks int64 = 32

// BuildActivity groups voice frames into per-speaker activity blocks. Frames
// without a tick or from non-opus formats are still counted: the point is "who
// talked when", which the raw packets already answer.
func BuildActivity(frames []Frame, joinTicks int64) []Activity {
	if joinTicks <= 0 {
		joinTicks = DefaultActivityJoinTicks
	}
	byXUID := map[uint64][]Frame{}
	for _, fr := range frames {
		byXUID[fr.XUID] = append(byXUID[fr.XUID], fr)
	}
	out := make([]Activity, 0, len(byXUID))
	for xuid, list := range byXUID {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Tick < list[j].Tick })
		var cur *Activity
		for _, fr := range list {
			if cur == nil {
				cur = &Activity{XUID: xuid, StartTick: fr.Tick, EndTick: fr.Tick, Frames: 1}
				continue
			}
			if fr.Tick-cur.EndTick <= joinTicks {
				cur.EndTick = fr.Tick
				cur.Frames++
				continue
			}
			out = append(out, *cur)
			cur = &Activity{XUID: xuid, StartTick: fr.Tick, EndTick: fr.Tick, Frames: 1}
		}
		if cur != nil {
			out = append(out, *cur)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartTick != out[j].StartTick {
			return out[i].StartTick < out[j].StartTick
		}
		return out[i].XUID < out[j].XUID
	})
	return out
}

// FilterRange keeps the activity blocks that overlap [startTick, endTick].
func FilterRange(blocks []Activity, startTick, endTick int64) []Activity {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]Activity, 0, len(blocks))
	for _, b := range blocks {
		if b.EndTick < startTick || b.StartTick > endTick {
			continue
		}
		if b.StartTick < startTick {
			b.StartTick = startTick
		}
		if b.EndTick > endTick {
			b.EndTick = endTick
		}
		out = append(out, b)
	}
	return out
}

// BySpeaker groups blocks per speaker for a per-row timeline.
func BySpeaker(blocks []Activity) ([]uint64, map[uint64][]Activity) {
	rows := map[uint64][]Activity{}
	var order []uint64
	for _, b := range blocks {
		if _, ok := rows[b.XUID]; !ok {
			order = append(order, b.XUID)
		}
		rows[b.XUID] = append(rows[b.XUID], b)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	return order, rows
}
