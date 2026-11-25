package voice

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func sample() []Frame {
	return []Frame{
		{Tick: 100, XUID: 76561198000000001, Format: 0, SampleRate: 24000, Sequence: 12, Section: 1, Offset: 960, Payload: []byte{1, 2, 3, 4}},
		{Tick: 132, XUID: 76561198000000002, Format: 0, SampleRate: 24000, Sequence: 3, Section: 2, Offset: 1920, Payload: bytes.Repeat([]byte{9}, 40)},
		{Tick: 164, XUID: 76561198000000001, Format: 1, SampleRate: 48000, Sequence: 13, Section: 3, Offset: 0, Payload: []byte{7}},
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	frames := sample()
	for _, fr := range frames {
		if err := WriteFrame(&buf, fr); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	// Prefix the header the way Extract does.
	var withHeader bytes.Buffer
	withHeader.Write(magic[:])
	mustWriteU16(&withHeader, FormatVersion)
	withHeader.Write(buf.Bytes())

	got, err := ReadFrames(&withHeader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != len(frames) {
		t.Fatalf("frames = %d", len(got))
	}
	for i := range frames {
		if got[i].Tick != frames[i].Tick || got[i].XUID != frames[i].XUID {
			t.Fatalf("frame %d header mismatch: %+v", i, got[i])
		}
		if got[i].SampleRate != frames[i].SampleRate || got[i].Offset != frames[i].Offset {
			t.Fatalf("frame %d audio meta mismatch: %+v", i, got[i])
		}
		if !bytes.Equal(got[i].Payload, frames[i].Payload) {
			t.Fatalf("frame %d payload mismatch", i)
		}
	}
}

func TestReadFramesRejectsForeignData(t *testing.T) {
	if _, err := ReadFrames(bytes.NewReader([]byte("not a capture"))); !errors.Is(err, ErrInvalidContainer) {
		t.Fatalf("err = %v", err)
	}
	bad := append([]byte{}, magic[:]...)
	bad = append(bad, 0x09, 0x00)
	if _, err := ReadFrames(bytes.NewReader(bad)); !errors.Is(err, ErrInvalidContainer) {
		t.Fatalf("version err = %v", err)
	}
}

func TestReadFramesRejectsHugePayload(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(magic[:])
	mustWriteU16(&buf, FormatVersion)
	hdr := make([]byte, 31)
	buf.Write(hdr)
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	if _, err := ReadFrames(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected length guard to fire")
	}
}

func TestWriteFramePayloadLimit(t *testing.T) {
	var buf bytes.Buffer
	err := WriteFrame(&buf, Frame{Payload: make([]byte, maxPayload+1)})
	if err == nil {
		t.Fatal("expected payload limit error")
	}
}

func TestExtractDemoWithoutVoice(t *testing.T) {
	// The bundled fixtures were recorded without voice: extraction must succeed
	// and report zero speakers rather than failing.
	var buf bytes.Buffer
	sum, err := Extract(t.Context(), "../../tests/furia-vs-falcons-m2-dust2.dem", &buf)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if sum.Frames != 0 || len(sum.Speakers) != 0 || sum.HasVoice {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	if buf.Len() != 6 {
		t.Fatalf("container header = %d bytes", buf.Len())
	}
	if _, err := ReadFrames(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("read empty container: %v", err)
	}
}

func mustWriteU16(w *bytes.Buffer, v uint16) {
	w.Write([]byte{byte(v), byte(v >> 8)})
}

func mustWriteU32(w *bytes.Buffer, v uint32) {
	w.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
}

// TestReadLegacyV1Container checks that captures written before the offset field
// was widened still load, with the 16-bit offset preserved.
func TestReadLegacyV1Container(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(magic[:])
	mustWriteU16(&buf, legacyFormatVersion)
	payload := []byte{1, 2, 3}
	var hdr [legacyFrameHeaderSize]byte
	binary.LittleEndian.PutUint64(hdr[0:], 4242)
	binary.LittleEndian.PutUint64(hdr[8:], 76561198000000077)
	hdr[16] = 2
	binary.LittleEndian.PutUint32(hdr[17:], 48000)
	binary.LittleEndian.PutUint32(hdr[21:], 9)
	binary.LittleEndian.PutUint32(hdr[25:], 3)
	binary.LittleEndian.PutUint16(hdr[29:], 65535)
	buf.Write(hdr[:])
	mustWriteU32(&buf, uint32(len(payload)))
	buf.Write(payload)

	frames, err := ReadFrames(&buf)
	if err != nil {
		t.Fatalf("read legacy: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("frames = %d", len(frames))
	}
	if frames[0].Offset != 65535 || frames[0].Tick != 4242 || frames[0].Section != 3 {
		t.Fatalf("legacy frame = %+v", frames[0])
	}
}

// TestOffsetBeyondLegacyRangeRoundTrips guards the bug where a 16-bit offset
// wrapped every 1.37s and scrambled long utterances.
func TestOffsetBeyondLegacyRangeRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	if err := NewContainerWriter(&buf); err != nil {
		t.Fatalf("header: %v", err)
	}
	want := Frame{Tick: 1, XUID: 5, Format: VoiceFormatOpus, SampleRate: 48000, Section: 2, Offset: 3_000_000, Payload: []byte{9}}
	if err := WriteFrame(&buf, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	frames, err := ReadFrames(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(frames) != 1 || frames[0].Offset != want.Offset {
		t.Fatalf("offset = %d, want %d", frames[0].Offset, want.Offset)
	}
}
