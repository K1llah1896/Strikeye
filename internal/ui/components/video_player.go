package components

import (
	"bufio"
	"fmt"
	"image"
	"io"
	"math"
	"os/exec"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/ui/theme"
)

// TimelineAnchor represents a clickable match event marker on the player timeline.
type TimelineAnchor struct {
	EventID  int64
	Tick     int64
	TimeSec  float64
	Kind     string // "kill", "death", "teammate_kill", "bomb_plant", "bomb_defuse", "bomb_explode"
	Label    string
	RoundNum int
}

// FFmpegVideoSeeker implements ui.VideoSeeker by streaming scaled RGBA frames from ffmpeg.
type FFmpegVideoSeeker struct {
	mu         sync.Mutex
	ffmpegPath string
	filePath   string
	width      int
	height     int
	fps        float64
	duration   time.Duration
	speed      float64
	cmd        *exec.Cmd
	stdout     io.ReadCloser
	reader     *bufio.Reader
	frameBuf   []byte
	img        *image.RGBA
	closed     bool
}

// NewFFmpegVideoSeeker creates a seeker for filePath decoded at width x height.
func NewFFmpegVideoSeeker(ffmpegPath, filePath string, width, height int, fps float64, duration time.Duration) *FFmpegVideoSeeker {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if width <= 0 {
		width = 960
	}
	if height <= 0 {
		height = 540
	}
	if fps <= 0 {
		fps = 30
	}
	return &FFmpegVideoSeeker{
		ffmpegPath: ffmpegPath,
		filePath:   filePath,
		width:      width,
		height:     height,
		fps:        fps,
		duration:   duration,
		speed:      1.0,
		frameBuf:   make([]byte, width*height*4),
		img:        image.NewRGBA(image.Rect(0, 0, width, height)),
	}
}

// Duration reports total media duration for ui.VideoPlayer.
func (s *FFmpegVideoSeeker) Duration() time.Duration {
	return s.duration
}

// SetSpeed updates playback speed multiplier (0.25x to 2.0x).
func (s *FFmpegVideoSeeker) SetSpeed(speed float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if speed < 0.25 {
		speed = 0.25
	}
	if speed > 2.0 {
		speed = 2.0
	}
	s.speed = speed
}

func (s *FFmpegVideoSeeker) stopProcessLocked() {
	if s.stdout != nil {
		_ = s.stdout.Close()
		s.stdout = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
		s.cmd = nil
	}
	s.reader = nil
}

func (s *FFmpegVideoSeeker) startAtLocked(pos time.Duration) error {
	s.stopProcessLocked()
	if s.closed {
		return io.EOF
	}
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", pos.Seconds()),
		"-i", s.filePath,
		"-vf", fmt.Sprintf("fps=%.2f,scale=%d:%d:flags=fast_bilinear", s.fps, s.width, s.height),
		"-f", "rawvideo", "-pix_fmt", "rgba", "pipe:1",
	}
	cmd := exec.Command(s.ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd = cmd
	s.stdout = stdout
	s.reader = bufio.NewReaderSize(stdout, len(s.frameBuf)*2)
	return nil
}

// Next decodes the next RGBA frame from ffmpeg.
func (s *FFmpegVideoSeeker) Next() (ui.VideoFrame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reader == nil {
		if err := s.startAtLocked(0); err != nil {
			return ui.VideoFrame{}, err
		}
	}
	if _, err := io.ReadFull(s.reader, s.frameBuf); err != nil {
		return ui.VideoFrame{}, io.EOF
	}
	copy(s.img.Pix, s.frameBuf)
	spd := s.speed
	if spd <= 0 {
		spd = 1.0
	}
	frameDur := time.Duration(float64(time.Second) / (s.fps * spd))
	return ui.VideoFrame{Image: s.img, Duration: frameDur}, nil
}

// Rewind restarts decoding at timestamp 0.
func (s *FFmpegVideoSeeker) Rewind() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startAtLocked(0)
}

// Seek restarts decoding at d from the start of the video.
func (s *FFmpegVideoSeeker) Seek(d time.Duration) (time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d < 0 {
		d = 0
	}
	if s.duration > 0 && d > s.duration {
		d = s.duration
	}
	if err := s.startAtLocked(d); err != nil {
		return d, err
	}
	return d, nil
}

// Close terminates the underlying ffmpeg decoder process.
func (s *FFmpegVideoSeeker) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.stopProcessLocked()
}

// VideoPlayerWidget wraps MyGo's ui.VideoPlayer with Strikeye tick-mapping,
// speed controls, 2.0s-pre-event anchors, and dual-slider In/Out trimming.
type VideoPlayerWidget struct {
	mu          sync.Mutex
	ffmpegPath  string
	filePath    string
	player      *ui.VideoPlayer
	seeker      *FFmpegVideoSeeker
	durationSec float64
	seekSec     float64
	speed       float64
	volume      float64
	fullscreen  bool
	startTick   int64
	endTick     int64
	tickRate    float64
	inSec       float64
	outSec      float64
}

// NewVideoPlayerWidget constructs a reusable Strikeye video player widget.
func NewVideoPlayerWidget(ffmpegPath string) *VideoPlayerWidget {
	return &VideoPlayerWidget{
		ffmpegPath: ffmpegPath,
		speed:      1.0,
		volume:     1.0,
		tickRate:   64.0,
	}
}

// Load opens filePath at an optional initialSeekSec offset and binds tick bounds.
func (w *VideoPlayerWidget) Load(filePath string, durationSec float64, startTick, endTick int64, tickRate float64, initialSeekSec float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.seeker != nil {
		w.seeker.Close()
		w.seeker = nil
	}
	if durationSec <= 0 {
		durationSec = 10
	}
	if tickRate <= 0 {
		tickRate = 64.0
	}
	w.filePath = filePath
	w.durationSec = durationSec
	w.startTick = startTick
	w.endTick = endTick
	w.tickRate = tickRate
	w.inSec = 0
	w.outSec = durationSec

	seeker := NewFFmpegVideoSeeker(w.ffmpegPath, filePath, 960, 540, 30, time.Duration(durationSec*float64(time.Second)))
	seeker.SetSpeed(w.speed)
	w.seeker = seeker
	p, err := ui.NewVideoPlayer(seeker)
	if err == nil {
		w.player = p
		if initialSeekSec > 0 {
			w.player.Seek(time.Duration(initialSeekSec * float64(time.Second)))
			w.seekSec = initialSeekSec
		} else {
			w.seekSec = 0
		}
	}
}

// Close releases the active ffmpeg decoder process.
func (w *VideoPlayerWidget) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.seeker != nil {
		w.seeker.Close()
		w.seeker = nil
	}
	w.player = nil
}

// SeekEvent jumps to max(0, eventSec - 2.0) per Section 8.2.
func (w *VideoPlayerWidget) SeekEvent(eventSec float64) {
	target := ComputeEventSeekTime(eventSec, 2.0)
	w.SeekTime(target)
}

// SeekTime seeks playback to sec.
func (w *VideoPlayerWidget) SeekTime(sec float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if sec < 0 {
		sec = 0
	}
	if w.durationSec > 0 && sec > w.durationSec {
		sec = w.durationSec
	}
	w.seekSec = sec
	if w.player != nil {
		w.player.Seek(time.Duration(sec * float64(time.Second)))
	}
}

// CurrentTimeSec returns the current playhead in seconds.
func (w *VideoPlayerWidget) CurrentTimeSec() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.player != nil {
		return w.player.Position().Seconds()
	}
	return w.seekSec
}

// CurrentTick returns the Demo Tick corresponding to the current playhead.
func (w *VideoPlayerWidget) CurrentTick() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	posSec := w.seekSec
	if w.player != nil {
		posSec = w.player.Position().Seconds()
	}
	return TimeToTick(w.startTick, posSec, w.tickRate)
}

// InOutTicks returns the trimmed [InTick, OutTick] range.
func (w *VideoPlayerWidget) InOutTicks() (int64, int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	inT := TimeToTick(w.startTick, w.inSec, w.tickRate)
	outT := TimeToTick(w.startTick, w.outSec, w.tickRate)
	if outT <= inT {
		outT = inT + int64(w.tickRate)
	}
	return inT, outT
}

// ComputeEventSeekTime returns max(0, eventSec - leadSec).
func ComputeEventSeekTime(eventSec, leadSec float64) float64 {
	t := eventSec - leadSec
	if t < 0 {
		return 0
	}
	return t
}

// TimeToTick converts a relative video playback timestamp in seconds into a Demo tick.
func TimeToTick(startTick int64, relSec, tickRate float64) int64 {
	if tickRate <= 0 {
		tickRate = 64.0
	}
	if relSec < 0 {
		relSec = 0
	}
	return startTick + int64(math.Round(relSec*tickRate))
}

// Render draws the video canvas, transport controls, speed/volume/fullscreen buttons,
// Event Timeline anchors, and optional In/Out trim sliders.
func (w *VideoPlayerWidget) Render(c *ui.Context, anchors []TimelineAnchor, enableTrim bool, onReRenderTrim func(inTick, outTick int64)) {
	w.mu.Lock()
	p := w.player
	dur := w.durationSec
	spd := w.speed
	vol := w.volume
	fs := w.fullscreen
	w.mu.Unlock()

	if p == nil {
		theme.Card(c, func() {
			theme.MutedText(c, "未加载视频")
		})
		return
	}

	pos := p.Position().Seconds()
	w.mu.Lock()
	w.seekSec = pos
	w.mu.Unlock()

	videoH := float32(420)
	if fs {
		videoH = 620
	}

	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Column(c).FillWidth().Height(videoH).Background(theme.BgBase).Border(1, theme.RimBorder).Radius(18).Clip().Center().Children(func() {
			ui.Video(c, p).Fill()
		})

		// Scrub bar
		scrubVal := pos
		if ui.Slider(c, &scrubVal, 0, math.Max(1, dur)).FillWidth().Changed() {
			w.SeekTime(scrubVal)
		}

		// Transport + speed + volume + fullscreen
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			playLabel := "播放"
			if p.Playing() {
				playLabel = "暂停"
			}
			if theme.AccentButton(c, playLabel).Clicked() {
				if p.Playing() {
					p.Pause()
				} else {
					p.Play()
				}
			}
			theme.StatText(c, fmt.Sprintf("%.1fs / %.1fs", pos, dur)).FontSize(12).TextColor(theme.TextPrimary)
			ui.Spacer(c)
			for _, s := range []float64{0.25, 0.5, 1.0, 1.5, 2.0} {
				label := fmt.Sprintf("%.2gx", s)
				btn := theme.ActionButton(c, label)
				if math.Abs(spd-s) < 0.01 {
					btn = btn.Border(1, theme.AccentAmber)
				}
				if btn.Clicked() {
					w.mu.Lock()
					w.speed = s
					if w.seeker != nil {
						w.seeker.SetSpeed(s)
					}
					w.mu.Unlock()
				}
			}
			theme.StatText(c, fmt.Sprintf("音量 %.0f%%", vol*100)).FontSize(11).TextColor(theme.TextMuted)
			if ui.Slider(c, &w.volume, 0, 1).Width(80).Changed() {
				// Volume state persisted for audio mixer integration.
			}
			fsLabel := "全屏"
			if fs {
				fsLabel = "退出全屏"
			}
			if theme.ActionButton(c, fsLabel).Clicked() {
				w.mu.Lock()
				w.fullscreen = !w.fullscreen
				w.mu.Unlock()
			}
		})

		// Event Timeline anchors (click jumps to event_time - 2.0s)
		if len(anchors) > 0 {
			theme.RowCenter(c).FillWidth().Gap(6).Children(func() {
				theme.MutedText(c, "事件锚点:")
				ui.ScrollHorizontal(c).FillWidth().Children(func() {
					theme.RowCenter(c).Gap(6).Children(func() {
						for _, a := range anchors {
							col := theme.AccentCyan
							switch a.Kind {
							case "kill":
								col = theme.StatusGreen
							case "death":
								col = theme.StatusRed
							case "bomb_plant", "bomb_explode":
								col = theme.AccentAmber
							}
							if ui.Button(c, fmt.Sprintf("%s (%.1fs)", a.Label, a.TimeSec)).
								Background(theme.BgElevated).
								Border(1, col).
								Radius(4).
								PaddingX(8).
								PaddingY(4).
								TextColor(col).
								Clicked() {
								w.SeekEvent(a.TimeSec)
							}
						}
					})
				})
			})
		}

		// Optional dual-slider In/Out trim for Highlights
		if enableTrim {
			theme.Card(c, func() {
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					theme.StatText(c, fmt.Sprintf("In: %.2fs", w.inSec)).FontSize(12).TextColor(theme.AccentCyan)
					ui.Slider(c, &w.inSec, 0, math.Max(1, dur)).FillWidth()
					theme.StatText(c, fmt.Sprintf("Out: %.2fs", w.outSec)).FontSize(12).TextColor(theme.AccentAmber)
					ui.Slider(c, &w.outSec, 0, math.Max(1, dur)).FillWidth()
					if theme.AccentButton(c, "重新按新 Tick 范围渲染").Clicked() && onReRenderTrim != nil {
						inT, outT := w.InOutTicks()
						onReRenderTrim(inT, outT)
					}
				})
			})
		}
	})
}
