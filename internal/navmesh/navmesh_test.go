package navmesh

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"strikeye/internal/mapgeom"
)

type navBuilder struct{ buf []byte }

func (b *navBuilder) u8(v uint8)   { b.buf = append(b.buf, v) }
func (b *navBuilder) u32(v uint32) { b.buf = binary.LittleEndian.AppendUint32(b.buf, v) }
func (b *navBuilder) i64(v int64)  { b.buf = binary.LittleEndian.AppendUint64(b.buf, uint64(v)) }
func (b *navBuilder) i32(v int32)  { b.u32(uint32(v)) }
func (b *navBuilder) vec3(x, y, z float32) {
	b.u32(math.Float32bits(x))
	b.u32(math.Float32bits(y))
	b.u32(math.Float32bits(z))
}
func (b *navBuilder) zeros(n int) {
	for i := 0; i < n; i++ {
		b.u8(0)
	}
}

// twoSquareNav writes a version-31 nav holding two adjacent squares:
// area 1 covers x in [0,100], area 2 covers x in [100,200]; both at y in [0,100].
func twoSquareNav() []byte {
	b := &navBuilder{}
	b.u32(FileMagic)
	b.u32(31)
	b.u32(1)
	b.u32(1) // analyzed

	// Shared corner pool.
	b.u32(6)
	b.vec3(0, 0, 0)
	b.vec3(100, 0, 0)
	b.vec3(100, 100, 0)
	b.vec3(0, 100, 0)
	b.vec3(200, 0, 0)
	b.vec3(200, 100, 0)
	// Polygons.
	b.u32(2)
	b.u8(4)
	b.u32(0)
	b.u32(1)
	b.u32(2)
	b.u32(3)
	b.u8(4)
	b.u32(1)
	b.u32(4)
	b.u32(5)
	b.u32(2)

	b.u32(2) // area count

	writeArea := func(id, poly uint32, neighbor uint32) {
		b.u32(id)
		b.i64(0)
		b.u8(0)
		b.u32(poly)
		b.u32(0)
		for i := 0; i < 4; i++ {
			if i == 1 {
				b.u32(1)
				b.u32(neighbor)
				b.u32(0)
			} else {
				b.u32(0)
			}
		}
		b.zeros(5)
		b.u32(0)
		b.u32(0)
	}
	writeArea(1, 0, 2)
	writeArea(2, 1, 1)
	return b.buf
}

func TestParseTwoSquares(t *testing.T) {
	m, err := Parse(twoSquareNav())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Version != 31 || !m.Analyzed {
		t.Fatalf("version=%d analyzed=%v", m.Version, m.Analyzed)
	}
	if m.AreaCount() != 2 {
		t.Fatalf("areas=%d", m.AreaCount())
	}
	a, ok := m.Area(1)
	if !ok || len(a.Corners) != 4 || len(a.Neighbors) != 1 || a.Neighbors[0] != 2 {
		t.Fatalf("area 1 = %+v", a)
	}
	got, ok := m.FindArea(mapgeom.Vec3{40, 40, 0})
	if !ok || got != 1 {
		t.Fatalf("FindArea(40,40) = %d, %v", got, ok)
	}
	got, ok = m.FindArea(mapgeom.Vec3{160, 40, 0})
	if !ok || got != 2 {
		t.Fatalf("FindArea(160,40) = %d, %v", got, ok)
	}
	if _, ok := m.FindArea(mapgeom.Vec3{500, 500, 0}); ok {
		t.Fatal("point outside the mesh resolved to an area")
	}

	path, err := m.FindPath(1, 2)
	if err != nil {
		t.Fatalf("FindPath: %v", err)
	}
	if len(path.Areas) != 2 || path.Areas[0] != 1 || path.Areas[1] != 2 {
		t.Fatalf("path = %v", path.Areas)
	}
	if math.Abs(path.Distance-100) > 0.01 {
		t.Fatalf("distance = %f", path.Distance)
	}
	if _, err := m.FindPath(1, 99); !errors.Is(err, ErrInvalidNav) {
		t.Fatalf("unknown area error = %v", err)
	}
}

func TestFindAreaPrefersClosestFloor(t *testing.T) {
	b := &navBuilder{}
	b.u32(FileMagic)
	b.u32(31)
	b.u32(1)
	b.u32(1)
	b.u32(8)
	for _, p := range [][3]float32{
		{0, 0, 0}, {100, 0, 0}, {100, 100, 0}, {0, 100, 0},
		{0, 0, 500}, {100, 0, 500}, {100, 100, 500}, {0, 100, 500},
	} {
		b.vec3(p[0], p[1], p[2])
	}
	b.u32(2)
	b.u8(4)
	b.u32(0)
	b.u32(1)
	b.u32(2)
	b.u32(3)
	b.u8(4)
	b.u32(4)
	b.u32(5)
	b.u32(6)
	b.u32(7)
	b.u32(2)
	for i := 0; i < 2; i++ {
		b.u32(uint32(10 + i))
		b.i64(0)
		b.u8(0)
		b.u32(uint32(i))
		b.u32(0)
		b.zeros(4 * 4)
		b.zeros(5)
		b.u32(0)
		b.u32(0)
	}
	m, err := Parse(b.buf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if id, _ := m.FindArea(mapgeom.Vec3{50, 50, 10}); id != 10 {
		t.Fatalf("ground floor = %d, want 10", id)
	}
	if id, _ := m.FindArea(mapgeom.Vec3{50, 50, 490}); id != 11 {
		t.Fatalf("upper floor = %d, want 11", id)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse([]byte("nope")); !errors.Is(err, ErrInvalidNav) {
		t.Fatalf("short input error = %v", err)
	}
	b := &navBuilder{}
	b.u32(FileMagic)
	b.u32(99)
	b.u32(0)
	b.u32(0)
	if _, err := Parse(b.buf); !errors.Is(err, ErrInvalidNav) {
		t.Fatalf("version error = %v", err)
	}
}

// v36Nav wraps the two-square mesh in the version-36 container: a placeholder
// KV3 document before the polygon table and another before the areas.
func v36Nav() []byte {
	b := &navBuilder{}
	b.u32(FileMagic)
	b.u32(36)
	b.u32(1)
	b.u32(1)
	writePlaceholderKV3(b)
	b.u32(6)
	b.vec3(0, 0, 0)
	b.vec3(100, 0, 0)
	b.vec3(100, 100, 0)
	b.vec3(0, 100, 0)
	b.vec3(200, 0, 0)
	b.vec3(200, 100, 0)
	b.u32(2)
	for _, poly := range [][]uint32{{0, 1, 2, 3}, {1, 4, 5, 2}} {
		b.u8(4)
		for _, idx := range poly {
			b.u32(idx)
		}
		b.u32(0) // per-polygon unknown field (version >= 35)
	}
	b.u32(0) // version >= 32 unknown
	b.u32(0) // marker count
	writePlaceholderKV3(b)
	b.u32(2)
	for i := uint32(0); i < 2; i++ {
		b.u32(i + 1)
		b.i64(0)
		b.u8(0)
		b.u32(i)
		b.u32(0)
		for e := 0; e < 4; e++ {
			if e == 1 {
				b.u32(1)
				b.u32(2 - i)
				b.u32(0)
			} else {
				b.u32(0)
			}
		}
		b.zeros(5)
		b.u32(0)
		b.u32(0)
	}
	return b.buf
}

func writePlaceholderKV3(b *navBuilder) {
	for len(b.buf)%8 != 0 {
		b.u8(0)
	}
	b.u32(0x4B56_3300 | 5)
	b.zeros(16) // format GUID
	b.u32(0)    // method: uncompressed
	b.zeros(4)  // dictionary id + frame size
	b.zeros(16) // countBytes1/4/8 + countTypes
	b.zeros(4)  // countObjects + countArrays
	b.i32(0)    // size_unc_total
	b.i32(0)    // size_cmp_total
	b.i32(0)    // blocks
	b.i32(0)    // blobs
	b.u32(0)    // countBytes2
	b.i32(0)    // size_block_compressed
	b.i32(0)    // unc1
	b.i32(0)    // cmp1
	b.i32(0)    // unc2
	b.i32(0)    // cmp2
	b.zeros(32) // trailing buffer-2 counts
}

func TestParseVersion36SkipsKV3(t *testing.T) {
	m, err := Parse(v36Nav())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Version != 36 || m.AreaCount() != 2 {
		t.Fatalf("version=%d areas=%d", m.Version, m.AreaCount())
	}
	path, err := m.FindPath(1, 2)
	if err != nil || math.Abs(path.Distance-100) > 0.01 {
		t.Fatalf("path=%+v err=%v", path, err)
	}
}
