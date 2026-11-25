// Package navmesh parses CS2 navigation meshes (.nav) and answers area lookups
// and shortest-path queries.
//
// The file format (versions 30-36) follows the reverse-engineered layout
// documented by ValveResourceFormat and implemented by pnxenopoulos/awpy
// (MIT, crates/awpy/src/nav.rs). CS2 currently ships version 36.
package navmesh

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"

	"strikeye/internal/mapgeom"
)

// FileMagic is the little-endian magic at the start of every .nav file.
const FileMagic uint32 = 0xFEED_FACE

// Supported version range.
const (
	MinVersion = 30
	MaxVersion = 36
)

// ErrInvalidNav reports a malformed or unsupported navigation mesh.
var ErrInvalidNav = errors.New("invalid nav mesh")

// kv3Magic is the "3VK" signature family used by embedded KV3 documents.
const kv3MagicMask uint32 = 0x4B56_3300

// Area is one walkable convex polygon plus its connection graph edges.
type Area struct {
	ID       uint32
	Hull     uint8
	DynFlags int64
	Corners  []mapgeom.Vec3
	// Neighbors lists area IDs reachable directly from this area (duplicates
	// across polygon edges are removed).
	Neighbors []uint32
	LaddersUp []uint32
	LaddersDn []uint32
}

// Centroid returns the mean corner position.
func (a Area) Centroid() mapgeom.Vec3 {
	if len(a.Corners) == 0 {
		return mapgeom.Vec3{}
	}
	var sum mapgeom.Vec3
	for _, c := range a.Corners {
		sum[0] += c[0]
		sum[1] += c[1]
		sum[2] += c[2]
	}
	n := float32(len(a.Corners))
	return mapgeom.Vec3{sum[0] / n, sum[1] / n, sum[2] / n}
}

// containsXY reports whether the polygon covers (x, y); Z is ignored.
func (a Area) containsXY(x, y float32) bool {
	n := len(a.Corners)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := a.Corners[i][0], a.Corners[i][1]
		xj, yj := a.Corners[j][0], a.Corners[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}

// Mesh is a parsed navigation mesh.
type Mesh struct {
	Version   uint32
	Subver    uint32
	Analyzed  bool
	Areas     []Area
	byID      map[uint32]int
	source    string
	areaCount int
}

// Load parses a .nav file from disk.
func Load(path string) (*Mesh, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	m.source = path
	return m, nil
}

// Source returns the path the mesh was loaded from, if any.
func (m *Mesh) Source() string { return m.source }

// AreaCount returns the number of areas in the mesh.
func (m *Mesh) AreaCount() int { return len(m.Areas) }

// Area looks an area up by its numeric ID.
func (m *Mesh) Area(id uint32) (Area, bool) {
	i, ok := m.byID[id]
	if !ok {
		return Area{}, false
	}
	return m.Areas[i], true
}

// FindArea returns the area a world point stands in. Stacked floors overlap in
// the XY plane, so among containing polygons the one whose centroid Z is
// closest to the point wins.
func (m *Mesh) FindArea(p mapgeom.Vec3) (uint32, bool) {
	var best uint32
	bestDZ := float32(math.Inf(1))
	found := false
	for _, a := range m.Areas {
		if !a.containsXY(p[0], p[1]) {
			continue
		}
		dz := float32(math.Abs(float64(a.Centroid()[2] - p[2])))
		if dz < bestDZ {
			best, bestDZ, found = a.ID, dz, true
		}
	}
	return best, found
}

// Path is a route through the connection graph.
type Path struct {
	Areas    []uint32
	Distance float64
}

// FindPath returns the shortest route between two areas, weighted by centroid
// distance. When from == to the path holds a single area. Unreachable areas
// yield ErrNoPath.
func (m *Mesh) FindPath(from, to uint32) (Path, error) {
	if _, ok := m.byID[from]; !ok {
		return Path{}, fmt.Errorf("%w: unknown area %d", ErrInvalidNav, from)
	}
	if _, ok := m.byID[to]; !ok {
		return Path{}, fmt.Errorf("%w: unknown area %d", ErrInvalidNav, to)
	}
	if from == to {
		return Path{Areas: []uint32{from}}, nil
	}
	dist := map[uint32]float64{from: 0}
	prev := map[uint32]uint32{}
	done := map[uint32]bool{}
	queue := &costQueue{}
	push(queue, from, 0)
	for queue.Len() > 0 {
		item := pop(queue)
		if done[item.id] {
			continue
		}
		done[item.id] = true
		if item.id == to {
			break
		}
		area, _ := m.Area(item.id)
		centroid := area.Centroid()
		for _, next := range area.Neighbors {
			if done[next] {
				continue
			}
			nb, ok := m.Area(next)
			if !ok {
				continue
			}
			nc := nb.Centroid()
			dx := float64(centroid[0] - nc[0])
			dy := float64(centroid[1] - nc[1])
			dz := float64(centroid[2] - nc[2])
			cost := item.cost + math.Sqrt(dx*dx+dy*dy+dz*dz)
			if old, ok := dist[next]; ok && cost >= old {
				continue
			}
			dist[next] = cost
			prev[next] = item.id
			push(queue, next, cost)
		}
	}
	total, ok := dist[to]
	if !ok || !done[to] {
		return Path{}, fmt.Errorf("%w: %d -> %d", ErrNoPath, from, to)
	}
	route := []uint32{to}
	for cur := to; cur != from; {
		p, ok := prev[cur]
		if !ok {
			return Path{}, fmt.Errorf("%w: %d -> %d", ErrNoPath, from, to)
		}
		route = append(route, p)
		cur = p
	}
	for i, j := 0, len(route)-1; i < j; i, j = i+1, j-1 {
		route[i], route[j] = route[j], route[i]
	}
	return Path{Areas: route, Distance: total}, nil
}

// ErrNoPath reports that two areas are not connected.
var ErrNoPath = errors.New("no nav path")

// costQueue is a minimal binary min-heap keyed by cost.
type costQueue struct {
	items []costItem
}

type costItem struct {
	id   uint32
	cost float64
}

func (q *costQueue) Len() int { return len(q.items) }

func push(q *costQueue, id uint32, cost float64) {
	q.items = append(q.items, costItem{id, cost})
	i := len(q.items) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if q.items[parent].cost <= q.items[i].cost {
			break
		}
		q.items[parent], q.items[i] = q.items[i], q.items[parent]
		i = parent
	}
}

func pop(q *costQueue) costItem {
	top := q.items[0]
	last := len(q.items) - 1
	q.items[0] = q.items[last]
	q.items = q.items[:last]
	i := 0
	for {
		l, r := 2*i+1, 2*i+2
		smallest := i
		if l < len(q.items) && q.items[l].cost < q.items[smallest].cost {
			smallest = l
		}
		if r < len(q.items) && q.items[r].cost < q.items[smallest].cost {
			smallest = r
		}
		if smallest == i {
			break
		}
		q.items[smallest], q.items[i] = q.items[i], q.items[smallest]
		i = smallest
	}
	return top
}

// Parse reads a .nav file from memory.
func Parse(data []byte) (*Mesh, error) {
	r := &reader{data: data}
	magic, err := r.u32()
	if err != nil {
		return nil, err
	}
	if magic != FileMagic {
		return nil, fmt.Errorf("%w: bad magic %#x", ErrInvalidNav, magic)
	}
	version, err := r.u32()
	if err != nil {
		return nil, err
	}
	if version < MinVersion || version > MaxVersion {
		return nil, fmt.Errorf("%w: unsupported version %d (supported %d-%d)", ErrInvalidNav, version, MinVersion, MaxVersion)
	}
	subver, err := r.u32()
	if err != nil {
		return nil, err
	}
	flags, err := r.u32()
	if err != nil {
		return nil, err
	}
	m := &Mesh{Version: version, Subver: subver, Analyzed: flags&1 != 0, byID: map[uint32]int{}}

	if version >= 36 {
		if err := skipKV3(r); err != nil {
			return nil, err
		}
	}
	var polygons [][]mapgeom.Vec3
	if version >= 31 {
		polygons, err = readPolygons(r, version)
		if err != nil {
			return nil, err
		}
	}
	if version >= 32 {
		if err := r.skip(4); err != nil {
			return nil, err
		}
	}
	if version >= 35 {
		count, err := r.u32()
		if err != nil {
			return nil, err
		}
		if count > 1<<20 {
			return nil, fmt.Errorf("%w: implausible marker count %d", ErrInvalidNav, count)
		}
		for i := uint32(0); i < count; i++ {
			if err := r.skipCString(); err != nil {
				return nil, err
			}
			if err := r.skip(48); err != nil {
				return nil, err
			}
		}
	}
	if version >= 36 {
		if err := skipKV3(r); err != nil {
			return nil, err
		}
	}
	count, err := r.u32()
	if err != nil {
		return nil, err
	}
	if count > 1<<22 {
		return nil, fmt.Errorf("%w: implausible area count %d", ErrInvalidNav, count)
	}
	m.Areas = make([]Area, 0, count)
	for i := uint32(0); i < count; i++ {
		area, err := readArea(r, version, polygons)
		if err != nil {
			return nil, err
		}
		m.byID[area.ID] = len(m.Areas)
		m.Areas = append(m.Areas, area)
	}
	return m, nil
}

func readPolygons(r *reader, version uint32) ([][]mapgeom.Vec3, error) {
	cornerCount, err := r.u32()
	if err != nil {
		return nil, err
	}
	if cornerCount > 1<<25 {
		return nil, fmt.Errorf("%w: implausible corner count %d", ErrInvalidNav, cornerCount)
	}
	corners := make([]mapgeom.Vec3, cornerCount)
	for i := range corners {
		v, err := r.vec3()
		if err != nil {
			return nil, err
		}
		corners[i] = v
	}
	polyCount, err := r.u32()
	if err != nil {
		return nil, err
	}
	if polyCount > 1<<22 {
		return nil, fmt.Errorf("%w: implausible polygon count %d", ErrInvalidNav, polyCount)
	}
	polygons := make([][]mapgeom.Vec3, 0, polyCount)
	for i := uint32(0); i < polyCount; i++ {
		n, err := r.u8()
		if err != nil {
			return nil, err
		}
		poly := make([]mapgeom.Vec3, 0, n)
		for j := uint8(0); j < n; j++ {
			idx, err := r.u32()
			if err != nil {
				return nil, err
			}
			if int(idx) >= len(corners) {
				return nil, fmt.Errorf("%w: polygon corner index %d out of range", ErrInvalidNav, idx)
			}
			poly = append(poly, corners[idx])
		}
		if version >= 35 {
			if err := r.skip(4); err != nil {
				return nil, err
			}
		}
		polygons = append(polygons, poly)
	}
	return polygons, nil
}

func readArea(r *reader, version uint32, polygons [][]mapgeom.Vec3) (Area, error) {
	id, err := r.u32()
	if err != nil {
		return Area{}, err
	}
	dyn, err := r.i64()
	if err != nil {
		return Area{}, err
	}
	hull, err := r.u8()
	if err != nil {
		return Area{}, err
	}
	area := Area{ID: id, Hull: hull, DynFlags: dyn}
	if version >= 31 {
		pi, err := r.u32()
		if err != nil {
			return Area{}, err
		}
		if int(pi) >= len(polygons) {
			return Area{}, fmt.Errorf("%w: area polygon index %d out of range", ErrInvalidNav, pi)
		}
		area.Corners = polygons[pi]
	} else {
		cornerCount, err := r.u32()
		if err != nil {
			return Area{}, err
		}
		if cornerCount > 1<<12 {
			return Area{}, fmt.Errorf("%w: implausible corner count %d", ErrInvalidNav, cornerCount)
		}
		area.Corners = make([]mapgeom.Vec3, cornerCount)
		for i := range area.Corners {
			v, err := r.vec3()
			if err != nil {
				return Area{}, err
			}
			area.Corners[i] = v
		}
	}
	if err := r.skip(4); err != nil {
		return Area{}, err
	}
	seen := map[uint32]bool{}
	for range area.Corners {
		connCount, err := r.u32()
		if err != nil {
			return Area{}, err
		}
		if connCount > 1<<10 {
			return Area{}, fmt.Errorf("%w: implausible connection count %d", ErrInvalidNav, connCount)
		}
		for j := uint32(0); j < connCount; j++ {
			neighbor, err := r.u32()
			if err != nil {
				return Area{}, err
			}
			if _, err := r.u32(); err != nil { // edge id, unused
				return Area{}, err
			}
			if !seen[neighbor] {
				seen[neighbor] = true
				area.Neighbors = append(area.Neighbors, neighbor)
			}
		}
	}
	if err := r.skip(5); err != nil { // legacy hiding spots + encounter paths
		return Area{}, err
	}
	if area.LaddersUp, err = readIDs(r); err != nil {
		return Area{}, err
	}
	if area.LaddersDn, err = readIDs(r); err != nil {
		return Area{}, err
	}
	return area, nil
}

func readIDs(r *reader) ([]uint32, error) {
	count, err := r.u32()
	if err != nil {
		return nil, err
	}
	if count > 1<<16 {
		return nil, fmt.Errorf("%w: implausible id count %d", ErrInvalidNav, count)
	}
	ids := make([]uint32, count)
	for i := range ids {
		if ids[i], err = r.u32(); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// skipKV3 advances past an embedded BinaryKV3 document. Shipped navs store
// empty placeholders, so only the container header is needed to compute length.
func skipKV3(r *reader) error {
	if err := r.align8(); err != nil {
		return err
	}
	magic, err := r.u32()
	if err != nil {
		return err
	}
	if magic == 0x0356_4B56 {
		return fmt.Errorf("%w: KV3 v0 document not supported", ErrInvalidNav)
	}
	version := magic & 0xFF
	if magic&kv3MagicMask != kv3MagicMask || version < 2 || version > 5 {
		return fmt.Errorf("%w: unsupported KV3 signature %#x", ErrInvalidNav, magic)
	}
	if err := r.skip(16); err != nil { // format GUID
		return err
	}
	method, err := r.u32()
	if err != nil {
		return err
	}
	if err := r.skip(4); err != nil { // dictionary id + frame size
		return err
	}
	if err := r.skip(16); err != nil { // four count fields
		return err
	}
	if err := r.skip(4); err != nil { // objects + arrays
		return err
	}
	sizeUncTotal, err := r.i32()
	if err != nil {
		return err
	}
	sizeCmpTotal, err := r.i32()
	if err != nil {
		return err
	}
	nBlocks, err := r.i32()
	if err != nil {
		return err
	}
	nBlobs, err := r.i32()
	if err != nil {
		return err
	}
	var sizeBlockCmp int64
	if version >= 4 {
		if err := r.skip(4); err != nil { // countBytes2
			return err
		}
		blocks, err := r.i32()
		if err != nil {
			return err
		}
		sizeBlockCmp = int64(blocks)
	}
	var unc1, cmp1, unc2, cmp2 int32
	if version >= 5 {
		if unc1, err = r.i32(); err != nil {
			return err
		}
		if cmp1, err = r.i32(); err != nil {
			return err
		}
		if unc2, err = r.i32(); err != nil {
			return err
		}
		if cmp2, err = r.i32(); err != nil {
			return err
		}
		if err := r.skip(32); err != nil { // eight buffer-2 count fields
			return err
		}
	} else {
		unc1, cmp1 = sizeUncTotal, sizeCmpTotal
	}
	if method > 2 {
		return fmt.Errorf("%w: unsupported KV3 compression method %d", ErrInvalidNav, method)
	}
	if nBlocks != 0 || nBlobs != 0 {
		return fmt.Errorf("%w: KV3 block/blob data unsupported (blocks=%d, blobs=%d)", ErrInvalidNav, nBlocks, nBlobs)
	}
	onDisk := func(unc, cmp int32) int64 {
		if method == 0 {
			return int64(unc)
		}
		return int64(cmp)
	}
	body := onDisk(unc1, cmp1) + onDisk(unc2, cmp2) + sizeBlockCmp
	if body < 0 || body > int64(len(r.data)) {
		return fmt.Errorf("%w: bad KV3 body size %d", ErrInvalidNav, body)
	}
	return r.skip(int(body))
}

// reader is a bounds-checked little-endian cursor.
type reader struct {
	data []byte
	pos  int
}

func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || r.pos+n > len(r.data) {
		return nil, fmt.Errorf("%w: unexpected end of data at %d (+%d)", ErrInvalidNav, r.pos, n)
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *reader) skip(n int) error {
	_, err := r.take(n)
	return err
}

func (r *reader) align8() error {
	pad := (8 - r.pos%8) % 8
	return r.skip(pad)
}

func (r *reader) u8() (uint8, error) {
	b, err := r.take(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (r *reader) u32() (uint32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (r *reader) i32() (int32, error) {
	v, err := r.u32()
	return int32(v), err
}

func (r *reader) i64() (int64, error) {
	b, err := r.take(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b)), nil
}

func (r *reader) vec3() (mapgeom.Vec3, error) {
	b, err := r.take(12)
	if err != nil {
		return mapgeom.Vec3{}, err
	}
	return mapgeom.Vec3{
		math.Float32frombits(binary.LittleEndian.Uint32(b[0:])),
		math.Float32frombits(binary.LittleEndian.Uint32(b[4:])),
		math.Float32frombits(binary.LittleEndian.Uint32(b[8:])),
	}, nil
}

func (r *reader) skipCString() error {
	for {
		b, err := r.u8()
		if err != nil {
			return err
		}
		if b == 0 {
			return nil
		}
	}
}

// SortedIDs returns the area IDs in ascending order (useful for CLI output).
func (m *Mesh) SortedIDs() []uint32 {
	ids := make([]uint32, 0, len(m.Areas))
	for _, a := range m.Areas {
		ids = append(ids, a.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
