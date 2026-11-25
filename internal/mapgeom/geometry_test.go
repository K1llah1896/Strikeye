package mapgeom

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
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
