package recorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"strikeye/internal/composer"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

// CapturedStreams describes raw video/image-sequence and audio assets produced
// by HLAE mirv_streams in a capture directory.
type CapturedStreams struct {
	VideoInput   string // path to video file or printf pattern (e.g. %05d.tga)
	AudioWAV     string // optional path to .wav audio
	IsImageSeq   bool
	ImagePattern string
}

// MuxVerifyOptions configures post-capture FFmpeg muxing and ffprobe gating.
type MuxVerifyOptions struct {
	TakeDir           string
	PrebuiltMP4Path   string
	OutputMP4Path     string
	FFmpegExe         string
	FFprobeExe        string
	PreferredEncoder  string
	FPS               int
	StartTick         int64
	EndTick           int64
	TickRate          float64
	KeyOverlayEnabled bool
	InputEdges        []parser.InputEdge
	PlayerSlot        int
	FrameMeld         composer.FrameMeldConfig
	MinFileBytes      int64
	RequireAudio      bool
}

// ResolveCapturedStreams inspects takeDir (including nested takeXXXX folders)
// for mirv_streams video/image-sequence and WAV audio outputs.
func ResolveCapturedStreams(takeDir string) (CapturedStreams, error) {
	if takeDir == "" {
		return CapturedStreams{}, fmt.Errorf("take_dir is required")
	}
	var videos, images, wavs []string
	err := filepath.WalkDir(takeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		switch ext {
		case ".mp4", ".avi", ".mkv":
			videos = append(videos, path)
		case ".tga", ".png", ".jpg", ".jpeg":
			images = append(images, path)
		case ".wav":
			wavs = append(wavs, path)
		}
		return nil
	})
	if err != nil {
		return CapturedStreams{}, fmt.Errorf("scan take_dir %q: %w", takeDir, err)
	}
	sort.Strings(videos)
	sort.Strings(images)
	sort.Strings(wavs)

	var out CapturedStreams
	if len(wavs) > 0 {
		out.AudioWAV = wavs[0]
	}
	if len(videos) > 0 {
		out.VideoInput = videos[0]
		return out, nil
	}
	if len(images) > 0 {
		out.IsImageSeq = true
		out.VideoInput = images[0]
		out.ImagePattern = filepath.Join(filepath.Dir(images[0]), "*"+filepath.Ext(images[0]))
		return out, nil
	}
	return CapturedStreams{}, fmt.Errorf("no video or image sequence found in %q", takeDir)
}

// MuxAndVerifyTask muxes raw captured streams into OutputMP4Path via composer,
// validates the resulting MP4 with ffprobe, and persists it into media_videos.
func MuxAndVerifyTask(ctx context.Context, db *storage.DB, task storage.RenderTask, opts MuxVerifyOptions) (storage.MediaVideo, error) {
	if db == nil {
		return storage.MediaVideo{}, fmt.Errorf("db is required")
	}
	if opts.OutputMP4Path == "" {
		return storage.MediaVideo{}, fmt.Errorf("output_mp4_path is required")
	}
	if err := os.MkdirAll(filepath.Dir(opts.OutputMP4Path), 0o755); err != nil {
		return storage.MediaVideo{}, fmt.Errorf("create output dir: %w", err)
	}

	ffmpegExe := opts.FFmpegExe
	if ffmpegExe == "" {
		ffmpegExe = "ffmpeg"
	}
	ffprobeExe := opts.FFprobeExe
	if ffprobeExe == "" {
		ffprobeExe = "ffprobe"
	}

	workingMP4 := opts.OutputMP4Path
	if opts.PrebuiltMP4Path != "" {
		if opts.PrebuiltMP4Path != opts.OutputMP4Path {
			if err := copyFileAtomic(opts.PrebuiltMP4Path, opts.OutputMP4Path); err != nil {
				return storage.MediaVideo{}, fmt.Errorf("stage prebuilt mp4: %w", err)
			}
		}
	} else {
		streams, err := ResolveCapturedStreams(opts.TakeDir)
		if err != nil {
			return storage.MediaVideo{}, err
		}
		enc, err := composer.ProbeEncoder(ctx, ffmpegExe, opts.PreferredEncoder)
		if err != nil {
			return storage.MediaVideo{}, fmt.Errorf("probe encoder: %w", err)
		}
		if err := muxStreamsToMP4(ctx, ffmpegExe, enc.Selected, opts.FPS, streams, workingMP4); err != nil {
			return storage.MediaVideo{}, err
		}
	}

	hasKeyOverlay := false
	if opts.KeyOverlayEnabled && len(opts.InputEdges) > 0 {
		hudOut := workingMP4 + ".hud.mp4"
		err := composer.ApplyKeyOverlay(ctx, workingMP4, hudOut, composer.KeyOverlayOptions{
			FFmpegExe:  ffmpegExe,
			Edges:      opts.InputEdges,
			PlayerSlot: opts.PlayerSlot,
			StartTick:  opts.StartTick,
			EndTick:    opts.EndTick,
			TickRate:   opts.TickRate,
			FPS:        float64(opts.FPS),
		})
		if err != nil {
			return storage.MediaVideo{}, fmt.Errorf("apply key overlay: %w", err)
		}
		if err := os.Rename(hudOut, workingMP4); err != nil {
			return storage.MediaVideo{}, fmt.Errorf("promote hud mp4: %w", err)
		}
		hasKeyOverlay = true
	}

	if opts.FrameMeld.Enabled {
		fmOut := workingMP4 + ".framemeld.mp4"
		if _, err := composer.RunFrameMeld(ctx, opts.FrameMeld, workingMP4, fmOut); err != nil {
			return storage.MediaVideo{}, fmt.Errorf("framemeld: %w", err)
		}
		if err := os.Rename(fmOut, workingMP4); err != nil {
			return storage.MediaVideo{}, fmt.Errorf("promote framemeld mp4: %w", err)
		}
	}

	verified, err := composer.VerifyMediaFile(ctx, workingMP4, composer.MediaVerifyOptions{
		FFprobeExe:   ffprobeExe,
		MinFileBytes: opts.MinFileBytes,
		RequireAudio: opts.RequireAudio,
	})
	if err != nil {
		return storage.MediaVideo{}, fmt.Errorf("ffprobe quality gate failed: %w", err)
	}

	steamID := task.TargetSteamID64
	playerName := task.TargetPlayerName
	scope := task.TaskScope
	if scope == "" {
		scope = "highlight_clip"
	}
	media, err := db.Media().Upsert(ctx, storage.MediaVideo{
		DemoID:          task.DemoID,
		RoundNumber:     task.RoundNumber,
		PlayerSteamID64: &steamID,
		PlayerName:      &playerName,
		Perspective:     task.Perspective,
		VideoScope:      scope,
		StartTick:       opts.StartTick,
		EndTick:         opts.EndTick,
		SegmentsJSON:    task.SegmentsJSON,
		FilePath:        workingMP4,
		FileSize:        verified.FileSize,
		DurationSec:     verified.DurationSec,
		Width:           verified.Width,
		Height:          verified.Height,
		FPS:             verified.FPS,
		VideoCodec:      verified.VideoCodec,
		HasAudio:        verified.HasAudio,
		HasKeyOverlay:   hasKeyOverlay,
		VerifiedStatus:  "verified",
	})
	if err != nil {
		return storage.MediaVideo{}, fmt.Errorf("persist verified media: %w", err)
	}
	return media, nil
}

func muxStreamsToMP4(ctx context.Context, ffmpegExe, codec string, fps int, streams CapturedStreams, outMP4 string) error {
	if fps <= 0 {
		fps = 60
	}
	var args []string
	args = append(args, "-y")
	if streams.IsImageSeq {
		args = append(args, "-framerate", fmt.Sprintf("%d", fps), "-pattern_type", "glob", "-i", streams.ImagePattern)
	} else {
		args = append(args, "-i", streams.VideoInput)
	}
	if streams.AudioWAV != "" {
		args = append(args, "-i", streams.AudioWAV)
	}
	args = append(args, "-c:v", codec, "-pix_fmt", "yuv420p")
	if streams.AudioWAV != "" {
		args = append(args, "-c:a", "aac", "-ar", "48000", "-shortest")
	}
	args = append(args, outMP4)

	out, err := exec.CommandContext(ctx, ffmpegExe, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg mux failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
