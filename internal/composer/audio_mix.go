package composer

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// AudioTrack is one externally supplied audio file layered onto a render, such
// as team comms captured from a demo or an imported voice-over. OffsetSec may be
// negative to trim the head of the track (cameras/mics rarely start aligned),
// Volume is a linear gain (0 means "use 1.0"), and Mute drops the track while
// keeping it in the spec so the UI can toggle it without losing the path.
type AudioTrack struct {
	Path      string  `json:"path"`
	OffsetSec float64 `json:"offset_sec,omitempty"`
	Volume    float64 `json:"volume,omitempty"`
	Mute      bool    `json:"mute,omitempty"`
}

// DefaultTrackVolume is applied when a track leaves Volume at zero.
const DefaultTrackVolume = 1.0

// EffectiveVolume resolves the gain applied to this track, honouring Mute.
func (t AudioTrack) EffectiveVolume() float64 {
	if t.Mute {
		return 0
	}
	if t.Volume <= 0 {
		return DefaultTrackVolume
	}
	if t.Volume > 4 {
		return 4
	}
	return t.Volume
}

// ParseAudioTrackSpec parses `path`, `path@<offset>` or `path@<offset>:<volume>`
// (offset/volume may be signed decimals). Windows paths keep their drive colon
// because the volume separator is only recognised after the '@'.
func ParseAudioTrackSpec(spec string) (AudioTrack, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return AudioTrack{}, fmt.Errorf("empty audio track spec")
	}
	track := AudioTrack{}
	if i := strings.LastIndex(spec, "@"); i == 0 && looksNumeric(spec[1:]) {
		return AudioTrack{}, fmt.Errorf("audio track is missing a file path")
	} else if i > 0 && looksNumeric(spec[i+1:]) {
		rest := spec[i+1:]
		track.Path = spec[:i]
		if j := strings.LastIndex(rest, ":"); j >= 0 {
			off, err := strconv.ParseFloat(strings.TrimSpace(rest[:j]), 64)
			if err != nil {
				return AudioTrack{}, fmt.Errorf("parse offset %q: %w", rest[:j], err)
			}
			vol, err := strconv.ParseFloat(strings.TrimSpace(rest[j+1:]), 64)
			if err != nil {
				return AudioTrack{}, fmt.Errorf("parse volume %q: %w", rest[j+1:], err)
			}
			if vol < 0 {
				return AudioTrack{}, fmt.Errorf("volume must be >= 0")
			}
			track.OffsetSec, track.Volume = off, vol
		} else {
			off, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
			if err != nil {
				return AudioTrack{}, fmt.Errorf("parse offset %q: %w", rest, err)
			}
			track.OffsetSec = off
		}
	} else {
		track.Path = spec
	}
	if strings.TrimSpace(track.Path) == "" {
		return AudioTrack{}, fmt.Errorf("audio track is missing a file path")
	}
	if track.Path == "mute" {
		return AudioTrack{}, fmt.Errorf("audio track spec needs a file path")
	}
	if math.IsNaN(track.OffsetSec) || math.IsInf(track.OffsetSec, 0) {
		return AudioTrack{}, fmt.Errorf("invalid offset")
	}
	return track, nil
}

// looksNumeric reports whether s starts like an offset/volume so filenames that
// merely contain '@' are still treated as paths.
func looksNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	switch c := s[0]; {
	case c >= '0' && c <= '9':
		return true
	case c == '-', c == '+', c == '.':
		return true
	default:
		return false
	}
}

// MixAudioTracks layers `tracks` onto the audio of videoPath and writes outPath
// (video stream copied, audio re-encoded to stereo AAC). totalDur bounds every
// track so a long voice-over cannot extend the video; baseVolume scales the
// existing audio and is ignored when the video has none.
func MixAudioTracks(ctx context.Context, ffmpegExe, videoPath string, tracks []AudioTrack, outPath string, totalDur, baseVolume float64) error {
	if ffmpegExe == "" {
		ffmpegExe = "ffmpeg"
	}
	active := make([]AudioTrack, 0, len(tracks))
	for _, t := range tracks {
		if t.Mute || t.EffectiveVolume() == 0 || strings.TrimSpace(t.Path) == "" {
			continue
		}
		active = append(active, t)
	}
	if len(active) == 0 {
		return fmt.Errorf("no audible audio tracks to mix")
	}
	if totalDur <= 0 {
		return fmt.Errorf("total duration must be positive")
	}
	if baseVolume <= 0 {
		baseVolume = 1.0
	}

	args := []string{"-y", "-i", videoPath}
	for _, t := range active {
		args = append(args, "-i", t.Path)
	}

	parts := []string{fmt.Sprintf("[0:a]volume=%.3f[vbase]", baseVolume)}
	mixInputs := []string{"[vbase]"}
	for i, t := range active {
		offsetMS := int(math.Round(t.OffsetSec * 1000))
		chain := fmt.Sprintf("volume=%.3f", t.EffectiveVolume())
		if offsetMS < 0 {
			// Negative offset trims the head of the track.
			chain = fmt.Sprintf("atrim=start=%.3f,asetpts=PTS-STARTPTS,%s", math.Abs(t.OffsetSec), chain)
			offsetMS = 0
		}
		if offsetMS > 0 {
			// adelay needs one value per channel; all=1 applies the same delay.
			chain = fmt.Sprintf("%s,adelay=%d:all=1", chain, offsetMS)
		}
		label := fmt.Sprintf("[a%d]", i)
		parts = append(parts, fmt.Sprintf("[%d:a]atrim=0:%.3f,asetpts=PTS-STARTPTS,%s%s", i+1, totalDur, chain, label))
		mixInputs = append(mixInputs, label)
	}
	parts = append(parts, fmt.Sprintf("%samix=inputs=%d:duration=first:normalize=0[aout]",
		strings.Join(mixInputs, ""), len(mixInputs)))

	args = append(args,
		"-filter_complex", strings.Join(parts, ";"),
		"-map", "0:v",
		"-map", "[aout]",
		"-c:v", "copy",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-t", fmt.Sprintf("%.3f", totalDur),
		outPath,
	)
	if out, err := exec.CommandContext(ctx, ffmpegExe, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ExtractAudio writes the mixed audio alone as m4a, which is what "audio only"
// exports and podcast/voice-review workflows need.
func ExtractAudio(ctx context.Context, ffmpegExe, videoPath, outPath string, totalDur float64) error {
	if ffmpegExe == "" {
		ffmpegExe = "ffmpeg"
	}
	args := []string{"-y", "-i", videoPath, "-vn", "-c:a", "aac", "-b:a", "160k", "-ar", "48000", "-ac", "2"}
	if totalDur > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", totalDur))
	}
	args = append(args, outPath)
	if out, err := exec.CommandContext(ctx, ffmpegExe, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// AudioTrackExt returns the preferred container extension for audio-only output.
func AudioTrackExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return ""
	}
	return ext
}
