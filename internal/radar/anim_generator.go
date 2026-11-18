package radar

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	AnimFPS         = 24
	AnimDurationSec = 4.0
	AnimTotalFrames = 96
	AnimOutWidth    = 1920
	AnimOutHeight   = 1080

	animP1End = 0.30
	animP2End = 0.60
)

// ScaleAt evaluates the monotonic 3-stage intro animation scale curve at normalized time t in [0, 1].
//   - 0.00..0.30: ease-in (power 2.5) from 0.30 to 0.55
//   - 0.30..0.60: ease-out-cubic from 0.55 to 1.00
//   - 0.60..1.00: frozen at 1.00 (no overshoot, no bounce)
func ScaleAt(t float64) float64 {
	if t <= 0 {
		return 0.30
	}
	if t >= animP2End {
		return 1.00
	}
	if t <= animP1End {
		p := t / animP1End
		return 0.30 + 0.25*math.Pow(p, 2.5)
	}
	p := (t - animP1End) / (animP2End - animP1End)
	easeOut := 1.0 - math.Pow(1.0-p, 3.0)
	return 0.55 + 0.45*easeOut
}

// RenderAnimOptions configures parallel frame rendering and FFmpeg encoding.
type RenderAnimOptions struct {
	FFmpegExe string
	Encoder   string // defaults to "libx264"
	Workers   int    // defaults to runtime.NumCPU()
}

// RenderAnimationMP4 renders 96 frames (2560x1440) in parallel and pipes raw RGBA frames
// directly into FFmpeg to produce a 4.0s 24fps 1920x1080 H.264 MP4 at outputPath.
func RenderAnimationMP4(ctx context.Context, in CardInput, outputPath string, opts RenderAnimOptions) error {
	ffmpegExe := opts.FFmpegExe
	if ffmpegExe == "" {
		ffmpegExe = "ffmpeg"
	}
	encoder := opts.Encoder
	if encoder == "" {
		encoder = "libx264"
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers < 1 {
			workers = 1
		}
	}
	if workers > AnimTotalFrames {
		workers = AnimTotalFrames
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create animation dir: %w", err)
	}
	tmpPath := outputPath + ".tmp.mp4"
	_ = os.Remove(tmpPath)

	args := []string{
		"-y",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", CanvasWidth, CanvasHeight),
		"-r", fmt.Sprintf("%d", AnimFPS),
		"-i", "pipe:0",
		"-vf", fmt.Sprintf("scale=%d:%d:flags=lanczos", AnimOutWidth, AnimOutHeight),
		"-c:v", encoder,
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		tmpPath,
	}
	cmd := exec.CommandContext(ctx, ffmpegExe, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdin pipe: %w", err)
	}
	var stderrBuf limitedBuffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	assets := loadCardAssets(in)
	frames := make([]*image.RGBA, AnimTotalFrames)
	jobs := make(chan int, AnimTotalFrames)
	for i := 0; i < AnimTotalFrames; i++ {
		jobs <- i
	}
	close(jobs)

	var (
		wg       sync.WaitGroup
		firstErr error
		errOnce  sync.Once
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cf, err := loadCardFonts()
			if err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}
			defer cf.Close()

			for idx := range jobs {
				if ctx.Err() != nil {
					errOnce.Do(func() { firstErr = ctx.Err() })
					return
				}
				tt := float64(idx) / float64(AnimTotalFrames-1)
				flash := 0.0
				if tt > animP1End && tt < animP2End {
					p := (tt - animP1End) / (animP2End - animP1End)
					flash = math.Sin(p * math.Pi)
				}
				params := RenderParams{
					Scale:         ScaleAt(tt),
					MatchAvgAlpha: 1.0, // always visible from frame 1
					PulsePhase:    tt,
					FlashAlpha:    flash,
				}
				frames[idx] = renderCardFrame(in, params, assets, cf)
			}
		}()
	}
	wg.Wait()

	if firstErr != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		_ = os.Remove(tmpPath)
		return firstErr
	}

	var writeErr error
	for idx := 0; idx < AnimTotalFrames; idx++ {
		if frames[idx] == nil {
			writeErr = fmt.Errorf("missing rendered frame %d", idx)
			break
		}
		if _, err := stdin.Write(frames[idx].Pix); err != nil {
			writeErr = fmt.Errorf("write frame %d to ffmpeg: %w", idx, err)
			break
		}
		frames[idx] = nil // release memory promptly
	}
	_ = stdin.Close()

	waitErr := cmd.Wait()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return writeErr
	}
	if waitErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg animation encode failed: %w (%s)", waitErr, stderrBuf.String())
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename animation output: %w", err)
	}
	return nil
}

// SyncAppearanceAnimation keeps an existing animation synchronized with updated portrait/team-logo
// assets. If animPath does not exist, it returns "" with nil error. If FFmpeg is unavailable or
// fails, it removes the stale animation file and returns "" so the caller falls back to static PNG.
func SyncAppearanceAnimation(ctx context.Context, in CardInput, animPath string, opts RenderAnimOptions) (string, error) {
	if animPath == "" {
		return "", nil
	}
	if _, err := os.Stat(animPath); os.IsNotExist(err) {
		return "", nil
	}
	if opts.FFmpegExe == "" {
		_ = os.Remove(animPath)
		return "", nil
	}
	if _, err := exec.LookPath(opts.FFmpegExe); err != nil {
		_ = os.Remove(animPath)
		return "", nil
	}
	if err := RenderAnimationMP4(ctx, in, animPath, opts); err != nil {
		_ = os.Remove(animPath)
		return "", err
	}
	return animPath, nil
}

type limitedBuffer struct {
	data []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	const maxBytes = 8192
	if len(b.data) < maxBytes {
		rem := maxBytes - len(b.data)
		if len(p) > rem {
			b.data = append(b.data, p[:rem]...)
		} else {
			b.data = append(b.data, p...)
		}
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return string(b.data)
}
