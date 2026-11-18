package composer

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"

	"strikeye/assets/fonts"
	"strikeye/internal/parser"
)

const (
	KeyHUDWidth  = 420
	KeyHUDHeight = 200
)

// KeyTimeline indexes sorted InputEdge transitions for fast tick-to-button lookup.
type KeyTimeline struct {
	edges []parser.InputEdge
}

// NewKeyTimeline filters edges for playerSlot (if >= 0) and sorts them by Tick.
func NewKeyTimeline(edges []parser.InputEdge, playerSlot int) *KeyTimeline {
	filtered := make([]parser.InputEdge, 0, len(edges))
	for _, e := range edges {
		if playerSlot >= 0 && int(e.PlayerSlot) != playerSlot {
			continue
		}
		filtered = append(filtered, e)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Tick < filtered[j].Tick
	})
	return &KeyTimeline{edges: filtered}
}

// MaskAtTick returns the active Strikeye button bitmask at tick.
func (kt *KeyTimeline) MaskAtTick(tick int64) uint16 {
	if kt == nil || len(kt.edges) == 0 {
		return 0
	}
	idx := sort.Search(len(kt.edges), func(i int) bool {
		return kt.edges[i].Tick > tick
	})
	if idx == 0 {
		return 0
	}
	return kt.edges[idx-1].Buttons
}

type keyCap struct {
	label string
	bit   uint16
	x, y  float64
	w, h  float64
}

var hudKeyLayout = []keyCap{
	{label: "W", bit: parser.KeyW, x: 74, y: 14, w: 48, h: 48},
	{label: "E", bit: parser.KeyUse, x: 128, y: 14, w: 44, h: 48},
	{label: "R", bit: parser.KeyReload, x: 178, y: 14, w: 44, h: 48},
	{label: "F", bit: parser.KeyInspect, x: 228, y: 14, w: 44, h: 48},

	{label: "A", bit: parser.KeyA, x: 20, y: 68, w: 48, h: 48},
	{label: "S", bit: parser.KeyS, x: 74, y: 68, w: 48, h: 48},
	{label: "D", bit: parser.KeyD, x: 128, y: 68, w: 48, h: 48},

	{label: "SHIFT", bit: parser.KeyWalk, x: 20, y: 122, w: 72, h: 42},
	{label: "CTRL", bit: parser.KeyCrouch, x: 98, y: 122, w: 66, h: 42},
	{label: "SPACE", bit: parser.KeyJump, x: 170, y: 122, w: 102, h: 42},

	{label: "M1", bit: parser.KeyFire, x: 292, y: 14, w: 54, h: 72},
	{label: "M2", bit: parser.KeyScope, x: 352, y: 14, w: 54, h: 72},
}

// RenderKeyHUDFrame renders a transparent 420x200 RGBA key HUD image for the given button mask.
func RenderKeyHUDFrame(mask uint16, face font.Face) *image.RGBA {
	dc := gg.NewContext(KeyHUDWidth, KeyHUDHeight)
	if face != nil {
		dc.SetFontFace(face)
	}

	// Subtle translucent backing panel.
	dc.SetRGBA(0.06, 0.08, 0.12, 0.52)
	dc.DrawRoundedRectangle(4, 4, KeyHUDWidth-8, KeyHUDHeight-24, 14)
	dc.Fill()

	for _, k := range hudKeyLayout {
		active := (mask & k.bit) != 0
		if active {
			dc.SetRGBA(0.96, 0.62, 0.04, 0.92) // Strikeye amber highlight
		} else {
			dc.SetRGBA(0.11, 0.15, 0.22, 0.85)
		}
		dc.DrawRoundedRectangle(k.x, k.y, k.w, k.h, 8)
		dc.Fill()

		if active {
			dc.SetRGBA(1.0, 0.92, 0.70, 1.0)
			dc.SetLineWidth(2.2)
		} else {
			dc.SetRGBA(0.42, 0.50, 0.62, 0.55)
			dc.SetLineWidth(1.2)
		}
		dc.DrawRoundedRectangle(k.x, k.y, k.w, k.h, 8)
		dc.Stroke()

		if active {
			dc.SetRGBA(0.05, 0.07, 0.10, 1.0)
		} else {
			dc.SetRGBA(0.85, 0.90, 0.96, 0.90)
		}
		dc.DrawStringAnchored(k.label, k.x+k.w/2, k.y+k.h/2, 0.5, 0.5)
	}

	if rgba, ok := dc.Image().(*image.RGBA); ok {
		return rgba
	}
	return image.NewRGBA(image.Rect(0, 0, KeyHUDWidth, KeyHUDHeight))
}

// KeyOverlayOptions configures RGBA-piped key HUD compositing onto an existing video.
type KeyOverlayOptions struct {
	FFmpegExe  string
	Encoder    string
	StartTick  int64
	EndTick    int64
	TickRate   float64
	FPS        float64
	PlayerSlot int // -1 to accept all edges
	Edges      []parser.InputEdge
	PosX       int // overlay X; defaults to 40
	PosY       int // overlay Y; defaults to 40
}

// ApplyKeyOverlay composites a synchronized transparent key HUD onto inVideoPath via FFmpeg
// using an RGBA rawvideo pipe (zero intermediate frame files on disk).
func ApplyKeyOverlay(ctx context.Context, inVideoPath, outVideoPath string, opts KeyOverlayOptions) error {
	ffmpegExe := opts.FFmpegExe
	if ffmpegExe == "" {
		ffmpegExe = "ffmpeg"
	}
	encoder := opts.Encoder
	if encoder == "" {
		encoder = "libx264"
	}
	tickRate := opts.TickRate
	if tickRate <= 0 {
		tickRate = 64.0
	}
	fps := opts.FPS
	if fps <= 0 {
		fps = 60.0
	}
	posX := opts.PosX
	if posX <= 0 {
		posX = 40
	}
	posY := opts.PosY
	if posY <= 0 {
		posY = 40
	}

	spanTicks := opts.EndTick - opts.StartTick
	if spanTicks <= 0 {
		return fmt.Errorf("invalid tick range [%d, %d]", opts.StartTick, opts.EndTick)
	}
	durationSec := float64(spanTicks) / tickRate
	totalFrames := int(math.Ceil(durationSec * fps))
	if totalFrames < 1 {
		totalFrames = 1
	}

	if err := os.MkdirAll(filepath.Dir(outVideoPath), 0o755); err != nil {
		return err
	}
	tmpPath := outVideoPath + ".tmp.mp4"
	_ = os.Remove(tmpPath)

	filterComplex := fmt.Sprintf("[0:v][1:v]overlay=%d:%d:shortest=1[vout]", posX, posY)
	args := []string{
		"-y",
		"-i", inVideoPath,
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", KeyHUDWidth, KeyHUDHeight),
		"-r", fmt.Sprintf("%.3f", fps),
		"-i", "pipe:0",
		"-filter_complex", filterComplex,
		"-map", "[vout]",
		"-map", "0:a?",
		"-c:v", encoder,
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		tmpPath,
	}

	cmd := exec.CommandContext(ctx, ffmpegExe, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg overlay stdin pipe: %w", err)
	}
	var stderr limitedBuffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg overlay: %w", err)
	}

	face, err := fonts.NewFace(fonts.RajdhaniBold, 16)
	if err == nil {
		defer face.Close()
	}

	timeline := NewKeyTimeline(opts.Edges, opts.PlayerSlot)
	cache := make(map[uint16]*image.RGBA, 16)

	var writeErr error
	for i := 0; i < totalFrames; i++ {
		if ctx.Err() != nil {
			writeErr = ctx.Err()
			break
		}
		tick := opts.StartTick + int64(math.Round((float64(i)/fps)*tickRate))
		mask := timeline.MaskAtTick(tick)
		frame, ok := cache[mask]
		if !ok {
			frame = RenderKeyHUDFrame(mask, face)
			cache[mask] = frame
		}
		if _, err := stdin.Write(frame.Pix); err != nil {
			writeErr = fmt.Errorf("write key HUD frame %d: %w", i, err)
			break
		}
	}
	_ = stdin.Close()

	waitErr := cmd.Wait()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return writeErr
	}
	if waitErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg key overlay failed: %w (%s)", waitErr, stderr.String())
	}
	return os.Rename(tmpPath, outVideoPath)
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
