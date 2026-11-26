package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"strikeye/internal/composer"
)

// isAudioExt reports whether path names an audio-only container.
func isAudioExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m4a", ".aac", ".mp3", ".wav", ".ogg", ".flac", ".opus":
		return true
	}
	return false
}

// mixaudioOptions drives the `mixaudio` subcommand.
type mixaudioOptions struct {
	videoPath  string
	trackSpecs []string
	outPath    string
	baseVolume float64
	audioOnly  bool
	ffmpegExe  string
}

// runMixAudio layers voice/comms tracks onto an existing clip and optionally
// exports the mixed audio alone.
func runMixAudio(ctx context.Context, opts mixaudioOptions, out io.Writer) error {
	if strings.TrimSpace(opts.videoPath) == "" {
		return fmt.Errorf("--video is required")
	}
	if len(opts.trackSpecs) == 0 {
		return fmt.Errorf("at least one --track is required")
	}
	tracks := make([]composer.AudioTrack, 0, len(opts.trackSpecs))
	for _, spec := range opts.trackSpecs {
		track, err := composer.ParseAudioTrackSpec(spec)
		if err != nil {
			return err
		}
		tracks = append(tracks, track)
	}

	ffprobe := "ffprobe"
	info, err := composer.VerifyMediaFile(ctx, opts.videoPath, composer.MediaVerifyOptions{
		FFprobeExe:   ffprobe,
		MinFileBytes: 256,
		RequireAudio: true,
	})
	if err != nil {
		return fmt.Errorf("probe input: %w", err)
	}

	target := opts.outPath
	if target == "" {
		ext := ".mp4"
		if opts.audioOnly {
			ext = ".m4a"
		}
		base := strings.TrimSuffix(filepath.Base(opts.videoPath), filepath.Ext(opts.videoPath))
		target = filepath.Join(filepath.Dir(opts.videoPath), base+"_mixed"+ext)
	}

	encoder := opts.ffmpegExe
	// Audio-only targets need a video container for the intermediate mix,
	// otherwise ffmpeg would be asked to write the m4a in place.
	mixTarget := target
	audioOnly := opts.audioOnly || isAudioExt(target)
	intermediate := ""
	if audioOnly && isAudioExt(target) {
		intermediate = strings.TrimSuffix(target, filepath.Ext(target)) + "_mixed.mp4"
		mixTarget = intermediate
	}
	if err := composer.MixAudioTracks(ctx, encoder, opts.videoPath, tracks, mixTarget, info.DurationSec, opts.baseVolume); err != nil {
		return err
	}
	audioPath := ""
	if audioOnly {
		audioPath = target
		if !isAudioExt(audioPath) {
			audioPath = strings.TrimSuffix(target, filepath.Ext(target)) + ".m4a"
		}
		if err := composer.ExtractAudio(ctx, encoder, mixTarget, audioPath, info.DurationSec); err != nil {
			return err
		}
	}
	if intermediate != "" {
		_ = os.Remove(intermediate)
	}
	result := map[string]any{
		"output":       target,
		"tracks":       len(tracks),
		"duration_sec": info.DurationSec,
	}
	if audioPath != "" {
		result["audio_only"] = audioPath
	}

	verified, err := composer.VerifyMediaFile(ctx, target, composer.MediaVerifyOptions{
		MinFileBytes: 1024, RequireAudio: true,
	})
	if err == nil {
		result["verified"] = map[string]any{
			"duration_sec": verified.DurationSec,
			"has_audio":    verified.HasAudio,
			"file_size":    verified.FileSize,
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
