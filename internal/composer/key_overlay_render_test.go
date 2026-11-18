package composer

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"strikeye/internal/parser"
)

func TestKeyTimelineMaskLookup(t *testing.T) {
	edges := []parser.InputEdge{
		{Tick: 200, PlayerSlot: 3, Buttons: parser.KeyW | parser.KeyFire},
		{Tick: 100, PlayerSlot: 3, Buttons: parser.KeyA},
		{Tick: 150, PlayerSlot: 4, Buttons: parser.KeyJump}, // different player slot
	}
	kt := NewKeyTimeline(edges, 3)
	if got := kt.MaskAtTick(50); got != 0 {
		t.Fatalf("expected 0 before first edge, got %d", got)
	}
	if got := kt.MaskAtTick(150); got != parser.KeyA {
		t.Fatalf("expected KeyA at tick 150, got %d", got)
	}
	if got := kt.MaskAtTick(200); got != (parser.KeyW | parser.KeyFire) {
		t.Fatalf("expected KeyW|KeyFire at tick 200, got %d", got)
	}
}

func TestApplyKeyOverlaySmoke(t *testing.T) {
	ffmpegExe, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobeExe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "base.mp4")
	outPath := filepath.Join(tmp, "overlay.mp4")
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, ffmpegExe,
		"-y",
		"-f", "lavfi", "-i", "testsrc=size=640x360:rate=24:duration=1.0",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=1.0",
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		inPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate base mp4: %v (%s)", err, string(out))
	}

	err = ApplyKeyOverlay(ctx, inPath, outPath, KeyOverlayOptions{
		FFmpegExe:  ffmpegExe,
		Encoder:    "libx264",
		StartTick:  1000,
		EndTick:    1064,
		TickRate:   64,
		FPS:        24,
		PlayerSlot: 1,
		Edges: []parser.InputEdge{
			{Tick: 1000, PlayerSlot: 1, Buttons: parser.KeyW},
			{Tick: 1032, PlayerSlot: 1, Buttons: parser.KeyW | parser.KeyFire},
		},
	})
	if err != nil {
		t.Fatalf("ApplyKeyOverlay failed: %v", err)
	}

	vm, err := VerifyMediaFile(ctx, outPath, MediaVerifyOptions{
		FFprobeExe:       ffprobeExe,
		MinFileBytes:     1024,
		ExpectedDuration: 1.0,
		RequireAudio:     true,
	})
	if err != nil {
		t.Fatalf("VerifyMediaFile overlay failed: %v", err)
	}
	if vm.Width != 640 || vm.Height != 360 {
		t.Fatalf("unexpected dimensions: %dx%d", vm.Width, vm.Height)
	}
}
