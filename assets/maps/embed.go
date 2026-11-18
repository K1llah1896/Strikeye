package maps

import (
	"embed"
	"encoding/json"
	"math"
	"strings"
)

//go:embed *.png map-data.json
var fs embed.FS

// MapMeta describes CS2 radar image world-to-pixel coordinate transformation.
type MapMeta struct {
	PosX               float64 `json:"pos_x"`
	PosY               float64 `json:"pos_y"`
	Scale              float64 `json:"scale"`
	Rotate             *int    `json:"rotate,omitempty"`
	Zoom               *float64 `json:"zoom,omitempty"`
	LowerLevelMaxUnits float64 `json:"lower_level_max_units"`
	ContentX           int     `json:"content_x"`
	ContentY           int     `json:"content_y"`
	ContentWidth       int     `json:"content_width"`
	ContentHeight      int     `json:"content_height"`
	TransformVersion   int     `json:"transform_version"`
}

var parsedMeta map[string]MapMeta

func init() {
	raw, err := fs.ReadFile("map-data.json")
	if err != nil {
		parsedMeta = map[string]MapMeta{}
		return
	}
	var m map[string]MapMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		parsedMeta = map[string]MapMeta{}
		return
	}
	parsedMeta = m
}

// LookupMap returns coordinate metadata for the given CS2 map name (case-insensitive).
func LookupMap(name string) (MapMeta, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	meta, ok := parsedMeta[key]
	return meta, ok
}

// LoadRadarPNG returns the embedded PNG bytes for the upper or lower radar layer.
func LoadRadarPNG(name string, lower bool) ([]byte, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil, false
	}
	if lower {
		if data, err := fs.ReadFile(key + "_lower.png"); err == nil {
			return data, true
		}
	}
	data, err := fs.ReadFile(key + ".png")
	if err != nil {
		return nil, false
	}
	return data, true
}

// HasLowerLayer reports whether a lower-level radar PNG is bundled for the map.
func HasLowerLayer(name string) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return false
	}
	_, err := fs.ReadFile(key + "_lower.png")
	return err == nil
}

// IsLowerLevel reports whether a world Z coordinate belongs to the lower radar layer.
func (m MapMeta) IsLowerLevel(worldZ float64) bool {
	if m.LowerLevelMaxUnits <= -999999 {
		return false
	}
	return worldZ <= m.LowerLevelMaxUnits
}

// WorldToRadar converts CS2 world coordinates (X, Y) into pixel coordinates on a
// 1024x1024 radar image scaled to targetWidth x targetHeight.
func WorldToRadar(meta MapMeta, worldX, worldY, targetWidth, targetHeight float64) (float64, float64) {
	scale := meta.Scale
	if scale <= 0 {
		scale = 1
	}
	// Standard Source 2 radar projection on a 1024x1024 base canvas.
	px1024 := (worldX - meta.PosX) / scale
	py1024 := (meta.PosY - worldY) / scale
	return px1024 * (targetWidth / 1024.0), py1024 * (targetHeight / 1024.0)
}

// DefaultFallbackMeta computes a synthetic MapMeta bounding box for unknown/custom maps
// so the procedural grid fallback can render 2D replays and heatmaps cleanly.
func DefaultFallbackMeta(minX, maxX, minY, maxY float64) MapMeta {
	spanX := math.Abs(maxX - minX)
	spanY := math.Abs(maxY - minY)
	span := math.Max(spanX, spanY)
	if span < 512 {
		span = 2048
	}
	pad := span * 0.1
	scale := (span + 2*pad) / 1024.0
	return MapMeta{
		PosX:               minX - pad,
		PosY:               maxY + pad,
		Scale:              scale,
		LowerLevelMaxUnits: -1000000,
		ContentWidth:       1024,
		ContentHeight:      1024,
	}
}
