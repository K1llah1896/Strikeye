// Package mapgeom loads CS2 collision geometry (.tri) and answers line-of-sight
// queries. The file format and BVH layout are ported from akiver/cs-demo-analyzer
// (MIT, https://github.com/akiver/cs-demo-analyzer) and its cs2-map-geometry
// dataset (https://gitlab.com/akiver/cs2-map-geometry).
package mapgeom

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Extension is the geometry file extension.
const Extension = ".tri"

// CoreMaps are the competitive-pool maps whose geometry ships inside the release
// package. Casual and arms-race maps stay available through `maps-fetch` and the
// settings card, so a small download tops up an install instead of being a
// prerequisite for every map.
var CoreMaps = []string{
	"de_ancient",
	"de_anubis",
	"de_cache",
	"de_cbble",
	"de_dust2",
	"de_inferno",
	"de_mirage",
	"de_nuke",
	"de_overpass",
	"de_train",
	"de_vertigo",
}

// IsCoreMap reports whether name belongs to CoreMaps (case-insensitive, with or
// without the .tri extension and the workshop prefix).
func IsCoreMap(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(filepath.Base(name), Extension)
	for _, core := range CoreMaps {
		if name == core {
			return true
		}
	}
	return false
}

const (
	// FormatVersion is bumped when the layout changes; readers reject others.
	FormatVersion = 1
	headerSize    = 24
	vertexSize    = 12
	indexSize     = 4
	nodeSize      = 32

	maxElements          = 1 << 24
	chunkElements        = 4096
	preallocatedElements = 1 << 20
)

// ErrInvalidGeometry reports a malformed geometry file.
var ErrInvalidGeometry = errors.New("invalid map geometry file")

var magic = [8]byte{'C', 'S', 'D', 'A', 'G', 'E', 'O', 0}

// Vec3 is a compact 3D vector.
type Vec3 [3]float32

// Node is one BVH node; leaves store Count>0 with LeftOrFirst as first triangle,
// inner nodes store the left child index with the right child immediately after.
type Node struct {
	Min         Vec3
	Max         Vec3
	LeftOrFirst uint32
	Count       uint32
}

// Container magics accepted by decompressGeometry.
var (
	gzipMagic = []byte{0x1f, 0x8b}
	zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}
	xzMagic   = []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}
)

// decompressGeometry unwraps the container around the raw CSDAGEO payload.
// Plain payloads are accepted as well, which keeps any older cached file usable.
func decompressGeometry(r io.Reader) (io.Reader, func(), error) {
	br := bufio.NewReaderSize(r, 1<<20)
	head, err := br.Peek(6)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidGeometry, err)
	}
	switch {
	case bytes.HasPrefix(head, gzipMagic):
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrInvalidGeometry, err)
		}
		return gz, func() { _ = gz.Close() }, nil
	case bytes.HasPrefix(head, zstdMagic):
		dec, err := zstd.NewReader(br, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(1<<30))
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrInvalidGeometry, err)
		}
		return dec, dec.Close, nil
	case bytes.HasPrefix(head, xzMagic):
		dec, err := xz.NewReader(br)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrInvalidGeometry, err)
		}
		return dec, nil, nil
	}
	return br, nil, nil
}

// BVH is a bounding volume hierarchy over map triangles.
type BVH struct {
	Vertices []Vec3
	Indices  []uint32
	Nodes    []Node
}

// Load reads a geometry file and returns it with the SHA-256 of its bytes.
func Load(path string) (*BVH, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	bvh, err := Read(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	return bvh, hex.EncodeToString(sum[:]), nil
}

// Read decodes a geometry stream. The dataset publishes gzip-compressed files;
// bundled sidecars re-compress the payload with zstd (or xz) to save space, so
// the container is detected from its magic bytes.
func Read(r io.Reader) (*BVH, error) {
	payload, closer, err := decompressGeometry(r)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer()
	}
	br := bufio.NewReaderSize(payload, 1<<20)

	header := make([]byte, headerSize)
	if _, err := io.ReadFull(br, header); err != nil {
		return nil, fmt.Errorf("%w: read header: %w", ErrInvalidGeometry, err)
	}
	if [8]byte(header[:8]) != magic {
		return nil, fmt.Errorf("%w: bad magic", ErrInvalidGeometry)
	}
	if v := binary.LittleEndian.Uint32(header[8:]); v != FormatVersion {
		return nil, fmt.Errorf("%w: unsupported version %d", ErrInvalidGeometry, v)
	}
	vertexCount := binary.LittleEndian.Uint32(header[12:])
	indexCount := binary.LittleEndian.Uint32(header[16:])
	nodeCount := binary.LittleEndian.Uint32(header[20:])
	if vertexCount > maxElements || indexCount > maxElements || nodeCount > maxElements || indexCount%3 != 0 || nodeCount == 0 {
		return nil, fmt.Errorf("%w: invalid counts", ErrInvalidGeometry)
	}

	vertices, err := readSection(br, vertexCount, vertexSize, func(b []byte, _ int) (Vec3, error) {
		return Vec3{
			math.Float32frombits(binary.LittleEndian.Uint32(b)),
			math.Float32frombits(binary.LittleEndian.Uint32(b[4:])),
			math.Float32frombits(binary.LittleEndian.Uint32(b[8:])),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	indices, err := readSection(br, indexCount, indexSize, func(b []byte, _ int) (uint32, error) {
		idx := binary.LittleEndian.Uint32(b)
		if idx >= vertexCount {
			return 0, fmt.Errorf("%w: vertex index out of range", ErrInvalidGeometry)
		}
		return idx, nil
	})
	if err != nil {
		return nil, err
	}
	triangleCount := indexCount / 3
	nodes, err := readSection(br, nodeCount, nodeSize, func(b []byte, i int) (Node, error) {
		n := Node{
			Min:         Vec3{math.Float32frombits(binary.LittleEndian.Uint32(b)), math.Float32frombits(binary.LittleEndian.Uint32(b[4:])), math.Float32frombits(binary.LittleEndian.Uint32(b[8:]))},
			Max:         Vec3{math.Float32frombits(binary.LittleEndian.Uint32(b[12:])), math.Float32frombits(binary.LittleEndian.Uint32(b[16:])), math.Float32frombits(binary.LittleEndian.Uint32(b[20:]))},
			LeftOrFirst: binary.LittleEndian.Uint32(b[24:]),
			Count:       binary.LittleEndian.Uint32(b[28:]),
		}
		if n.Count > 0 {
			if uint64(n.LeftOrFirst)+uint64(n.Count) > uint64(triangleCount) {
				return Node{}, fmt.Errorf("%w: leaf out of range", ErrInvalidGeometry)
			}
		} else if uint64(n.LeftOrFirst)+1 >= uint64(nodeCount) || n.LeftOrFirst <= uint32(i) {
			return Node{}, fmt.Errorf("%w: node child out of range", ErrInvalidGeometry)
		}
		return n, nil
	})
	if err != nil {
		return nil, err
	}
	return &BVH{Vertices: vertices, Indices: indices, Nodes: nodes}, nil
}

func readSection[T any](r io.Reader, count uint32, size int, decode func([]byte, int) (T, error)) ([]T, error) {
	total := int(count)
	out := make([]T, 0, min(total, preallocatedElements))
	chunk := make([]byte, min(total, chunkElements)*size)
	for len(out) < total {
		data := chunk[:min(total-len(out), chunkElements)*size]
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidGeometry, err)
		}
		for off := 0; off < len(data); off += size {
			v, err := decode(data[off:off+size], len(out))
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
	}
	return out, nil
}
