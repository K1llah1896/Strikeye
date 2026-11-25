package composer

import (
	"context"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fogleman/gg"

	"strikeye/assets/fonts"
	"strikeye/internal/storage"
)

// MontageClip describes one gameplay highlight clip in a montage timeline.
type MontageClip struct {
	ID            string         `json:"id"`
	VideoPath     string         `json:"video_path"`
	TrimStartSec  float64        `json:"trim_start_sec"`
	TrimEndSec    float64        `json:"trim_end_sec"`
	Transition    string         `json:"transition"` // "", "cut", "fade", "fadewhite", "wipeleft", "wiperight", "smoothleft"
	TransitionDur float64        `json:"transition_dur"`
	NameCard      *NameCardInput `json:"name_card,omitempty"`
	Subtitle      string         `json:"subtitle,omitempty"`
}

// RadarSegment inserts a static PNG or animated MP4 radar card before a target clip.
type RadarSegment struct {
	BeforeClipID string  `json:"before_clip_id"`
	MediaPath    string  `json:"media_path"`
	DurationSec  float64 `json:"duration_sec"`
}

// MontageSpec defines all inputs and rendering parameters for a montage export.
type MontageSpec struct {
	DemoID        int64          `json:"demo_id"`
	ProjectID     *int64         `json:"project_id,omitempty"`
	Title         string         `json:"title"`
	IntroPath     string         `json:"intro_path,omitempty"`
	OutroPath     string         `json:"outro_path,omitempty"`
	Clips         []MontageClip  `json:"clips"`
	RadarSegments []RadarSegment `json:"radar_segments,omitempty"`
	BGMPath       string         `json:"bgm_path,omitempty"`
	ClipVolume    float64        `json:"clip_volume"`
	BGMVolume     float64        `json:"bgm_volume"`
	BGMFadeSec    float64        `json:"bgm_fade_sec"`
	AudioTracks   []AudioTrack   `json:"audio_tracks,omitempty"`
	Width         int            `json:"width"`
	Height        int            `json:"height"`
	FPS           int            `json:"fps"`
	OutputPath    string         `json:"output_path"`
}

// MontageDeps supplies external tool paths and optional storage repositories.
type MontageDeps struct {
	FFmpegExe    string
	FFprobeExe   string
	Encoder      string
	MinFileBytes int64
	DB           *storage.DB
}

// MontageResult summarizes a completed and verified montage export.
type MontageResult struct {
	ExportID     int64
	MediaVideoID int64
	Verified     VerifiedMedia
}

type preparedSegment struct {
	kind          string // "intro", "radar", "clip", "outro"
	path          string
	duration      float64
	transitionOut string
	transitionDur float64
}

// BuildMontage executes the full montage composition pipeline, verifies the output
// with ffprobe, and persists montage_exports + media_videos records when DB is provided.
func BuildMontage(ctx context.Context, spec MontageSpec, deps MontageDeps) (MontageResult, error) {
	if len(spec.Clips) == 0 {
		return MontageResult{}, fmt.Errorf("montage requires at least one clip")
	}
	if spec.OutputPath == "" {
		return MontageResult{}, fmt.Errorf("output_path is required")
	}
	ffmpegExe := deps.FFmpegExe
	if ffmpegExe == "" {
		ffmpegExe = "ffmpeg"
	}
	ffprobeExe := deps.FFprobeExe
	if ffprobeExe == "" {
		ffprobeExe = "ffprobe"
	}
	encoder := deps.Encoder
	if encoder == "" || encoder == "auto" {
		probe, err := ProbeEncoder(ctx, ffmpegExe, "auto")
		if err != nil {
			encoder = "libx264"
		} else {
			encoder = probe.Selected
		}
	}

	if spec.Width <= 0 {
		spec.Width = 1920
	}
	if spec.Height <= 0 {
		spec.Height = 1080
	}
	if spec.FPS <= 0 {
		spec.FPS = 60
	}
	if spec.ClipVolume <= 0 {
		spec.ClipVolume = 1.0
	}
	if spec.BGMVolume <= 0 {
		spec.BGMVolume = 0.25
	}
	if spec.BGMFadeSec <= 0 {
		spec.BGMFadeSec = 1.5
	}

	var exportID int64
	if deps.DB != nil {
		exp, err := deps.DB.Montage().CreateExport(ctx, spec.ProjectID, spec.OutputPath)
		if err != nil {
			return MontageResult{}, fmt.Errorf("create montage_exports row: %w", err)
		}
		exportID = exp.ID
		_ = deps.DB.Montage().UpdateExportProgress(ctx, exportID, "running", 5.0)
	}

	fail := func(err error) (MontageResult, error) {
		if deps.DB != nil && exportID > 0 {
			_ = deps.DB.Montage().FailExport(ctx, exportID, err.Error())
		}
		return MontageResult{ExportID: exportID}, err
	}

	workDir, err := os.MkdirTemp("", "strikeye-montage-*")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(workDir)

	if err := os.MkdirAll(filepath.Dir(spec.OutputPath), 0o755); err != nil {
		return fail(err)
	}

	radarByClipID := make(map[string][]RadarSegment)
	for _, rs := range spec.RadarSegments {
		radarByClipID[rs.BeforeClipID] = append(radarByClipID[rs.BeforeClipID], rs)
	}

	var segments []preparedSegment
	segIdx := 0

	if spec.IntroPath != "" {
		outSeg := filepath.Join(workDir, fmt.Sprintf("seg_%03d_intro.mp4", segIdx))
		segIdx++
		dur, err := prepareMediaSegment(ctx, ffmpegExe, ffprobeExe, encoder, spec.IntroPath, outSeg, segmentOpts{
			Width: spec.Width, Height: spec.Height, FPS: spec.FPS, DefaultDur: 3.0,
		})
		if err != nil {
			return fail(fmt.Errorf("prepare intro: %w", err))
		}
		segments = append(segments, preparedSegment{kind: "intro", path: outSeg, duration: dur})
	}

	for _, clip := range spec.Clips {
		if list, ok := radarByClipID[clip.ID]; ok {
			for _, rs := range list {
				outSeg := filepath.Join(workDir, fmt.Sprintf("seg_%03d_radar.mp4", segIdx))
				segIdx++
				defDur := rs.DurationSec
				if defDur <= 0 {
					defDur = 4.0
				}
				dur, err := prepareMediaSegment(ctx, ffmpegExe, ffprobeExe, encoder, rs.MediaPath, outSeg, segmentOpts{
					Width: spec.Width, Height: spec.Height, FPS: spec.FPS, DefaultDur: defDur,
				})
				if err != nil {
					return fail(fmt.Errorf("prepare radar segment before clip %s: %w", clip.ID, err))
				}
				segments = append(segments, preparedSegment{kind: "radar", path: outSeg, duration: dur})
			}
		}

		outSeg := filepath.Join(workDir, fmt.Sprintf("seg_%03d_clip.mp4", segIdx))
		segIdx++
		var nameCardPath, subPath string
		if clip.NameCard != nil {
			nameCardPath = filepath.Join(workDir, fmt.Sprintf("namecard_%03d.png", segIdx))
			if err := RenderNameCardPNG(*clip.NameCard, nameCardPath); err != nil {
				return fail(fmt.Errorf("render name card for clip %s: %w", clip.ID, err))
			}
		}
		if strings.TrimSpace(clip.Subtitle) != "" {
			subPath = filepath.Join(workDir, fmt.Sprintf("subtitle_%03d.png", segIdx))
			if err := renderSubtitlePNG(clip.Subtitle, spec.Width, subPath); err != nil {
				return fail(fmt.Errorf("render subtitle for clip %s: %w", clip.ID, err))
			}
		}
		dur, err := prepareMediaSegment(ctx, ffmpegExe, ffprobeExe, encoder, clip.VideoPath, outSeg, segmentOpts{
			Width:        spec.Width,
			Height:       spec.Height,
			FPS:          spec.FPS,
			TrimStart:    clip.TrimStartSec,
			TrimEnd:      clip.TrimEndSec,
			NameCardPath: nameCardPath,
			SubtitlePath: subPath,
		})
		if err != nil {
			return fail(fmt.Errorf("prepare clip %s: %w", clip.ID, err))
		}
		trans := normalizeTransitionName(clip.Transition)
		tdur := clip.TransitionDur
		if tdur <= 0 {
			tdur = 0.5
		}
		segments = append(segments, preparedSegment{
			kind:          "clip",
			path:          outSeg,
			duration:      dur,
			transitionOut: trans,
			transitionDur: tdur,
		})
	}

	if spec.OutroPath != "" {
		outSeg := filepath.Join(workDir, fmt.Sprintf("seg_%03d_outro.mp4", segIdx))
		dur, err := prepareMediaSegment(ctx, ffmpegExe, ffprobeExe, encoder, spec.OutroPath, outSeg, segmentOpts{
			Width: spec.Width, Height: spec.Height, FPS: spec.FPS, DefaultDur: 3.0,
		})
		if err != nil {
			return fail(fmt.Errorf("prepare outro: %w", err))
		}
		segments = append(segments, preparedSegment{kind: "outro", path: outSeg, duration: dur})
	}

	if deps.DB != nil && exportID > 0 {
		_ = deps.DB.Montage().UpdateExportProgress(ctx, exportID, "running", 50.0)
	}

	// Enforce hard-cut boundaries around non-clip segments (intro, radar, outro).
	for i := range segments {
		if i == len(segments)-1 {
			segments[i].transitionOut = ""
			continue
		}
		if segments[i].kind != "clip" || segments[i+1].kind != "clip" {
			segments[i].transitionOut = ""
		}
	}

	// Group contiguous clip segments linked by xfade transitions; each group produces one file.
	var concatPaths []string
	var expectedTotalDur float64
	i := 0
	groupIdx := 0
	for i < len(segments) {
		j := i
		for j < len(segments)-1 && segments[j].transitionOut != "" {
			j++
		}
		if i == j {
			concatPaths = append(concatPaths, segments[i].path)
			expectedTotalDur += segments[i].duration
			i++
			continue
		}
		groupOut := filepath.Join(workDir, fmt.Sprintf("xfade_group_%03d.mp4", groupIdx))
		groupIdx++
		gdur, err := renderXfadeGroup(ctx, ffmpegExe, encoder, segments[i:j+1], groupOut)
		if err != nil {
			return fail(fmt.Errorf("render xfade group %d: %w", groupIdx, err))
		}
		concatPaths = append(concatPaths, groupOut)
		expectedTotalDur += gdur
		i = j + 1
	}

	joinedPath := filepath.Join(workDir, "joined.mp4")
	if len(concatPaths) == 1 {
		joinedPath = concatPaths[0]
	} else {
		if err := concatSegments(ctx, ffmpegExe, concatPaths, joinedPath); err != nil {
			return fail(fmt.Errorf("concat segments: %w", err))
		}
	}

	if deps.DB != nil && exportID > 0 {
		_ = deps.DB.Montage().UpdateExportProgress(ctx, exportID, "running", 80.0)
	}

	tmpOut := spec.OutputPath + ".tmp.mp4"
	_ = os.Remove(tmpOut)
	if spec.BGMPath != "" {
		if err := mixBGM(ctx, ffmpegExe, joinedPath, spec.BGMPath, tmpOut, expectedTotalDur, spec.ClipVolume, spec.BGMVolume, spec.BGMFadeSec); err != nil {
			_ = os.Remove(tmpOut)
			return fail(fmt.Errorf("mix bgm: %w", err))
		}
	} else {
		if err := copyFile(joinedPath, tmpOut); err != nil {
			_ = os.Remove(tmpOut)
			return fail(err)
		}
	}

	// Extra tracks (comms, voice-over) layer on top of the BGM mix so their
	// offsets stay relative to the final timeline.
	if audible := audibleTracks(spec.AudioTracks); len(audible) > 0 {
		mixedPath := filepath.Join(workDir, "mixed_tracks.mp4")
		if err := MixAudioTracks(ctx, ffmpegExe, tmpOut, audible, mixedPath, expectedTotalDur, 1.0); err != nil {
			_ = os.Remove(tmpOut)
			return fail(fmt.Errorf("mix audio tracks: %w", err))
		}
		_ = os.Remove(tmpOut)
		if err := copyFile(mixedPath, tmpOut); err != nil {
			return fail(err)
		}
	}

	minBytes := deps.MinFileBytes
	if minBytes <= 0 {
		minBytes = 64 * 1024
	}
	vm, err := VerifyMediaFile(ctx, tmpOut, MediaVerifyOptions{
		FFprobeExe:         ffprobeExe,
		MinFileBytes:       minBytes,
		ExpectedDuration:   expectedTotalDur,
		DurationTolerance:  math.Max(0.75, 0.15*float64(len(segments))),
		RequireAudio:       true,
		ExpectedSampleRate: 48000,
	})
	if err != nil {
		_ = os.Remove(tmpOut)
		return fail(fmt.Errorf("verify montage output: %w", err))
	}
	if err := os.Rename(tmpOut, spec.OutputPath); err != nil {
		_ = os.Remove(tmpOut)
		return fail(err)
	}
	vm.FilePath = spec.OutputPath

	var mediaID int64
	if deps.DB != nil {
		if spec.DemoID > 0 {
			mv, err := deps.DB.Media().Upsert(ctx, storage.MediaVideo{
				DemoID:         spec.DemoID,
				Perspective:    "montage",
				VideoScope:     "montage_export",
				StartTick:      0,
				EndTick:        0,
				SegmentsJSON:   "[]",
				FilePath:       spec.OutputPath,
				FileSize:       vm.FileSize,
				DurationSec:    vm.DurationSec,
				Width:          vm.Width,
				Height:         vm.Height,
				FPS:            vm.FPS,
				VideoCodec:     vm.VideoCodec,
				HasAudio:       vm.HasAudio,
				VerifiedStatus: "verified",
			})
			if err != nil {
				return fail(fmt.Errorf("upsert media_videos: %w", err))
			}
			mediaID = mv.ID
		}
		if exportID > 0 {
			if mediaID > 0 {
				if err := deps.DB.Montage().CompleteExport(ctx, exportID, mediaID); err != nil {
					return fail(err)
				}
			} else {
				_ = deps.DB.Montage().UpdateExportProgress(ctx, exportID, "completed", 100.0)
			}
		}
	}

	return MontageResult{
		ExportID:     exportID,
		MediaVideoID: mediaID,
		Verified:     vm,
	}, nil
}

type segmentOpts struct {
	Width        int
	Height       int
	FPS          int
	DefaultDur   float64
	TrimStart    float64
	TrimEnd      float64
	NameCardPath string
	SubtitlePath string
}

func prepareMediaSegment(ctx context.Context, ffmpegExe, ffprobeExe, encoder, inPath, outPath string, opts segmentOpts) (float64, error) {
	isImage := isImageExt(inPath)
	var args []string
	var filterParts []string
	lastV := "vbase"

	if isImage {
		dur := opts.DefaultDur
		if dur <= 0 {
			dur = 4.0
		}
		args = append(args, "-y", "-loop", "1", "-t", fmt.Sprintf("%.3f", dur), "-i", inPath)
		args = append(args, "-f", "lavfi", "-t", fmt.Sprintf("%.3f", dur), "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
		filterParts = append(filterParts, fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,fps=%d,format=yuv420p[%s]",
			opts.Width, opts.Height, opts.Width, opts.Height, opts.FPS, lastV))
		args = append(args,
			"-filter_complex", strings.Join(filterParts, ";"),
			"-map", fmt.Sprintf("[%s]", lastV),
			"-map", "1:a",
			"-c:v", encoder, "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-ar", "48000", "-ac", "2",
			"-shortest", outPath,
		)
		if out, err := exec.CommandContext(ctx, ffmpegExe, args...).CombinedOutput(); err != nil {
			return 0, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return dur, nil
	}

	args = append(args, "-y")
	if opts.TrimStart > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", opts.TrimStart))
	}
	if opts.TrimEnd > opts.TrimStart && opts.TrimEnd > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", opts.TrimEnd-opts.TrimStart))
	}
	args = append(args, "-i", inPath)

	inputIdx := 1
	hasAudio, _ := probeHasAudio(ctx, ffprobeExe, inPath)
	audioMap := "0:a"
	if !hasAudio {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
		audioMap = fmt.Sprintf("%d:a", inputIdx)
		inputIdx++
	}

	filterParts = append(filterParts, fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,fps=%d,format=yuv420p[%s]",
		opts.Width, opts.Height, opts.Width, opts.Height, opts.FPS, lastV))

	if opts.NameCardPath != "" {
		args = append(args, "-i", opts.NameCardPath)
		nextV := "vnc"
		filterParts = append(filterParts, fmt.Sprintf("[%s][%d:v]overlay=40:H-h-40:enable='between(t,0,3.5)'[%s]", lastV, inputIdx, nextV))
		lastV = nextV
		inputIdx++
	}
	if opts.SubtitlePath != "" {
		args = append(args, "-i", opts.SubtitlePath)
		nextV := "vsub"
		filterParts = append(filterParts, fmt.Sprintf("[%s][%d:v]overlay=(W-w)/2:H-h-48[%s]", lastV, inputIdx, nextV))
		lastV = nextV
		inputIdx++
	}

	args = append(args,
		"-filter_complex", strings.Join(filterParts, ";"),
		"-map", fmt.Sprintf("[%s]", lastV),
		"-map", audioMap,
		"-c:v", encoder, "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-shortest", outPath,
	)
	if out, err := exec.CommandContext(ctx, ffmpegExe, args...).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	vm, err := VerifyMediaFile(ctx, outPath, MediaVerifyOptions{
		FFprobeExe:   ffprobeExe,
		MinFileBytes: 256,
		RequireAudio: true,
	})
	if err != nil {
		return 0, err
	}
	return vm.DurationSec, nil
}

func renderXfadeGroup(ctx context.Context, ffmpegExe, encoder string, segs []preparedSegment, outPath string) (float64, error) {
	args := []string{"-y"}
	for _, s := range segs {
		args = append(args, "-i", s.path)
	}
	var filters []string
	curV := "0:v"
	curA := "0:a"
	accDur := segs[0].duration
	for i := 1; i < len(segs); i++ {
		tdur := segs[i-1].transitionDur
		if tdur <= 0 || tdur >= accDur || tdur >= segs[i].duration {
			tdur = math.Min(0.4, math.Min(accDur, segs[i].duration)*0.3)
		}
		offset := accDur - tdur
		if offset < 0 {
			offset = 0
		}
		outV := fmt.Sprintf("xv%d", i)
		outA := fmt.Sprintf("xa%d", i)
		filters = append(filters, fmt.Sprintf("[%s][%d:v]xfade=transition=%s:duration=%.3f:offset=%.3f[%s]",
			curV, i, segs[i-1].transitionOut, tdur, offset, outV))
		filters = append(filters, fmt.Sprintf("[%s][%d:a]acrossfade=d=%.3f[%s]",
			curA, i, tdur, outA))
		curV = outV
		curA = outA
		accDur = accDur + segs[i].duration - tdur
	}

	args = append(args,
		"-filter_complex", strings.Join(filters, ";"),
		"-map", fmt.Sprintf("[%s]", curV),
		"-map", fmt.Sprintf("[%s]", curA),
		"-c:v", encoder, "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		outPath,
	)
	if out, err := exec.CommandContext(ctx, ffmpegExe, args...).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return accDur, nil
}

func concatSegments(ctx context.Context, ffmpegExe string, paths []string, outPath string) error {
	listPath := outPath + ".txt"
	var sb strings.Builder
	for _, p := range paths {
		escaped := strings.ReplaceAll(p, "'", "'\\''")
		sb.WriteString(fmt.Sprintf("file '%s'\n", escaped))
	}
	if err := os.WriteFile(listPath, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	defer os.Remove(listPath)

	out, err := exec.CommandContext(ctx, ffmpegExe,
		"-y", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", outPath,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// audibleTracks drops muted and silent tracks, and rejects entries without a path.
func audibleTracks(tracks []AudioTrack) []AudioTrack {
	out := make([]AudioTrack, 0, len(tracks))
	for _, t := range tracks {
		if t.Mute || t.EffectiveVolume() == 0 || strings.TrimSpace(t.Path) == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func mixBGM(ctx context.Context, ffmpegExe, videoPath, bgmPath, outPath string, totalDur, clipVol, bgmVol, fadeSec float64) error {
	if fadeSec*2 > totalDur {
		fadeSec = totalDur * 0.25
	}
	fadeOutStart := math.Max(0, totalDur-fadeSec)
	fc := fmt.Sprintf("[0:a]volume=%.3f[va];[1:a]atrim=0:%.3f,asetpts=PTS-STARTPTS,afade=t=in:st=0:d=%.3f,afade=t=out:st=%.3f:d=%.3f,volume=%.3f[ba];[va][ba]amix=inputs=2:duration=first:normalize=0[aout]",
		clipVol, totalDur, fadeSec, fadeOutStart, fadeSec, bgmVol)

	out, err := exec.CommandContext(ctx, ffmpegExe,
		"-y",
		"-i", videoPath,
		"-stream_loop", "-1", "-i", bgmPath,
		"-filter_complex", fc,
		"-map", "0:v",
		"-map", "[aout]",
		"-c:v", "copy",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-shortest",
		outPath,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func renderSubtitlePNG(text string, videoWidth int, outputPath string) error {
	w := videoWidth - 160
	if w < 640 {
		w = 640
	}
	const h = 76
	face, err := fonts.NewFace(fonts.NotoSansSCBold, 34)
	if err != nil {
		return err
	}
	defer face.Close()

	dc := gg.NewContext(w, h)
	dc.SetFontFace(face)
	dc.SetRGBA(0.04, 0.06, 0.10, 0.65)
	dc.DrawRoundedRectangle(0, 0, float64(w), float64(h), 12)
	dc.Fill()

	dc.SetRGBA(0.0, 0.0, 0.0, 0.90)
	dc.DrawStringAnchored(text, float64(w)/2+2, float64(h)/2+2, 0.5, 0.5)
	dc.SetRGBA(0.98, 0.99, 1.0, 1.0)
	dc.DrawStringAnchored(text, float64(w)/2, float64(h)/2, 0.5, 0.5)

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	if err := png.Encode(f, dc.Image()); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func normalizeTransitionName(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "fade", "fadewhite", "wipeleft", "wiperight", "smoothleft":
		return strings.ToLower(strings.TrimSpace(t))
	default:
		return ""
	}
}

func isImageExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".bmp", ".webp":
		return true
	default:
		return false
	}
}

func probeHasAudio(ctx context.Context, ffprobeExe, path string) (bool, error) {
	out, err := exec.CommandContext(ctx, ffprobeExe,
		"-v", "error", "-select_streams", "a", "-show_entries", "stream=index", "-of", "csv=p=0", path,
	).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
