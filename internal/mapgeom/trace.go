package mapgeom

import "math"

// segmentEndEpsilon ignores intersections at the segment ends so a point lying on
// a surface (a player's feet on the ground) is not occluded by it.
const segmentEndEpsilon = 1e-4

// Tracer answers segment queries against one BVH. It is not safe for concurrent
// use; create one per goroutine.
type Tracer struct {
	bvh   *BVH
	stack []uint32
}

// NewTracer returns a tracer for bvh.
func NewTracer(bvh *BVH) *Tracer {
	return &Tracer{bvh: bvh, stack: make([]uint32, 0, 64)}
}

// IsOccluded reports whether any triangle blocks the segment a-b.
func (t *Tracer) IsOccluded(a, b [3]float64) bool {
	if t.bvh == nil || len(t.bvh.Nodes) == 0 || len(t.bvh.Indices) == 0 {
		return false
	}
	origin := a
	direction := [3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
	var invDir [3]float64
	for i := range 3 {
		invDir[i] = 1 / direction[i]
	}

	stack := t.stack[:0]
	defer func() { t.stack = stack[:0] }()

	nodeIndex := uint32(0)
	for {
		node := &t.bvh.Nodes[nodeIndex]
		if intersectsAABB(node, origin, invDir) {
			if node.Count == 0 {
				stack = append(stack, node.LeftOrFirst+1)
				nodeIndex = node.LeftOrFirst
				continue
			}
			first := node.LeftOrFirst
			for tri := first; tri < first+node.Count; tri++ {
				if t.intersectsTriangle(tri, origin, direction) {
					return true
				}
			}
		}
		if len(stack) == 0 {
			return false
		}
		nodeIndex = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
	}
}

func intersectsAABB(node *Node, origin, invDir [3]float64) bool {
	tMin, tMax := 0.0, 1.0
	for axis := range 3 {
		t1 := (float64(node.Min[axis]) - origin[axis]) * invDir[axis]
		t2 := (float64(node.Max[axis]) - origin[axis]) * invDir[axis]
		if math.IsNaN(t1) || math.IsNaN(t2) {
			if origin[axis] < float64(node.Min[axis]) || origin[axis] > float64(node.Max[axis]) {
				return false
			}
			continue
		}
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		tMin = math.Max(tMin, t1)
		tMax = math.Min(tMax, t2)
		if tMin > tMax {
			return false
		}
	}
	return true
}

func (t *Tracer) intersectsTriangle(tri uint32, origin, direction [3]float64) bool {
	bvh := t.bvh
	off := tri * 3
	v0 := bvh.Vertices[bvh.Indices[off]]
	v1 := bvh.Vertices[bvh.Indices[off+1]]
	v2 := bvh.Vertices[bvh.Indices[off+2]]

	e1 := [3]float64{float64(v1[0] - v0[0]), float64(v1[1] - v0[1]), float64(v1[2] - v0[2])}
	e2 := [3]float64{float64(v2[0] - v0[0]), float64(v2[1] - v0[1]), float64(v2[2] - v0[2])}

	p := cross(direction, e2)
	det := dot(e1, p)
	if math.Abs(det) < 1e-12 {
		return false
	}
	invDet := 1 / det
	s := [3]float64{origin[0] - float64(v0[0]), origin[1] - float64(v0[1]), origin[2] - float64(v0[2])}
	u := dot(s, p) * invDet
	if u < 0 || u > 1 {
		return false
	}
	q := cross(s, e1)
	v := dot(direction, q) * invDet
	if v < 0 || u+v > 1 {
		return false
	}
	dist := dot(e2, q) * invDet
	return dist > segmentEndEpsilon && dist < 1-segmentEndEpsilon
}

func cross(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

func dot(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
