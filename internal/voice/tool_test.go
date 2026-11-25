package voice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeTool creates a stub decoder that writes WAVs into its second argument and
// exits with the requested code.
func writeTool(t *testing.T, code int, tracks int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub decoder uses sh")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "decoder.sh")
	script := "#!/bin/sh\nset -e\nout=\"$2\"\n"
	for i := 0; i < tracks; i++ {
		script += "printf 'RIFF' > \"$out/speaker" + string(rune('a'+i)) + ".wav\"\n"
	}
	if code != 0 {
		script = "#!/bin/sh\necho 'no voice data in demo' >&2\nexit " + itoa(code) + "\n"
	}
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return tool
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestDecodeWithToolProducesTracks(t *testing.T) {
	demo := filepath.Join(t.TempDir(), "m.dem")
	if err := os.WriteFile(demo, []byte("PBDEMS2"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	// A pre-existing WAV must not be reported as produced by this run.
	if err := os.WriteFile(filepath.Join(out, "old.wav"), []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := writeTool(t, 0, 2)

	tracks, err := DecodeWithTool(context.Background(), tool, demo, out)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("tracks = %v", tracks)
	}
	for _, p := range tracks {
		if !strings.HasSuffix(p, ".wav") {
			t.Fatalf("not a wav: %s", p)
		}
	}
}

func TestDecodeWithToolExitCodes(t *testing.T) {
	demo := filepath.Join(t.TempDir(), "m.dem")
	if err := os.WriteFile(demo, []byte("PBDEMS2"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := writeTool(t, 12, 0)

	_, err := DecodeWithTool(context.Background(), tool, demo, t.TempDir())
	if err == nil {
		t.Fatal("expected exit error")
	}
	var exitErr *ToolExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err type = %T", err)
	}
	if exitErr.Code != 12 {
		t.Fatalf("code = %d", exitErr.Code)
	}
	if !strings.Contains(err.Error(), "no voice data") {
		t.Fatalf("missing hint: %v", err)
	}
	if !strings.Contains(err.Error(), "no voice data in demo") {
		t.Fatalf("missing tool output: %v", err)
	}
}

func TestDecodeWithToolValidation(t *testing.T) {
	if _, err := DecodeWithTool(context.Background(), "", "x.dem", t.TempDir()); err == nil {
		t.Fatal("expected missing tool error")
	}
	if _, err := DecodeWithTool(context.Background(), "/bin/true", filepath.Join(t.TempDir(), "nope.dem"), t.TempDir()); err == nil {
		t.Fatal("expected missing demo error")
	}
}
