package main

import (
	"context"
	"encoding/json"
	typeStringSlice "flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"strikeye/internal/mapgeom"
)

// mapListFlag collects repeatable --map values.
type mapListFlag []string

func (m *mapListFlag) String() string { return strings.Join(*m, ",") }

func (m *mapListFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

var _ typeStringSlice.Value = (*mapListFlag)(nil)

func geometryCacheDir(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "map_geometry")
}

func runMapsList(dataRoot string, out io.Writer) error {
	dir := geometryCacheDir(dataRoot)
	cached, err := mapgeom.CachedMaps(dir)
	if err != nil {
		return err
	}
	doc := struct {
		CacheDir string   `json:"cache_dir"`
		Cached   []string `json:"cached"`
	}{dir, cached}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func runMapsFetch(dataRoot, baseURL string, requested []string, all bool, out io.Writer) error {
	if !all && len(requested) == 0 {
		return fmt.Errorf("--map or --all is required")
	}
	if baseURL == "" {
		baseURL = mapgeom.DefaultBaseURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Minute}

	manifest, err := mapgeom.FetchManifest(ctx, client, baseURL)
	if err != nil {
		return err
	}
	maps := requested
	if all {
		maps = manifest.MapNames()
	} else {
		for i, name := range maps {
			maps[i] = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), mapgeom.Extension)
		}
	}
	paths, err := mapgeom.EnsureMaps(ctx, client, geometryCacheDir(dataRoot), baseURL, manifest, maps, func(name string) {
		fmt.Fprintf(out, "downloading %s geometry...\n", name)
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	doc := struct {
		Source string   `json:"source"`
		Paths  []string `json:"paths"`
	}{baseURL, paths}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
