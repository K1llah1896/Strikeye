package mapgeom

import "strikeye/internal/mapassets"

// LoadForMap loads the collision geometry for mapName from the DataRoot cache,
// extracting it from the bundled map_geometry.zip sidecar the first time the map
// is needed. It returns ok=false when neither the cache nor a bundle has the
// map, which callers treat as "geometry-dependent metrics unavailable".
func LoadForMap(dataRoot, mapName string) (*BVH, bool) {
	return LoadForMapWithBundle(dataRoot, "", mapName)
}

// LoadForMapWithBundle is LoadForMap with an explicit geometry bundle path. An
// empty bundle falls back to the auto-detected sidecar next to the executables.
func LoadForMapWithBundle(dataRoot, bundlePath, mapName string) (*BVH, bool) {
	var (
		path string
		ok   bool
	)
	if bundlePath != "" {
		path, ok = mapassets.EnsureGeometryFrom(dataRoot, bundlePath, mapName)
	} else {
		path, ok = mapassets.EnsureGeometry(dataRoot, mapassets.ExeDir(), mapName)
	}
	if !ok {
		return nil, false
	}
	bvh, _, err := Load(path)
	if err != nil {
		return nil, false
	}
	return bvh, true
}
