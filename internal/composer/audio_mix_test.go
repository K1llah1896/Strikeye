package composer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAudioTrackSpec(t *testing.T) {
	cases := []struct {
		spec string
		want AudioTrack
	}{
		{"comms.wav", AudioTrack{Path: "comms.wav"}},
		{"comms.wav@1.5", AudioTrack{Path: "comms.wav", OffsetSec: 1.5}},
		{"comms.wav@-2", AudioTrack{Path: "comms.wav", OffsetSec: -2}},
		{"comms.wav@0.5:0.35", AudioTrack{Path: "comms.wav", OffsetSec: 0.5, Volume: 0.35}},
		{"C:/media/comms.wav@1", AudioTrack{Path: "C:/media/comms.wav", OffsetSec: 1}},
		{" /tmp/a@b.wav ", AudioTrack{Path: "/tmp/a@b.wav"}},
	}
	for _, tc := range cases {
		got, err := ParseAudioTrackSpec(tc.spec)
		if err != nil {
			t.Fatalf("%s: %v", tc.spec, err)
		}
		if got.Path != tc.want.Path || got.OffsetSec != tc.want.OffsetSec || got.Volume != tc.want.Volume {
			t.Fatalf("%s: got %+v want %+v", tc.spec, got, tc.want)
		}
	}
	// "a.wav@x" is a legitimate filename, so only genuinely malformed offsets fail.
	for _, bad := range []string{"", "   ", "mute", "@1.0", "a.wav@1:x", "a.wav@1:-1"} {
		if _, err := ParseAudioTrackSpec(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestAudioTrackEffectiveVolume(t *testing.T) {
	cases := []struct {
		track AudioTrack
		want  float64
	}{
		{AudioTrack{}, DefaultTrackVolume},
		{AudioTrack{Volume: 0.25}, 0.25},
		{AudioTrack{Volume: 9}, 4},
		{AudioTrack{Mute: true, Volume: 1}, 0},
	}
	for _, tc := range cases {
		if got := tc.track.EffectiveVolume(); got != tc.want {
			t.Fatalf("%+v: got %v want %v", tc.track, got, tc.want)
		}
	}
}

func TestAudibleTracksFiltersMutedAndBlank(t *testing.T) {
	got := audibleTracks([]AudioTrack{
		{Path: "a.wav"},
		{Path: "b.wav", Mute: true},
		{Path: "c.wav", Volume: 0.5},
		{Path: "  "},
	})
	if len(got) != 2 || got[0].Path != "a.wav" || got[1].Path != "c.wav" {
		t.Fatalf("unexpected tracks: %+v", got)
	}
}

// genTone writes a sine wave of the given frequency/duration so mixing can be
// verified by loudness rather than by eyeballing ffmpeg output.
func genTone(t *testing.T, path string, freq float64, dur float64) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	tone := fmt.Sprintf("sine=frequency=%.0f:duration=%.3f:sample_rate=48000", freq, dur)
	args := []string{"-y"}
	if strings.HasSuffix(strings.ToLower(path), ".wav") {
		args = append(args, "-f", "lavfi", "-i", tone, "-ac", "2", path)
	} else {
		args = append(args,
			"-f", "lavfi", "-i", fmt.Sprintf("color=c=black:s=160x90:r=15:d=%.3f", dur),
			"-f", "lavfi", "-i", tone,
			"-map", "0:v", "-map", "1:a",
			"-ac", "2", "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", path)
	}
	cmd := exec.Command(ffmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gen tone: %v: %s", err, out)
	}
}

func TestMixAudioTracksLayersVoiceAndKeepsDuration(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "base.mp4")
	genTone(t, video, 440, 3)
	voice := filepath.Join(dir, "voice.wav")
	genTone(t, voice, 880, 5)

	mixed := filepath.Join(dir, "mixed.mp4")
	if err := MixAudioTracks(context.Background(), ffmpeg, video,
		[]AudioTrack{{Path: voice, OffsetSec: 0.5, Volume: 0.6}}, mixed, 3, 0.8); err != nil {
		t.Fatalf("mix: %v", err)
	}
	info, err := VerifyMediaFile(context.Background(), mixed, MediaVerifyOptions{
		MinFileBytes: 256, RequireAudio: true,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if info.DurationSec < 2.5 || info.DurationSec > 3.5 {
		t.Fatalf("duration drift: %v", info.DurationSec)
	}
	if !info.HasAudio {
		t.Fatal("mixed file lost its audio stream")
	}

	// A muted-only track list is a caller error, not a silent no-op.
	if err := MixAudioTracks(context.Background(), ffmpeg, video,
		[]AudioTrack{{Path: voice, Mute: true}}, filepath.Join(dir, "x.mp4"), 3, 1); err == nil {
		t.Fatal("expected error when every track is muted")
	}

	audioOnly := filepath.Join(dir, "audio.m4a")
	if err := ExtractAudio(context.Background(), ffmpeg, mixed, audioOnly, 3); err != nil {
		t.Fatalf("extract: %v", err)
	}
	st, err := os.Stat(audioOnly)
	if err != nil || st.Size() < 1000 {
		t.Fatalf("audio-only export missing or empty: %v", err)
	}
}
