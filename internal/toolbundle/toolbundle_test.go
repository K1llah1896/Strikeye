package toolbundle

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"strikeye/internal/updater"
)

// writeArchive builds a zip laid out like the official tool archives.
func writeArchive(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ffmpegEntries() map[string][]byte {
	return map[string][]byte{
		"ffmpeg-x/bin/ffmpeg.exe":  []byte("ffmpeg-binary"),
		"ffmpeg-x/bin/ffprobe.exe": []byte("ffprobe-binary"),
		"ffmpeg-x/LICENSE.txt":     []byte("GPL"),
	}
}

func hlaeEntries() map[string][]byte {
	return map[string][]byte{
		"hlae/HLAE.exe":                []byte("hlae-binary"),
		"hlae/AfxHookSource2.dll":      []byte("hook-dll"),
		"hlae/mirv/MirvEngineStub.dll": []byte("mirv"),
	}
}

func TestArchiveNamesAreFixedToWindows(t *testing.T) {
	if got := ArchiveName(updater.ToolFFmpeg); got != "ffmpeg-windows-amd64.zip" {
		t.Fatalf("ffmpeg name = %q", got)
	}
	if got := ArchiveName(updater.ToolHLAE); got != "hlae-windows-amd64.zip" {
		t.Fatalf("hlae name = %q", got)
	}
	if got := ArchiveName("unknown"); got != "" {
		t.Fatalf("unknown tool should yield empty name, got %q", got)
	}
}

func TestProvisionExtractsBundledTools(t *testing.T) {
	dataRoot := t.TempDir()
	exeDir := t.TempDir()
	writeArchive(t, filepath.Join(exeDir, FFmpegArchiveName), ffmpegEntries())
	writeArchive(t, filepath.Join(exeDir, HLAAArchiveName), hlaeEntries())

	rep, err := Provision(dataRoot, exeDir)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if len(rep.Installed) != 2 || !rep.Ready() {
		t.Fatalf("expected both tools installed, got %+v", rep)
	}
	for _, rel := range []string{
		"tools/ffmpeg/ffmpeg-x/bin/ffmpeg.exe",
		"tools/ffmpeg/ffmpeg-x/bin/ffprobe.exe",
		"tools/hlae/hlae/HLAE.exe",
		"tools/hlae/hlae/AfxHookSource2.dll",
	} {
		if _, err := os.Stat(filepath.Join(dataRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s missing: %v", rel, err)
		}
	}
	if rep.Bundled[updater.ToolFFmpeg] == "" || rep.Bundled[updater.ToolHLAE] == "" {
		t.Fatalf("bundled paths not reported: %+v", rep.Bundled)
	}
}

func TestProvisionIsIdempotent(t *testing.T) {
	dataRoot := t.TempDir()
	exeDir := t.TempDir()
	writeArchive(t, filepath.Join(exeDir, FFmpegArchiveName), ffmpegEntries())
	writeArchive(t, filepath.Join(exeDir, HLAAArchiveName), hlaeEntries())
	if _, err := Provision(dataRoot, exeDir); err != nil {
		t.Fatalf("first Provision: %v", err)
	}
	// Remove the archives: a second run must not need them.
	if err := os.Remove(filepath.Join(exeDir, FFmpegArchiveName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(exeDir, HLAAArchiveName)); err != nil {
		t.Fatal(err)
	}
	rep, err := Provision(dataRoot, exeDir)
	if err != nil {
		t.Fatalf("second Provision: %v", err)
	}
	if len(rep.Installed) != 0 || len(rep.Present) != 2 || !rep.Ready() {
		t.Fatalf("expected both tools already present, got %+v", rep)
	}
}

func TestProvisionWithoutBundlesReportsMissing(t *testing.T) {
	rep, err := Provision(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if rep.Ready() || len(rep.Missing) != 2 {
		t.Fatalf("expected both tools missing, got %+v", rep)
	}
	if rep.Summary() == "" {
		t.Fatal("summary must not be empty")
	}
}

func TestFindSearchesDataRootAndAssets(t *testing.T) {
	dataRoot := t.TempDir()
	assets := filepath.Join(dataRoot, "assets")
	writeArchive(t, filepath.Join(assets, FFmpegArchiveName), ffmpegEntries())
	got, ok := Find(dataRoot, t.TempDir(), updater.ToolFFmpeg)
	if !ok {
		t.Fatal("expected to find ffmpeg archive under DataRoot/assets")
	}
	if got != filepath.Join(assets, FFmpegArchiveName) {
		t.Fatalf("found %q", got)
	}
	if _, ok := Find(dataRoot, "", updater.ToolHLAE); ok {
		t.Fatal("hlae archive is not present")
	}
}

func TestProvisionRejectsEmptyDataRoot(t *testing.T) {
	if _, err := Provision("  ", t.TempDir()); err == nil {
		t.Fatal("expected an error for an empty DataRoot")
	}
}

func TestProvisionFailsOnBrokenArchive(t *testing.T) {
	dataRoot := t.TempDir()
	exeDir := t.TempDir()
	// Valid zip, but without the required binaries.
	writeArchive(t, filepath.Join(exeDir, FFmpegArchiveName), map[string][]byte{"readme.txt": []byte("nothing here")})
	if _, err := Provision(dataRoot, exeDir); err == nil {
		t.Fatal("expected an error when the archive lacks ffmpeg binaries")
	}
}
