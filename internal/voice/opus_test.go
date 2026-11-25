package voice

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// makeOpusOgg renders a sine tone to Ogg/Opus with ffmpeg so the decoder can be
// exercised against real packets without shipping binary fixtures.
func makeOpusOgg(t *testing.T, dir string, seconds float64) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	path := filepath.Join(dir, "tone.ogg")
	cmd := exec.Command(ffmpeg, "-y", "-f", "lavfi", "-i",
		"sine=frequency=440:duration="+strconv.FormatFloat(seconds, 'f', 2, 64)+":sample_rate=48000",
		"-c:a", "libopus", "-b:a", "48k", "-ac", "1", "-ar", "48000", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("encode opus: %v: %s", err, out)
	}
	return path
}

func TestDecodeOpusMixesSpeakersAndWritesWAV(t *testing.T) {
	dir := t.TempDir()
	ogg := makeOpusOgg(t, dir, 2)
	raw, err := os.ReadFile(ogg)
	if err != nil {
		t.Fatalf("read ogg: %v", err)
	}
	packets, err := ParseXOggPackets(raw)
	if err != nil {
		t.Fatalf("parse ogg: %v", err)
	}
	if len(packets) < 2 {
		t.Fatalf("expected several opus packets, got %d", len(packets))
	}

	frames := make([]Frame, 0, len(packets)+1)
	for i, p := range packets {
		frames = append(frames, Frame{
			Tick: int64(i), XUID: 111, Format: VoiceFormatOpus, SampleRate: 48000,
			Section: 1, Offset: uint32(i * 960), Payload: p,
		})
	}
	// A second speaker gets interleaved to prove the mixer keeps both.
	frames = append(frames, Frame{
		Tick: 1, XUID: 222, Format: VoiceFormatOpus, SampleRate: 48000,
		Section: 1, Offset: 0, Payload: packets[0],
	})
	// Opposite format is skipped rather than failing the whole run.
	frames = append(frames, Frame{Tick: 2, XUID: 333, Format: 0, Payload: []byte{1, 2, 3}})

	var wav bytes.Buffer
	sum, err := DecodeOpus(frames, &wav)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sum.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", sum.Skipped)
	}
	if len(sum.Speakers) != 2 {
		t.Fatalf("speakers = %d, want 2", len(sum.Speakers))
	}
	for _, sp := range sum.Speakers {
		if sp.Samples <= 0 || sp.Seconds <= 0 {
			t.Fatalf("speaker %d produced no samples: %+v", sp.XUID, sp)
		}
	}

	got := wav.Bytes()
	if len(got) < 44 || string(got[:4]) != "RIFF" || string(got[8:12]) != "WAVE" {
		t.Fatalf("not a RIFF/WAVE payload (%d bytes)", len(got))
	}
	dataLen := int(binary.LittleEndian.Uint32(got[40:44]))
	if dataLen+44 != len(got) {
		t.Fatalf("header/length mismatch: data=%d total=%d", dataLen, len(got))
	}
	// The mixed track follows the demo timeline, so it is at least as long as the
	// longest single speaker (overlapping speech is summed, not concatenated).
	longest := 0
	for _, sp := range sum.Speakers {
		if sp.Samples > longest {
			longest = sp.Samples
		}
	}
	if dataLen < longest*2 {
		t.Fatalf("mixed track shorter than longest speaker: data=%d longest=%d", dataLen, longest)
	}
	if rate := binary.LittleEndian.Uint32(got[24:28]); rate != 48000 {
		t.Fatalf("sample rate = %d", rate)
	}
	// 2s source plus one extra packet decodes to roughly 2-2.1s per speaker.
	secs := float64(sum.Samples) / 48000
	if secs < 1.8 || secs > 2.3 {
		t.Fatalf("decoded %.2fs of speaker audio, expected ~2s", secs)
	}
}

func TestDecodeOpusRejectsNonOpusFrames(t *testing.T) {
	_, err := DecodeOpus([]Frame{{Format: 0, Payload: []byte{1, 2, 3}}}, &bytes.Buffer{})
	if err != ErrNoOpusFrames {
		t.Fatalf("err = %v, want ErrNoOpusFrames", err)
	}
	if _, err := DecodeOpus(nil, &bytes.Buffer{}); err != ErrNoOpusFrames {
		t.Fatalf("nil frames err = %v", err)
	}
}

func TestParseXOggPacketsRejectsGarbage(t *testing.T) {
	if _, err := ParseXOggPackets([]byte("not ogg at all")); err == nil {
		t.Fatal("expected error for non-ogg input")
	}
	if _, err := ParseXOggPackets([]byte("OggSgarbage")); err == nil {
		t.Fatal("expected error for bad page")
	}
}

func TestDecodeOpusDirWritesSpeakerTracks(t *testing.T) {
	dir := t.TempDir()
	ogg := makeOpusOgg(t, dir, 1)
	raw, err := os.ReadFile(ogg)
	if err != nil {
		t.Fatalf("read ogg: %v", err)
	}
	packets, err := ParseXOggPackets(raw)
	if err != nil {
		t.Fatalf("parse ogg: %v", err)
	}
	var frames []Frame
	for i, p := range packets {
		frames = append(frames, Frame{
			Tick: int64(i), XUID: 76561198000000011, Format: VoiceFormatOpus, SampleRate: 48000,
			Section: 1, Offset: uint32(i * 960), Payload: p,
		})
	}
	// A second speaker starts 1s later on the same timeline.
	for i, p := range packets {
		if i > 5 {
			break
		}
		frames = append(frames, Frame{
			Tick: int64(i), XUID: 76561198000000022, Format: VoiceFormatOpus, SampleRate: 48000,
			Section: 1, Offset: uint32(48000 + i*960), Payload: p,
		})
	}

	outDir := filepath.Join(dir, "tracks")
	sum, tracks, err := DecodeOpusDir(frames, outDir)
	if err != nil {
		t.Fatalf("decode dir: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("tracks = %d, want 2", len(tracks))
	}
	for _, tr := range tracks {
		info, err := os.Stat(tr.File)
		if err != nil || info.Size() <= 44 {
			t.Fatalf("track %s missing: %v", tr.File, err)
		}
		if tr.Seconds <= 0 || tr.Frames == 0 {
			t.Fatalf("track %+v empty", tr)
		}
	}
	mixed, err := os.Stat(filepath.Join(outDir, "mixed.wav"))
	if err != nil || mixed.Size() <= 44 {
		t.Fatalf("mixed track missing: %v", err)
	}
	if sum.Frames != len(packets)+6 {
		t.Fatalf("frames = %d, want %d", sum.Frames, len(packets)+6)
	}
}
