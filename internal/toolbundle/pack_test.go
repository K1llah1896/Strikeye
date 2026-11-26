package toolbundle

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"

	"strikeye/internal/updater"
)

// upstreamFFmpegZip mimics the layout of the official FFmpeg builds: a top
// directory with bin/, doc/ and a third static binary the app never runs.
func upstreamFFmpegZip(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, body []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	add("ffmpeg-9.1-essentials_build/bin/ffmpeg.exe", bytes.Repeat([]byte("ffmpeg"), 4096))
	add("ffmpeg-9.1-essentials_build/bin/ffprobe.exe", bytes.Repeat([]byte("ffprobe"), 4096))
	add("ffmpeg-9.1-essentials_build/bin/ffplay.exe", bytes.Repeat([]byte("ffplay"), 4096))
	add("ffmpeg-9.1-essentials_build/doc/ffmpeg.html", bytes.Repeat([]byte("<html/>"), 4096))
	add("ffmpeg-9.1-essentials_build/LICENSE.txt", []byte("GPLv3"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackFFmpegKeepsOnlyTheTwoBinaries(t *testing.T) {
	dir := t.TempDir()
	upstream := filepath.Join(dir, "upstream.zip")
	upstreamFFmpegZip(t, upstream)
	dest := filepath.Join(dir, FFmpegArchiveName)

	size, err := PackFFmpeg(upstream, dest)
	if err != nil {
		t.Fatalf("PackFFmpeg: %v", err)
	}
	if size <= 0 {
		t.Fatalf("reported size %d", size)
	}
	if !LooksLikeTarZst(dest) {
		t.Fatal("sidecar is not a zstd frame")
	}
	names := tarZstNames(t, dest)
	want := map[string]bool{"bin/ffmpeg.exe": true, "bin/ffprobe.exe": true, "LICENSE.txt": true}
	if len(names) != len(want) {
		t.Fatalf("entries = %v, want %d", names, len(want))
	}
	for _, name := range names {
		if !want[name] {
			t.Fatalf("unexpected entry %q (ffplay/doc must be dropped)", name)
		}
	}
	// The verification step inside PackFFmpeg already extracted it; unpack again
	// and check the payloads survived byte for byte.
	out := t.TempDir()
	if err := updater.ExtractToolArchive(dest, out); err != nil {
		t.Fatalf("ExtractToolArchive: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(out, "bin", "ffprobe.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, bytes.Repeat([]byte("ffprobe"), 4096)) {
		t.Fatal("ffprobe payload changed")
	}
}

func TestPackFFmpegRejectsIncompleteUpstream(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("ffmpeg-x/bin/ffmpeg.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("only-ffmpeg")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	upstream := filepath.Join(dir, "upstream.zip")
	if err := os.WriteFile(upstream, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PackFFmpeg(upstream, filepath.Join(dir, FFmpegArchiveName)); err == nil {
		t.Fatal("expected an error when ffprobe.exe is missing")
	}
	if _, err := os.Stat(filepath.Join(dir, FFmpegArchiveName)); !os.IsNotExist(err) {
		t.Fatal("no sidecar should be left behind after a failed pack")
	}
}

func TestProvisionExtractsTarZstSidecar(t *testing.T) {
	dataRoot := t.TempDir()
	exeDir := t.TempDir()
	writeTarZst(t, filepath.Join(exeDir, FFmpegArchiveName), ffmpegEntries())
	rep, err := Provision(dataRoot, exeDir)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if len(rep.Installed) != 1 || rep.Installed[0] != updater.ToolFFmpeg {
		t.Fatalf("installed = %v", rep.Installed)
	}
	if _, err := os.Stat(filepath.Join(dataRoot, "tools", "ffmpeg", "ffmpeg-x", "bin", "ffmpeg.exe")); err != nil {
		t.Fatalf("ffmpeg.exe not staged: %v", err)
	}
}

func TestFindPrefersCurrentSidecarName(t *testing.T) {
	dir := t.TempDir()
	writeArchive(t, filepath.Join(dir, legacyFFmpegArchiveName), ffmpegEntries())
	writeTarZst(t, filepath.Join(dir, FFmpegArchiveName), ffmpegEntries())
	got, ok := Find(t.TempDir(), dir, updater.ToolFFmpeg)
	if !ok {
		t.Fatal("Find reported no ffmpeg sidecar")
	}
	if filepath.Base(got) != FFmpegArchiveName {
		t.Fatalf("Find picked %q, want the tar.zst sidecar", got)
	}
}

func tarZstNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec, err := zstd.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()
	var names []string
	tr := tar.NewReader(dec)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			names = append(names, hdr.Name)
		}
	}
}
