// Package composer implements Strikeye's FFmpeg hardware encoder probing,
// RGBA-piped key HUD overlay, lower-third player name cards, optional FrameMeld
// adapter, and montage export pipeline with ffprobe verification.
package composer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// DefaultEncoderPriority defines the hardware-first H.264 encoder probing order.
var DefaultEncoderPriority = []string{
	"h264_nvenc",
	"h264_qsv",
	"h264_amf",
	"libx264",
}

// ProbeCandidate records the outcome of testing one encoder.
type ProbeCandidate struct {
	Encoder   string `json:"encoder"`
	Listed    bool   `json:"listed"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// ProbeResult summarizes encoder probing.
type ProbeResult struct {
	Selected   string           `json:"selected"`
	Candidates []ProbeCandidate `json:"candidates"`
}

// ProbeEncoder checks ffmpeg -encoders and runs a 1-frame 64x64 smoke encode
// for each candidate in priority order, returning the first working H.264 encoder.
func ProbeEncoder(ctx context.Context, ffmpegExe, preferred string) (ProbeResult, error) {
	if ffmpegExe == "" {
		ffmpegExe = "ffmpeg"
	}
	listOut, err := exec.CommandContext(ctx, ffmpegExe, "-hide_banner", "-encoders").CombinedOutput()
	if err != nil {
		return ProbeResult{}, fmt.Errorf("list ffmpeg encoders: %w (%s)", err, strings.TrimSpace(string(listOut)))
	}
	listed := string(listOut)

	order := make([]string, 0, len(DefaultEncoderPriority)+1)
	if preferred != "" && preferred != "auto" {
		order = append(order, preferred)
	}
	for _, enc := range DefaultEncoderPriority {
		if enc != preferred {
			order = append(order, enc)
		}
	}

	var res ProbeResult
	for _, enc := range order {
		cand := ProbeCandidate{
			Encoder: enc,
			Listed:  strings.Contains(listed, enc),
		}
		if !cand.Listed {
			cand.Error = "not listed by ffmpeg -encoders"
			res.Candidates = append(res.Candidates, cand)
			continue
		}
		if err := smokeEncode1Frame(ctx, ffmpegExe, enc); err != nil {
			cand.Error = err.Error()
			res.Candidates = append(res.Candidates, cand)
			continue
		}
		cand.Available = true
		res.Candidates = append(res.Candidates, cand)
		if res.Selected == "" {
			res.Selected = enc
			break
		}
	}

	if res.Selected == "" {
		return res, fmt.Errorf("no working H.264 encoder found among %v", order)
	}
	return res, nil
}

func smokeEncode1Frame(ctx context.Context, ffmpegExe, encoder string) error {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "lavfi",
		"-i", "color=c=black:s=128x128:d=0.04",
		"-frames:v", "1",
		"-c:v", encoder,
		"-pix_fmt", "yuv420p",
		"-f", "null",
		"-",
	}
	out, err := exec.CommandContext(ctx, ffmpegExe, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// MediaVerifyOptions configures ffprobe validation thresholds.
type MediaVerifyOptions struct {
	FFprobeExe         string
	MinFileBytes       int64   // defaults to 65536 (64 KB) when <= 0
	ExpectedDuration   float64 // > 0 enables duration tolerance check
	DurationTolerance  float64 // defaults to 0.5s when <= 0
	RequireAudio       bool
	ExpectedSampleRate int // defaults to 48000 when RequireAudio is true and <= 0
}

// VerifiedMedia holds metadata extracted by ffprobe after passing all checks.
type VerifiedMedia struct {
	FilePath    string  `json:"file_path"`
	FileSize    int64   `json:"file_size"`
	DurationSec float64 `json:"duration_sec"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FPS         float64 `json:"fps"`
	VideoCodec  string  `json:"video_codec"`
	HasAudio    bool    `json:"has_audio"`
	SampleRate  int     `json:"sample_rate"`
}

// VerifyMediaFile runs ffprobe on filePath and enforces file size, video/audio stream,
// and duration tolerance checks.
func VerifyMediaFile(ctx context.Context, filePath string, opts MediaVerifyOptions) (VerifiedMedia, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return VerifiedMedia{}, fmt.Errorf("stat media %q: %w", filePath, err)
	}
	minBytes := opts.MinFileBytes
	if minBytes <= 0 {
		minBytes = 64 * 1024
	}
	if info.Size() < minBytes {
		return VerifiedMedia{}, fmt.Errorf("media %q size %d < minimum %d bytes", filePath, info.Size(), minBytes)
	}

	ffprobeExe := opts.FFprobeExe
	if ffprobeExe == "" {
		ffprobeExe = "ffprobe"
	}
	out, err := exec.CommandContext(
		ctx,
		ffprobeExe,
		"-v", "error",
		"-show_format",
		"-show_streams",
		"-of", "json",
		filePath,
	).CombinedOutput()
	if err != nil {
		return VerifiedMedia{}, fmt.Errorf("ffprobe %q: %w (%s)", filePath, err, strings.TrimSpace(string(out)))
	}

	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			RFrameRate   string `json:"r_frame_rate"`
			AvgFrameRate string `json:"avg_frame_rate"`
			NbFrames     string `json:"nb_frames"`
			SampleRate   string `json:"sample_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return VerifiedMedia{}, fmt.Errorf("parse ffprobe json: %w", err)
	}

	dur, err := strconv.ParseFloat(parsed.Format.Duration, 64)
	if err != nil || dur <= 0 {
		return VerifiedMedia{}, fmt.Errorf("invalid duration %q in %q", parsed.Format.Duration, filePath)
	}

	res := VerifiedMedia{
		FilePath:    filePath,
		FileSize:    info.Size(),
		DurationSec: dur,
	}
	var hasVideo bool
	for _, st := range parsed.Streams {
		switch st.CodecType {
		case "video":
			if !hasVideo && st.Width > 0 && st.Height > 0 {
				hasVideo = true
				res.Width = st.Width
				res.Height = st.Height
				res.VideoCodec = st.CodecName
				res.FPS = parseFFprobeFPS(st.AvgFrameRate, st.RFrameRate)
				if st.NbFrames != "" {
					if n, nErr := strconv.ParseInt(st.NbFrames, 10, 64); nErr == nil && n <= 0 {
						return VerifiedMedia{}, fmt.Errorf("video stream has nb_frames=%d", n)
					}
				}
			}
		case "audio":
			if !res.HasAudio {
				res.HasAudio = true
				if sr, srErr := strconv.Atoi(st.SampleRate); srErr == nil {
					res.SampleRate = sr
				}
			}
		}
	}

	if !hasVideo {
		return VerifiedMedia{}, fmt.Errorf("no valid video stream found in %q", filePath)
	}
	if opts.RequireAudio {
		if !res.HasAudio {
			return VerifiedMedia{}, fmt.Errorf("missing required audio stream in %q", filePath)
		}
		expSR := opts.ExpectedSampleRate
		if expSR <= 0 {
			expSR = 48000
		}
		if res.SampleRate != expSR {
			return VerifiedMedia{}, fmt.Errorf("audio sample rate %d != expected %d", res.SampleRate, expSR)
		}
	}
	if opts.ExpectedDuration > 0 {
		tol := opts.DurationTolerance
		if tol <= 0 {
			tol = 0.5
		}
		if math.Abs(res.DurationSec-opts.ExpectedDuration) > tol {
			return VerifiedMedia{}, fmt.Errorf("duration %.3fs differs from expected %.3fs by > %.3fs", res.DurationSec, opts.ExpectedDuration, tol)
		}
	}
	return res, nil
}

func parseFFprobeFPS(avg, raw string) float64 {
	for _, s := range []string{avg, raw} {
		if s == "" || s == "0/0" {
			continue
		}
		if parts := strings.Split(s, "/"); len(parts) == 2 {
			num, err1 := strconv.ParseFloat(parts[0], 64)
			den, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 == nil && err2 == nil && den > 0 {
				return num / den
			}
		}
		if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 {
			return v
		}
	}
	return 0
}
