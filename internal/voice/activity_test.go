package voice

import "testing"

func activityFrames() []Frame {
	var frames []Frame
	// Speaker 1 talks at ticks 100..104, pauses, then again at 200..202.
	for _, tick := range []int64{100, 101, 102, 103, 104} {
		frames = append(frames, Frame{Tick: tick, XUID: 1, Format: VoiceFormatOpus, Payload: []byte{1}})
	}
	for _, tick := range []int64{200, 201, 202} {
		frames = append(frames, Frame{Tick: tick, XUID: 1, Format: VoiceFormatOpus, Payload: []byte{1}})
	}
	// Speaker 2 talks once, interleaved out of order.
	frames = append(frames, Frame{Tick: 150, XUID: 2, Format: VoiceFormatOpus, Payload: []byte{1}})
	frames = append(frames, Frame{Tick: 148, XUID: 2, Format: VoiceFormatOpus, Payload: []byte{1}})
	return frames
}

func TestBuildActivityMergesBurstsAndSorts(t *testing.T) {
	blocks := BuildActivity(activityFrames(), 16)
	if len(blocks) != 3 {
		t.Fatalf("blocks = %d, want 3: %+v", len(blocks), blocks)
	}
	// Sorted by start tick across speakers.
	if blocks[0].XUID != 1 || blocks[0].StartTick != 100 || blocks[0].EndTick != 104 {
		t.Fatalf("first block = %+v", blocks[0])
	}
	if blocks[1].XUID != 2 || blocks[1].StartTick != 148 || blocks[1].EndTick != 150 || blocks[1].Frames != 2 {
		t.Fatalf("second block = %+v", blocks[1])
	}
	if blocks[2].XUID != 1 || blocks[2].StartTick != 200 || blocks[2].EndTick != 202 {
		t.Fatalf("third block = %+v", blocks[2])
	}
}

func TestBuildActivityJoinsCloseBursts(t *testing.T) {
	frames := []Frame{
		{Tick: 100, XUID: 1, Format: VoiceFormatOpus, Payload: []byte{1}},
		{Tick: 110, XUID: 1, Format: VoiceFormatOpus, Payload: []byte{1}},
	}
	if got := BuildActivity(frames, 16); len(got) != 1 || got[0].EndTick != 110 {
		t.Fatalf("close frames should merge: %+v", got)
	}
	if got := BuildActivity(frames, 4); len(got) != 2 {
		t.Fatalf("far frames should stay separate: %+v", got)
	}
	if got := BuildActivity(frames, 0); len(got) != 1 {
		t.Fatalf("zero join should fall back to the default: %+v", got)
	}
}

func TestFilterRangeAndBySpeaker(t *testing.T) {
	blocks := []Activity{
		{XUID: 1, StartTick: 50, EndTick: 120, Frames: 4},
		{XUID: 2, StartTick: 200, EndTick: 260, Frames: 3},
		{XUID: 1, StartTick: 300, EndTick: 320, Frames: 2},
	}
	got := FilterRange(blocks, 100, 250)
	if len(got) != 2 {
		t.Fatalf("filtered = %+v", got)
	}
	if got[0].StartTick != 100 || got[0].EndTick != 120 {
		t.Fatalf("clamped start not applied: %+v", got[0])
	}
	if got[1].EndTick != 250 {
		t.Fatalf("clamped end not applied: %+v", got[1])
	}
	if len(FilterRange(nil, 0, 10)) != 0 {
		t.Fatal("nil input should stay nil")
	}

	order, rows := BySpeaker(blocks)
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("order = %v", order)
	}
	if len(rows[1]) != 2 || len(rows[2]) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
}
