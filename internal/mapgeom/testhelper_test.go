package mapgeom

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"testing"
)

func writeQuad(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	var raw bytes.Buffer
	raw.WriteString("CSDAGEO\x00")
	for _, v := range []uint32{FormatVersion, 4, 6, 1} {
		if err := binary.Write(&raw, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	verts := []float32{0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0}
	for _, f := range verts {
		_ = binary.Write(&raw, binary.LittleEndian, f)
	}
	for _, i := range []uint32{0, 1, 2, 0, 2, 3} {
		_ = binary.Write(&raw, binary.LittleEndian, i)
	}
	for _, f := range []float32{0, 0, 0, 1, 1, 0} {
		_ = binary.Write(&raw, binary.LittleEndian, f)
	}
	_ = binary.Write(&raw, binary.LittleEndian, uint32(0))
	_ = binary.Write(&raw, binary.LittleEndian, uint32(2))
	if _, err := gz.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newGzipWriter(w *bytes.Buffer) *gzip.Writer { return gzip.NewWriter(w) }

func gzipBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gunzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
