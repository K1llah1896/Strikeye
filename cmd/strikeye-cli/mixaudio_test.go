package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func makeClip(t *testing.T, path string, dur float64, withAudio bool) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=black:s=160x90:r=15:d=%.3f", dur)}
	if withAudio {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%.3f:sample_rate=48000", dur),
			"-map", "0:v", "-map", "1:a", "-ac", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-shortest")
		if strings.HasSuffix(path, ".wav") {
			args = []string{"-y", "-f", "lavfi", "-i",
				fmt.Sprintf("sine=frequency=880:duration=%.3f:sample_rate=48000", dur), "-ac", "2"}
		}
	} else {
		args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p")
	}
	if out, err := exec.Command(ffmpeg, append(args, path)...).CombinedOutput(); err != nil {
		t.Fatalf("make clip: %v: %s", err, out)
	}
}

func TestRunMixAudioProducesMixedClipAndAudioOnly(t *testing.T) {
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	makeClip(t, clip, 3, true)
	comms := filepath.Join(dir, "comms.wav")
	makeClip(t, comms, 4, true)

	outPath := filepath.Join(dir, "final.mp4")
	var buf bytes.Buffer
	if err := runMixAudio(context.Background(), mixaudioOptions{
		videoPath:  clip,
		trackSpecs: []string{comms + "@0.5:0.6"},
		outPath:    outPath,
		audioOnly:  true,
	}, &buf); err != nil {
		t.Fatalf("mix: %v", err)
	}
	var got struct {
		Output    string  `json:"output"`
		AudioOnly string  `json:"audio_only"`
		Tracks    int     `json:"tracks"`
		Duration  float64 `json:"duration_sec"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, buf.String())
	}
	if got.Tracks != 1 || got.Duration < 2.5 || got.Duration > 3.5 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Output != outPath {
		t.Fatalf("output = %s, want %s", got.Output, outPath)
	}
	if !strings.HasSuffix(got.AudioOnly, ".m4a") {
		t.Fatalf("audio-only naming: %s", got.AudioOnly)
	}

	muted := filepath.Join(dir, "mute.json")
	if err := runMixAudio(context.Background(), mixaudioOptions{
		videoPath:  clip,
		trackSpecs: []string{comms},
		outPath:    muted,
	}, &buf); err == nil {
		t.Fatal("expected failure for a non-media output path")
	}

	if err := runMixAudio(context.Background(), mixaudioOptions{videoPath: clip}, &buf); err == nil {
		t.Fatal("expected error without tracks")
	}
	if err := runMixAudio(context.Background(), mixaudioOptions{trackSpecs: []string{comms}}, &buf); err == nil {
		t.Fatal("expected error without a video")
	}
	if err := runMixAudio(context.Background(), mixaudioOptions{
		videoPath: clip, trackSpecs: []string{"@1.0"},
	}, &buf); err == nil {
		t.Fatal("expected error for a malformed track spec")
	}
}
