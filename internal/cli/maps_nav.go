package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"strikeye/internal/callouts"
	"strikeye/internal/mapdata"
	"strikeye/internal/mapgeom"
	"strikeye/internal/navmesh"
)

// navCacheDir is where downloaded .nav meshes live.
func navCacheDir(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "map_nav")
}

// calloutsCachePath is where the downloaded callout table lives.
func calloutsCachePath(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "map_data", "callouts.csv")
}

func runNavsList(dataRoot string, out io.Writer) error {
	cached, err := mapdata.CachedNavs(navCacheDir(dataRoot))
	if err != nil {
		return err
	}
	calloutsPath := calloutsCachePath(dataRoot)
	if _, err := filepath.Abs(calloutsPath); err != nil {
		return err
	}
	doc := struct {
		CacheDir string   `json:"cache_dir"`
		Release  string   `json:"release"`
		Cached   []string `json:"cached"`
		Callouts string   `json:"callouts"`
	}{navCacheDir(dataRoot), mapdata.NavRelease, cached, calloutsPath}
	if cached == nil {
		doc.Cached = []string{}
	}
	return writeIndentedJSON(out, doc)
}

func runNavsFetch(dataRoot, baseURL string, requested []string, all bool, out io.Writer) error {
	if !all && len(requested) == 0 {
		return fmt.Errorf("--map or --all is required")
	}
	if baseURL == "" {
		baseURL = mapdata.NavBaseURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Minute}

	manifest, err := mapdata.FetchNavManifest(ctx, client, baseURL)
	if err != nil {
		return err
	}
	maps := requested
	if all {
		maps = manifest.SortedMaps()
	} else {
		for i, name := range maps {
			maps[i] = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".nav")
		}
	}
	paths, err := mapdata.EnsureNavs(ctx, client, navCacheDir(dataRoot), baseURL, manifest, maps, func(name string) {
		fmt.Fprintf(out, "downloading %s...\n", name)
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	doc := struct {
		Release       string   `json:"release"`
		PatchVersion  string   `json:"patch_version"`
		ClientVersion string   `json:"client_version"`
		Maps          []string `json:"maps"`
		Paths         []string `json:"paths"`
	}{manifest.ReleaseKey, manifest.PatchVersion, manifest.ClientVersion, maps, paths}
	return writeIndentedJSON(out, doc)
}

func runCalloutsFetch(dataRoot string, out io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Minute}
	path, err := mapdata.EnsureCallouts(ctx, client, calloutsCachePath(dataRoot))
	if err != nil {
		return err
	}
	table, err := callouts.LoadCSV(path)
	if err != nil {
		return err
	}
	doc := struct {
		Path   string   `json:"path"`
		Source string   `json:"source"`
		Maps   []string `json:"maps"`
	}{path, mapdata.CalloutsURL, table.Maps()}
	return writeIndentedJSON(out, doc)
}

func loadCalloutsTable(dataRoot, mapName string) (*callouts.Set, error) {
	table, err := callouts.LoadCSV(calloutsCachePath(dataRoot))
	if err != nil {
		if isNotExist(err) {
			return nil, fmt.Errorf("callout table missing (%s); run: strikeye-cli callouts-fetch", calloutsCachePath(dataRoot))
		}
		return nil, err
	}
	return table.For(mapName)
}

func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

func loadNavMesh(dataRoot, mapName string) (*navmesh.Mesh, error) {
	path := filepath.Join(navCacheDir(dataRoot), strings.ToLower(mapName)+".nav")
	mesh, err := navmesh.Load(path)
	if err != nil {
		if isNotExist(err) {
			return nil, fmt.Errorf("nav mesh missing for %s (%s); run: strikeye-cli navs-fetch --map %s", mapName, path, mapName)
		}
		return nil, err
	}
	return mesh, nil
}

func runArea(dataRoot, mapName string, point mapgeom.Vec3, out io.Writer) error {
	mesh, err := loadNavMesh(dataRoot, mapName)
	if err != nil {
		return err
	}
	set, err := loadCalloutsTable(dataRoot, mapName)
	if err != nil {
		return err
	}
	areaID, areaOK := mesh.FindArea(point)
	place, placeOK := set.At(point)
	doc := struct {
		Map       string    `json:"map"`
		Point     []float32 `json:"point"`
		AreaID    uint32    `json:"area_id"`
		AreaOK    bool      `json:"area_found"`
		Callout   string    `json:"callout"`
		PlaceID   string    `json:"place_id"`
		CalloutOK bool      `json:"callout_found"`
	}{Map: mapName, Point: []float32{point[0], point[1], point[2]}, AreaID: areaID, AreaOK: areaOK,
		Callout: place.Label, PlaceID: place.ID, CalloutOK: placeOK}
	return writeIndentedJSON(out, doc)
}

// runNavPath routes between two named places using the nav mesh.
func runNavPath(dataRoot, mapName, from, to string, out io.Writer) error {
	mesh, err := loadNavMesh(dataRoot, mapName)
	if err != nil {
		return err
	}
	set, err := loadCalloutsTable(dataRoot, mapName)
	if err != nil {
		return err
	}
	resolve := func(label string) (callouts.Place, uint32, error) {
		place, ok := set.Find(label)
		if !ok {
			for _, p := range set.Places() {
				if strings.EqualFold(p.Label, label) {
					place, ok = p, true
					break
				}
			}
		}
		if !ok {
			return callouts.Place{}, 0, fmt.Errorf("unknown place %q on %s; run: strikeye-cli callouts-fetch and pick an existing callout", label, mapName)
		}
		area, ok := mesh.FindArea(place.Centroid())
		if !ok {
			return callouts.Place{}, 0, fmt.Errorf("no nav area at %s (%.0f, %.0f, %.0f)", place.Label, place.Centroid()[0], place.Centroid()[1], place.Centroid()[2])
		}
		return place, area, nil
	}
	fromPlace, fromArea, err := resolve(from)
	if err != nil {
		return err
	}
	toPlace, toArea, err := resolve(to)
	if err != nil {
		return err
	}
	path, err := mesh.FindPath(fromArea, toArea)
	if err != nil {
		return err
	}
	const unitsPerMeter = 39.37
	const runSpeed = 250.0 // hammer units per second with a knife out
	doc := struct {
		Map        string   `json:"map"`
		From       string   `json:"from"`
		To         string   `json:"to"`
		FromArea   uint32   `json:"from_area"`
		ToArea     uint32   `json:"to_area"`
		Hops       int      `json:"hops"`
		Units      float64  `json:"units"`
		Meters     float64  `json:"meters"`
		RunSeconds float64  `json:"run_seconds"`
		AreaPath   []uint32 `json:"area_path"`
	}{Map: mapName, From: fromPlace.Label, To: toPlace.Label, FromArea: fromArea, ToArea: toArea,
		Hops: len(path.Areas), Units: path.Distance, Meters: path.Distance / unitsPerMeter,
		RunSeconds: path.Distance / runSpeed, AreaPath: path.Areas}
	return writeIndentedJSON(out, doc)
}

func writeIndentedJSON(out io.Writer, doc any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
