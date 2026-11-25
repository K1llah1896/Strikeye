package parser

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"strikeye/internal/mapgeom"
)

// wallGeometry builds a 100x100 unit wall at z in [0,128] around the origin.
func wallGeometry(t *testing.T) *mapgeom.Tracer {
	t.Helper()
	var raw bytes.Buffer
	raw.WriteString("CSDAGEO\x00")
	for _, v := range []uint32{1, 4, 6, 1} {
		_ = binary.Write(&raw, binary.LittleEndian, v)
	}
	verts := []float32{
		-100, 0, 0, 100, 0, 0, 100, 0, 128, -100, 0, 128,
	}
	for _, f := range verts {
		_ = binary.Write(&raw, binary.LittleEndian, f)
	}
	for _, i := range []uint32{0, 1, 2, 0, 2, 3} {
		_ = binary.Write(&raw, binary.LittleEndian, i)
	}
	for _, f := range []float32{-100, -1, 0, 100, 1, 128} {
		_ = binary.Write(&raw, binary.LittleEndian, f)
	}
	_ = binary.Write(&raw, binary.LittleEndian, uint32(0))
	_ = binary.Write(&raw, binary.LittleEndian, uint32(2))

	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	if _, err := gz.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "de_test"+mapgeom.Extension)
	if err := os.WriteFile(path, gzBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	bvh, _, err := mapgeom.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return mapgeom.NewTracer(bvh)
}

func TestVisibleRunMSStopsAtWall(t *testing.T) {
	tr := wallGeometry(t)
	const tickRate = 64.0
	// 10 samples at 64 Hz (15.6 ms apart) behind and in front of the wall.
	var samples []visSample
	for i := 0; i < 20; i++ {
		tick := int64(i * 2)
		pos := map[string][3]float64{
			"att": {-50, -200, 64},
			"vic": {-50, -200, 64},
		}
		if i >= 10 {
			pos["vic"] = [3]float64{-50, -200, 64}
		} else {
			pos["vic"] = [3]float64{-50, 200, 64}
		}
		samples = append(samples, visSample{Tick: tick, Pos: pos})
	}
	ms, ok := visibleRunMS(tr, samples, "att", "vic", 38, int64(5*tickRate), tickRate)
	if !ok {
		t.Fatalf("expected a visible run for the last samples")
	}
	if ms < 250 || ms > 320 {
		t.Fatalf("expected roughly 5 visible samples, got %.1fms", ms)
	}
	if _, ok := visibleRunMS(tr, samples, "att", "vic", 10, int64(5*tickRate), tickRate); ok {
		t.Fatal("victim was behind the wall, no run expected")
	}
}

func TestVisibleRunMSRequiresWindow(t *testing.T) {
	tr := wallGeometry(t)
	samples := []visSample{{Tick: 0, Pos: map[string][3]float64{"att": {0, -200, 64}, "vic": {0, -200, 64}}}}
	if _, ok := visibleRunMS(tr, samples, "att", "vic", 5000, 320, 64); ok {
		t.Fatal("sample outside the window must not count")
	}
}

func TestVisibleRunMSRespectsFOV(t *testing.T) {
	tr := wallGeometry(t)
	const tickRate = 64.0
	// No wall between the pair (both at y=-200), victim 200 units along +X.
	build := func(yaw float64) []visSample {
		var samples []visSample
		for i := 0; i < 12; i++ {
			tick := int64(i * 2)
			samples = append(samples, visSample{
				Tick: tick,
				Pos: map[string][3]float64{
					"att": {0, -200, 64},
					"vic": {200, -200, 64},
				},
				Yaw:   map[string]float64{"att": yaw},
				Blind: map[string]float64{"att": 0},
			})
		}
		return samples
	}
	if ms, ok := visibleRunMS(tr, build(0), "att", "vic", 22, int64(5*tickRate), tickRate); !ok || ms <= 0 {
		t.Fatalf("facing the victim should be visible, ms=%.1f ok=%v", ms, ok)
	}
	if _, ok := visibleRunMS(tr, build(180), "att", "vic", 22, int64(5*tickRate), tickRate); ok {
		t.Fatal("facing away must not count as seen")
	}
	// 30° off axis is inside a 53° half angle but outside a 20° one.
	if _, ok := visibleRunMSOptics(tr, build(30), "att", "vic", 22, int64(5*tickRate), tickRate, 20); ok {
		t.Fatal("20° cone should reject 30° offset")
	}
	if _, ok := visibleRunMSOptics(tr, build(30), "att", "vic", 22, int64(5*tickRate), tickRate, 53); !ok {
		t.Fatal("53° cone should accept 30° offset")
	}
}

func TestVisibleRunMSRespectsBlindness(t *testing.T) {
	tr := wallGeometry(t)
	const tickRate = 64.0
	samples := make([]visSample, 0, 10)
	for i := 0; i < 10; i++ {
		samples = append(samples, visSample{
			Tick: int64(i * 2),
			Pos: map[string][3]float64{
				"att": {0, -200, 64},
				"vic": {200, -200, 64},
			},
			Yaw:   map[string]float64{"att": 0},
			Blind: map[string]float64{"att": 1.4},
		})
	}
	if _, ok := visibleRunMS(tr, samples, "att", "vic", 18, int64(5*tickRate), tickRate); ok {
		t.Fatal("a blinded attacker must not record a visible run")
	}
	for i := range samples {
		samples[i].Blind["att"] = 0.05
	}
	if _, ok := visibleRunMS(tr, samples, "att", "vic", 18, int64(5*tickRate), tickRate); !ok {
		t.Fatal("residual flash below the threshold must not block visibility")
	}
}

func TestAngleBetweenDeg(t *testing.T) {
	cases := []struct {
		yaw  float64
		dir  [3]float64
		want float64
	}{
		{0, [3]float64{1, 0, 0}, 0},
		{90, [3]float64{0, 1, 0}, 0},
		{0, [3]float64{0, 1, 0}, 90},
		{180, [3]float64{-1, 0, 0}, 0},
		{0, [3]float64{0, 0, 0}, 0},
	}
	for _, c := range cases {
		if got := angleBetweenDeg(c.yaw, c.dir); math.Abs(got-c.want) > 1e-6 {
			t.Fatalf("angleBetweenDeg(%v,%v) = %v want %v", c.yaw, c.dir, got, c.want)
		}
	}
}
