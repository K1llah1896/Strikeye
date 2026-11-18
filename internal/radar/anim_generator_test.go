package radar

import (
	"context"
	"encoding/json"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestScaleAtMonotonicAndFreeze(t *testing.T) {
	prev := -1.0
	for i := 0; i <= 1000; i++ {
		tt := float64(i) / 1000.0
		s := ScaleAt(tt)
		if s < prev-1e-12 {
			t.Fatalf("scale decreased at t=%v: prev=%v cur=%v", tt, prev, s)
		}
		if s > 1.0+1e-12 {
			t.Fatalf("scale exceeded 1.0 at t=%v: %v", tt, s)
		}
		prev = s
	}
	if !almostEqual(ScaleAt(0.0), 0.30, 1e-6) {
		t.Fatalf("expected ScaleAt(0)=0.30, got %v", ScaleAt(0.0))
	}
	if !almostEqual(ScaleAt(0.30), 0.55, 1e-6) {
		t.Fatalf("expected ScaleAt(0.30)=0.55, got %v", ScaleAt(0.30))
	}
	if !almostEqual(ScaleAt(0.60), 1.00, 1e-6) || !almostEqual(ScaleAt(1.00), 1.00, 1e-6) {
		t.Fatalf("expected ScaleAt(>=0.60)=1.00, got %v / %v", ScaleAt(0.60), ScaleAt(1.00))
	}
}

func TestRenderAnimationMP4AndAppearanceSync(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}

	tmp := t.TempDir()
	raw := RawValues{KPR: 0.9, Surviving: 0.45, ADR: 92, KAST: 0.80, MultiKill: 6, Rating: 1.28}
	in := CardInput{
		PlayerName: "AnimPlayer",
		TeamKey:    "CT",
		TeamLabel:  "STRIKEYE",
		Kills:      21,
		Deaths:     14,
		Assists:    5,
		Raw:        raw,
		Normalized: Normalize(raw),
		MatchAvg:   NormalizedValues{KPR: 0.75, Surviving: 0.75, ADR: 0.75, KAST: 0.75, MultiKill: 0.6, Rating: 0.75},
	}

	animPath := filepath.Join(tmp, "anim.mp4")
	ctx := context.Background()
	if err := RenderAnimationMP4(ctx, in, animPath, RenderAnimOptions{
		FFmpegExe: ffmpegPath,
		Encoder:   "libx264",
	}); err != nil {
		t.Fatalf("RenderAnimationMP4 failed: %v", err)
	}

	out, err := exec.Command(ffprobePath, "-v", "error", "-show_format", "-show_streams", "-of", "json", animPath).Output()
	if err != nil {
		t.Fatalf("ffprobe failed: %v", err)
	}
	var probe struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			NbFrames  string `json:"nb_frames"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatalf("parse ffprobe json: %v", err)
	}
	dur, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || math.Abs(dur-AnimDurationSec) > 0.25 {
		t.Fatalf("unexpected duration %q (err=%v)", probe.Format.Duration, err)
	}
	if len(probe.Streams) == 0 || probe.Streams[0].Width != AnimOutWidth || probe.Streams[0].Height != AnimOutHeight {
		t.Fatalf("unexpected stream dimensions: %+v", probe.Streams)
	}

	// Appearance update with FFmpeg available regenerates the file.
	portraitPath := filepath.Join(tmp, "new_portrait.png")
	writeSolidPNG(t, portraitPath, 64, 64, color.RGBA{R: 255, G: 200, B: 90, A: 255})
	in.PortraitPath = portraitPath
	gotPath, err := SyncAppearanceAnimation(ctx, in, animPath, RenderAnimOptions{FFmpegExe: ffmpegPath, Encoder: "libx264"})
	if err != nil || gotPath != animPath {
		t.Fatalf("SyncAppearanceAnimation regen failed: path=%q err=%v", gotPath, err)
	}

	// Appearance update without FFmpeg removes stale animation cleanly.
	gotPath, err = SyncAppearanceAnimation(ctx, in, animPath, RenderAnimOptions{FFmpegExe: ""})
	if err != nil || gotPath != "" {
		t.Fatalf("expected empty path without ffmpeg, got %q err=%v", gotPath, err)
	}
	if _, statErr := os.Stat(animPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected stale animation to be removed, stat err=%v", statErr)
	}
}
