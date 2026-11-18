package composer

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestProbeEncoderAndVerifyMediaFile(t *testing.T) {
	ffmpegExe, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobeExe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}

	ctx := context.Background()
	probe, err := ProbeEncoder(ctx, ffmpegExe, "auto")
	if err != nil {
		t.Fatalf("ProbeEncoder failed: %v", err)
	}
	if probe.Selected == "" {
		t.Fatalf("expected non-empty selected encoder")
	}

	// Generate a 1.0s synthetic clip with 48kHz stereo AAC audio and verify via ffprobe.
	outPath := filepath.Join(t.TempDir(), "sample.mp4")
	cmd := exec.CommandContext(ctx, ffmpegExe,
		"-y",
		"-f", "lavfi", "-i", "testsrc=size=640x360:rate=30:duration=1.0",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=1.0",
		"-c:v", probe.Selected, "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate sample mp4: %v (%s)", err, string(out))
	}

	vm, err := VerifyMediaFile(ctx, outPath, MediaVerifyOptions{
		FFprobeExe:         ffprobeExe,
		MinFileBytes:       1024,
		ExpectedDuration:   1.0,
		DurationTolerance:  0.25,
		RequireAudio:       true,
		ExpectedSampleRate: 48000,
	})
	if err != nil {
		t.Fatalf("VerifyMediaFile failed: %v", err)
	}
	if vm.Width != 640 || vm.Height != 360 || !vm.HasAudio || vm.SampleRate != 48000 {
		t.Fatalf("unexpected VerifiedMedia: %+v", vm)
	}
}
