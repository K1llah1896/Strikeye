package mapgeom

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func TestLoadQuad(t *testing.T) {
	data := writeQuad(t)
	path := filepath.Join(t.TempDir(), "de_test.tri")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	bvh, sum, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := sha256.Sum256(data)
	if sum != hex.EncodeToString(want[:]) {
		t.Fatalf("sha mismatch: %s", sum)
	}
	if len(bvh.Vertices) != 4 || len(bvh.Indices) != 6 || len(bvh.Nodes) != 1 {
		t.Fatalf("unexpected geometry: %d verts, %d indices, %d nodes", len(bvh.Vertices), len(bvh.Indices), len(bvh.Nodes))
	}
	if bvh.Nodes[0].Count != 2 || bvh.Nodes[0].LeftOrFirst != 0 {
		t.Fatalf("unexpected root node: %+v", bvh.Nodes[0])
	}
}

func TestLoadRejectsBadMagic(t *testing.T) {
	var buf bytes.Buffer
	gz := newGzipWriter(&buf)
	if _, err := gz.Write(make([]byte, 64)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected error for bad magic")
	}
}

func TestReadRejectsTruncatedSection(t *testing.T) {
	data := writeQuad(t)
	raw, err := gunzip(data)
	if err != nil {
		t.Fatal(err)
	}
	truncated := raw[:len(raw)-8]
	if _, err := Read(bytes.NewReader(gzipBytes(t, truncated))); err == nil {
		t.Fatal("expected error for truncated node section")
	}
}

// TestReadAcceptsEveryContainer pins the packed-bundle contract: the loader takes
// the published gzip files, the raw payload, and the zstd/xz frames the release
// sidecar stores, all of which decode to the same mesh.
func TestReadAcceptsEveryContainer(t *testing.T) {
	gzipped := writeQuad(t)
	raw, err := gunzip(gzipped)
	if err != nil {
		t.Fatal(err)
	}
	var zstdBuf bytes.Buffer
	zw, err := zstd.NewWriter(&zstdBuf, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	var xzBuf bytes.Buffer
	xw, err := xz.WriterConfig{}.NewWriter(&xzBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := xw.Close(); err != nil {
		t.Fatal(err)
	}

	cases := map[string][]byte{
		"gzip": gzipped,
		"raw":  raw,
		"zstd": zstdBuf.Bytes(),
		"xz":   xzBuf.Bytes(),
	}
	for name, data := range cases {
		bvh, err := Read(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: Read: %v", name, err)
		}
		if len(bvh.Vertices) != 4 || len(bvh.Nodes) != 1 {
			t.Fatalf("%s: unexpected geometry %d/%d", name, len(bvh.Vertices), len(bvh.Nodes))
		}
	}
}

// TestCoreMaps pins the maps the release bundle carries: they must be the
// competitive pool, and the lookup must tolerate the name shapes the manifest
// and the demo headers produce.
func TestCoreMaps(t *testing.T) {
	if len(CoreMaps) != 11 {
		t.Fatalf("CoreMaps has %d entries, want 11", len(CoreMaps))
	}
	for _, name := range CoreMaps {
		if !strings.HasPrefix(name, "de_") && !strings.HasPrefix(name, "cs_") {
			t.Fatalf("suspicious map name %q", name)
		}
	}
	for _, name := range []string{"de_dust2", "DE_MIRAGE", "workshop/123/de_inferno.tri"} {
		if !IsCoreMap(name) {
			t.Fatalf("IsCoreMap(%q) = false", name)
		}
	}
	for _, name := range []string{"cs_italy", "cs_office", "de_poseidon", ""} {
		if IsCoreMap(name) {
			t.Fatalf("IsCoreMap(%q) = true", name)
		}
	}
}
