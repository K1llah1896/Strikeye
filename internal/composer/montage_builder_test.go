package composer

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"strikeye/internal/config"
	"strikeye/internal/storage"
)

func TestBuildMontageEndToEndSynthetic(t *testing.T) {
	ffmpegExe, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobeExe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}

	tmp := t.TempDir()
	layout, err := config.EnsureDataLayout(tmp)
	if err != nil {
		t.Fatalf("EnsureDataLayout: %v", err)
	}
	db, err := storage.Open(filepath.Join(tmp, "strikeye.db"), layout.BackupDBSnapshots)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:      filepath.Join(tmp, "match.dem"),
		Filename:      "match.dem",
		ContentSHA256: "montagetest",
		FileSize:      2048,
	})
	if err != nil {
		t.Fatalf("UpsertByContentHash: %v", err)
	}

	// Create synthetic intro (1s), clip1 (2s), clip2 (2s), radar PNG (1.5s), and BGM (5s).
	clip1 := filepath.Join(tmp, "clip1.mp4")
	clip2 := filepath.Join(tmp, "clip2.mp4")
	bgm := filepath.Join(tmp, "bgm.wav")
	radarPNG := filepath.Join(tmp, "radar.png")

	genClip := func(path string, colorExpr string, dur string) {
		out, err := exec.CommandContext(ctx, ffmpegExe,
			"-y",
			"-f", "lavfi", "-i", colorExpr+"=size=640x360:rate=24:duration="+dur,
			"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration="+dur,
			"-c:v", "libx264", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-ar", "48000", "-ac", "2",
			path,
		).CombinedOutput()
		if err != nil {
			t.Fatalf("gen clip %s: %v (%s)", path, err, string(out))
		}
	}
	genClip(clip1, "testsrc", "2.0")
	genClip(clip2, "smptebars", "2.0")
	if out, err := exec.CommandContext(ctx, ffmpegExe,
		"-y", "-f", "lavfi", "-i", "sine=frequency=220:sample_rate=48000:duration=5.0", bgm,
	).CombinedOutput(); err != nil {
		t.Fatalf("gen bgm: %v (%s)", err, string(out))
	}

	// Write a 640x360 radar PNG.
	img := image.NewRGBA(image.Rect(0, 0, 640, 360))
	for y := 0; y < 360; y++ {
		for x := 0; x < 640; x++ {
			img.SetRGBA(x, y, color.RGBA{20, 40, 80, 255})
		}
	}
	f, err := os.Create(radarPNG)
	if err != nil {
		t.Fatalf("create radar png: %v", err)
	}
	_ = png.Encode(f, img)
	_ = f.Close()

	outMP4 := filepath.Join(layout.MediaExports, "montage.mp4")
	res, err := BuildMontage(ctx, MontageSpec{
		DemoID: demo.ID,
		Title:  "Test Montage",
		Clips: []MontageClip{
			{
				ID:            "c1",
				VideoPath:     clip1,
				Transition:    "fade",
				TransitionDur: 0.4,
				NameCard: &NameCardInput{
					PlayerName:   "PlayerOne",
					TeamKey:      "CT",
					RoundNumber:  5,
					HighlightTag: "三杀",
				},
				Subtitle: "Nice 3K!",
			},
			{
				ID:        "c2",
				VideoPath: clip2,
			},
		},
		RadarSegments: []RadarSegment{
			{BeforeClipID: "c2", MediaPath: radarPNG, DurationSec: 1.5},
		},
		BGMPath:    bgm,
		Width:      640,
		Height:     360,
		FPS:        24,
		OutputPath: outMP4,
	}, MontageDeps{
		FFmpegExe:    ffmpegExe,
		FFprobeExe:   ffprobeExe,
		Encoder:      "libx264",
		MinFileBytes: 1024,
		DB:           db,
	})
	if err != nil {
		t.Fatalf("BuildMontage failed: %v", err)
	}

	// Expected duration: clip1 (2.0) + radar (1.5) + clip2 (2.0) = 5.5s (hard-cut around radar).
	if math.Abs(res.Verified.DurationSec-5.5) > 0.5 {
		t.Fatalf("expected ~5.5s montage, got %.3fs", res.Verified.DurationSec)
	}
	if !res.Verified.HasAudio || res.Verified.SampleRate != 48000 {
		t.Fatalf("expected 48kHz audio, got %+v", res.Verified)
	}
	if res.MediaVideoID == 0 || res.ExportID == 0 {
		t.Fatalf("expected persisted MediaVideoID and ExportID, got %+v", res)
	}

	exp, err := db.Montage().GetExport(ctx, res.ExportID)
	if err != nil || exp.Status != "completed" {
		t.Fatalf("expected completed montage_export, got %+v err=%v", exp, err)
	}

	// Also verify xfade transition grouping between consecutive clips.
	outXfade := filepath.Join(layout.MediaExports, "montage_xfade.mp4")
	res2, err := BuildMontage(ctx, MontageSpec{
		DemoID: demo.ID,
		Title:  "Xfade Montage",
		Clips: []MontageClip{
			{ID: "x1", VideoPath: clip1, Transition: "fade", TransitionDur: 0.4},
			{ID: "x2", VideoPath: clip2},
		},
		Width:      640,
		Height:     360,
		FPS:        24,
		OutputPath: outXfade,
	}, MontageDeps{
		FFmpegExe:    ffmpegExe,
		FFprobeExe:   ffprobeExe,
		Encoder:      "libx264",
		MinFileBytes: 1024,
		DB:           db,
	})
	if err != nil {
		t.Fatalf("BuildMontage xfade failed: %v", err)
	}
	// 2.0 + 2.0 - 0.4 = 3.6s
	if math.Abs(res2.Verified.DurationSec-3.6) > 0.5 {
		t.Fatalf("expected ~3.6s xfade montage, got %.3fs", res2.Verified.DurationSec)
	}
}
