package settings

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"strikeye/internal/mapdata"
	"strikeye/internal/mapgeom"
)

// mapDataBusyTimeout bounds a nav download; the archive is ~7.7MB and the
// individual .nav files are 5-20MB once extracted.
const mapDataBusyTimeout = 10 * time.Minute

// mapDataPaths returns the cache locations the analysis tabs read from.
func mapDataPaths(dataRoot string) (navDir, calloutsPath string) {
	return filepath.Join(dataRoot, "cache", "map_nav"), filepath.Join(dataRoot, "cache", "map_data", "callouts.csv")
}

// mapDataStatus summarises what is cached so the settings row can show it.
func mapDataStatus(dataRoot string) (navs []string, calloutsBytes int64) {
	navDir, calloutsPath := mapDataPaths(dataRoot)
	if cached, err := mapdata.CachedNavs(navDir); err == nil {
		navs = cached
	}
	if info, err := os.Stat(calloutsPath); err == nil {
		calloutsBytes = info.Size()
	}
	return navs, calloutsBytes
}

// fetchNavs downloads the navigation mesh archive and extracts the requested
// maps. An empty maps slice means "every official map".
func fetchNavs(ctx context.Context, dataRoot string, maps []string) ([]string, error) {
	navDir, _ := mapDataPaths(dataRoot)
	if err := os.MkdirAll(navDir, 0o755); err != nil {
		return nil, fmt.Errorf("create nav cache: %w", err)
	}
	client := &http.Client{Timeout: mapDataBusyTimeout}
	manifest, err := mapdata.FetchNavManifest(ctx, client, mapdata.NavBaseURL)
	if err != nil {
		return nil, fmt.Errorf("fetch nav manifest: %w", err)
	}
	targets := maps
	if len(targets) == 0 {
		targets = manifest.SortedMaps()
	}
	return mapdata.EnsureNavs(ctx, client, navDir, mapdata.NavBaseURL, manifest, targets, nil)
}

// fetchCallouts downloads the place-name CSV used to label zones.
func fetchCallouts(ctx context.Context, dataRoot string) (string, error) {
	_, calloutsPath := mapDataPaths(dataRoot)
	if err := os.MkdirAll(filepath.Dir(calloutsPath), 0o755); err != nil {
		return "", fmt.Errorf("create callout cache: %w", err)
	}
	client := &http.Client{Timeout: mapDataBusyTimeout}
	return mapdata.EnsureCallouts(ctx, client, calloutsPath)
}

// geometryDir is where the collision geometry (.tri) lives; TTD and per-kill
// line-of-sight read from here.
func geometryDir(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "map_geometry")
}

// fetchGeometry downloads the collision meshes for the requested maps (all maps
// when the list is empty) and returns their paths.
func fetchGeometry(ctx context.Context, dataRoot string, maps []string) ([]string, error) {
	dir := geometryDir(dataRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create geometry cache: %w", err)
	}
	client := &http.Client{Timeout: mapDataBusyTimeout}
	manifest, err := mapgeom.FetchManifest(ctx, client, mapgeom.DefaultBaseURL)
	if err != nil {
		return nil, err
	}
	targets := maps
	if len(targets) == 0 {
		targets = manifest.MapNames()
	}
	return mapgeom.EnsureMaps(ctx, client, dir, mapgeom.DefaultBaseURL, manifest, targets, nil)
}

// geometryCount reports how many collision meshes are cached.
func geometryCount(dataRoot string) int {
	files, err := mapgeom.CachedMaps(geometryDir(dataRoot))
	if err != nil {
		return 0
	}
	return len(files)
}
