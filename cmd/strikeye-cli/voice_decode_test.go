package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"strikeye/internal/voice"
)

// writeSyntheticCapture builds a .cs2voice container from real Opus packets so
// the built-in decoder can be verified end to end without a voiced demo.
func writeSyntheticCapture(t *testing.T, dataRoot string, demoID int64) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	ogg := filepath.Join(dir, "tone.ogg")
	if out, err := exec.Command(ffmpeg, "-y", "-f", "lavfi", "-i",
		"sine=frequency=500:duration=1.00:sample_rate=48000",
		"-c:a", "libopus", "-b:a", "48k", "-ac", "1", ogg).CombinedOutput(); err != nil {
		t.Fatalf("encode: %v: %s", err, out)
	}
	raw, err := os.ReadFile(ogg)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	packets, err := voice.ParseXOggPackets(raw)
	if err != nil || len(packets) == 0 {
		t.Fatalf("parse ogg: %v (%d packets)", err, len(packets))
	}

	containerDir := filepath.Join(dataRoot, "media", "voice")
	if err := os.MkdirAll(containerDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(containerDir, fmt.Sprintf("demo_%d.cs2voice", demoID))
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := voice.NewContainerWriter(f); err != nil {
		t.Fatalf("write header: %v", err)
	}
	for i, p := range packets {
		if err := voice.WriteFrame(f, voice.Frame{
			Tick: int64(i), XUID: 76561198000000001, Format: voice.VoiceFormatOpus,
			SampleRate: 48000, Section: 1, Offset: uint32(i * 960), Payload: p,
		}); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	}
	return path
}

func TestRunVoiceDecodeBuiltinOpus(t *testing.T) {
	dataRoot := t.TempDir()
	writeSyntheticCapture(t, dataRoot, 7)

	var buf bytes.Buffer
	if err := runVoiceDecode(context.Background(), dataRoot, 7, "", "", &buf); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var got struct {
		DemoID   int64   `json:"demo_id"`
		Engine   string  `json:"engine"`
		File     string  `json:"file"`
		Frames   int     `json:"frames"`
		Seconds  float64 `json:"seconds"`
		HasData  bool    `json:"has_voice"`
		Speakers []struct {
			XUID   uint64 `json:"xuid"`
			Frames int    `json:"frames"`
		} `json:"speakers"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode json: %v (%s)", err, buf.String())
	}
	if got.Engine != "opus" || !got.HasData || got.Frames < 2 || len(got.Speakers) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Seconds < 0.8 || got.Seconds > 1.3 {
		t.Fatalf("decoded %.2fs, expected ~1s", got.Seconds)
	}
	info, err := os.Stat(got.File)
	if err != nil || info.Size() < 44 {
		t.Fatalf("wav missing: %v", err)
	}
}

func TestRunVoiceDecodeWithoutCaptureFails(t *testing.T) {
	dataRoot := t.TempDir()
	var buf bytes.Buffer
	err := runVoiceDecode(context.Background(), dataRoot, 3, "", "", &buf)
	if err == nil {
		t.Fatal("expected error when the capture does not exist")
	}
}
