package watcher

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func demoPayload(size int) []byte {
	buf := make([]byte, size)
	copy(buf, DemoHeaderMagic)
	for i := 8; i < len(buf); i++ {
		buf[i] = byte(i % 251)
	}
	return buf
}

func writeZip(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, payload := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %q: %v", name, err)
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("zip write %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}
}

func TestArchiveKindDetection(t *testing.T) {
	yes := map[string]string{
		"a.dem.zip": "zip",
		"a.zip":     "zip",
		"a.dem.gz":  "gzip",
		"a.gz":      "gzip",
		"a.dem.bz2": "bzip2",
		"a.dem.zst": "zstd",
		"a.DEM.ZST": "zstd",
	}
	for name, kind := range yes {
		got, ok := ArchiveKind(name)
		if !ok || got != kind {
			t.Errorf("ArchiveKind(%q) = %q,%v want %q,true", name, got, ok, kind)
		}
	}
	for _, name := range []string{"a.dem", "a.rar", "a.7z", "a.tar"} {
		if _, ok := ArchiveKind(name); ok {
			t.Errorf("ArchiveKind(%q) unexpectedly supported", name)
		}
	}
}

func TestExtractZipDemosFlattensAndNeutralizesTraversal(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "faceit_match.zip")
	writeZip(t, archivePath, map[string][]byte{
		"nested/match.dem": demoPayload(2048),
		"../../evil.dem":   demoPayload(1500),
		"notes.txt":        []byte("ignore"),
		"second/inner.dem": demoPayload(1200),
	})

	out, err := ExtractDemoArchives(context.Background(), archivePath, filepath.Join(tmp, "imports"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 extracted demos, got %d (%v)", len(out), out)
	}
	seen := map[string]bool{}
	for _, p := range out {
		if filepath.Dir(p) != filepath.Join(tmp, "imports", archiveExtractDirName(archivePath)) {
			t.Fatalf("demo extracted outside import dir: %s", p)
		}
		seen[filepath.Base(p)] = true
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing extracted file %s: %v", p, err)
		}
	}
	if !seen["match.dem"] || !seen["evil.dem"] || !seen["inner.dem"] {
		t.Fatalf("unexpected extracted names: %v", seen)
	}
	if _, err := os.Stat(filepath.Join(tmp, "evil.dem")); !os.IsNotExist(err) {
		t.Fatalf("traversal entry escaped import dir")
	}

	// Re-extraction is idempotent: same directory, same files.
	out2, err := ExtractDemoArchives(context.Background(), archivePath, filepath.Join(tmp, "imports"))
	if err != nil || len(out2) != 3 {
		t.Fatalf("second extract: %v (%d)", err, len(out2))
	}
}

func TestExtractZipRejectsTooManyEntriesAndEmptyArchives(t *testing.T) {
	tmp := t.TempDir()

	many := make(map[string][]byte, MaxArchiveEntries+1)
	for i := 0; i < MaxArchiveEntries+1; i++ {
		many[fmt.Sprintf("entry_%02d.dem", i)] = demoPayload(16)
	}
	big := filepath.Join(tmp, "many.zip")
	writeZip(t, big, many)
	if _, err := ExtractDemoArchives(context.Background(), big, filepath.Join(tmp, "imports")); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("expected ErrArchiveLimit, got %v", err)
	}

	empty := filepath.Join(tmp, "empty.zip")
	writeZip(t, empty, map[string][]byte{"readme.txt": []byte("no demo")})
	if _, err := ExtractDemoArchives(context.Background(), empty, filepath.Join(tmp, "imports")); !errors.Is(err, ErrArchiveEmpty) {
		t.Fatalf("expected ErrArchiveEmpty, got %v", err)
	}
}

func TestExtractSingleStreamArchives(t *testing.T) {
	tmp := t.TempDir()
	payload := demoPayload(4096)

	gzPath := filepath.Join(tmp, "match.dem.gz")
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gzPath, gzBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := ExtractDemoArchives(context.Background(), gzPath, filepath.Join(tmp, "imports"))
	if err != nil || len(out) != 1 || filepath.Base(out[0]) != "match.dem" {
		t.Fatalf("gzip extract: %v %v", out, err)
	}
	if data, err := os.ReadFile(out[0]); err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("gzip payload mismatch: %v", err)
	}

	zstPath := filepath.Join(tmp, "scrim.dem.zst")
	var zstBuf bytes.Buffer
	zw, err := zstd.NewWriter(&zstBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zstPath, zstBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = ExtractDemoArchives(context.Background(), zstPath, filepath.Join(tmp, "imports"))
	if err != nil || len(out) != 1 || filepath.Base(out[0]) != "scrim.dem" {
		t.Fatalf("zstd extract: %v %v", out, err)
	}
	if data, err := os.ReadFile(out[0]); err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("zstd payload mismatch: %v", err)
	}

	// gzip without a demo extension still yields a .dem file.
	plainGz := filepath.Join(tmp, "renamed.gz")
	if err := os.WriteFile(plainGz, gzBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = ExtractDemoArchives(context.Background(), plainGz, filepath.Join(tmp, "imports"))
	if err != nil || len(out) != 1 || filepath.Ext(out[0]) != ".dem" {
		t.Fatalf("extensionless gzip extract: %v %v", out, err)
	}
}

func TestSanitizeEntryBase(t *testing.T) {
	cases := map[string]string{
		"../../evil.dem": "evil.dem",
		`..\..\win.dem`:  "win.dem",
		"dir/a:b?.dem":   "a_b_.dem",
		"":               "",
		".":              "",
	}
	for in, want := range cases {
		if got := sanitizeEntryBase(in); got != want {
			t.Errorf("sanitizeEntryBase(%q) = %q want %q", in, got, want)
		}
	}
}
