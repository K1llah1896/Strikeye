package recorder

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"strikeye/internal/storage"
)

func TestResolveCapturedStreams(t *testing.T) {
	tmp := t.TempDir()
	takeDir := filepath.Join(tmp, "take0000")
	if err := os.MkdirAll(takeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(takeDir, "00001.tga"), []byte("tga"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(takeDir, "audio.wav"), []byte("wav"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := ResolveCapturedStreams(tmp)
	if err != nil {
		t.Fatalf("ResolveCapturedStreams: %v", err)
	}
	if !s.IsImageSeq || s.AudioWAV == "" {
		t.Fatalf("expected image sequence + wav, got %+v", s)
	}
}

func TestMuxAndVerifyTaskQualityGate(t *testing.T) {
	tmp := t.TempDir()
	db, err := storage.OpenDefault(tmp)
	if err != nil {
		t.Fatalf("OpenDefault: %v", err)
	}
	defer db.Close()

	mapName := "de_mirage"
	demo, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
		Filename:      "test.dem",
		FilePath:      filepath.Join(tmp, "test.dem"),
		ContentSHA256: "abc123",
		MapName:       &mapName,
		TickRate:      64,
		DemoEndTick:   10000,
	})
	if err != nil {
		t.Fatal(err)
	}

	task, _, err := db.RenderTasks().EnqueueIdempotent(context.Background(), storage.RenderTask{
		TaskUID:          "uid-verify-1",
		DemoID:           demo.ID,
		TargetSteamID64:  "76561198000000001",
		TargetPlayerName: "donk",
		TargetSpecSlot:   1,
		Perspective:      "pov",
		TaskScope:        "highlight_clip",
		SegmentsJSON:     `[{"start_tick":100,"end_tick":300}]`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a fake ffprobe script that returns valid JSON on demand.
	ffprobeScript := filepath.Join(tmp, "ffprobe-ok.sh")
	scriptBody := `#!/bin/sh
cat <<'EOF'
{"format":{"duration":"3.125"},"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"r_frame_rate":"60/1","avg_frame_rate":"60/1"},{"codec_type":"audio","codec_name":"aac","sample_rate":"48000"}]}
EOF
`
	if err := os.WriteFile(ffprobeScript, []byte(scriptBody), 0o755); err != nil {
		t.Fatal(err)
	}

	prebuilt := filepath.Join(tmp, "raw.mp4")
	if err := os.WriteFile(prebuilt, make([]byte, 70*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	outMP4 := filepath.Join(tmp, "out", "clip.mp4")

	media, err := MuxAndVerifyTask(context.Background(), db, task, MuxVerifyOptions{
		PrebuiltMP4Path: prebuilt,
		OutputMP4Path:   outMP4,
		FFprobeExe:      ffprobeScript,
		StartTick:       100,
		EndTick:         300,
		TickRate:        64,
		RequireAudio:    true,
	})
	if err != nil {
		t.Fatalf("MuxAndVerifyTask: %v", err)
	}
	if media.ID == 0 || media.VideoCodec != "h264" || !media.HasAudio {
		t.Fatalf("unexpected media row: %+v", media)
	}

	// Corrupt output (below min size) must fail quality gate.
	corrupt := filepath.Join(tmp, "corrupt.mp4")
	if err := os.WriteFile(corrupt, []byte("tiny"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MuxAndVerifyTask(context.Background(), db, task, MuxVerifyOptions{
		PrebuiltMP4Path: corrupt,
		OutputMP4Path:   filepath.Join(tmp, "out", "bad.mp4"),
		FFprobeExe:      ffprobeScript,
		MinFileBytes:    64 * 1024,
	}); err == nil {
		t.Fatalf("expected quality gate failure for tiny/corrupt mp4")
	}
}
