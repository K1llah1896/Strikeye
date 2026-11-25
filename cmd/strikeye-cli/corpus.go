package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"strikeye/internal/corpus"
)

// runCorpus builds or verifies golden metrics snapshots for the regression
// corpus (tests/*.dem by default).
func runCorpus(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("corpus", flag.ContinueOnError)
	fs.SetOutput(out)
	dir := fs.String("dir", "tests", "Directory holding fixture demos")
	goldenDir := fs.String("golden", filepath.Join("tests", "fixtures", "golden"), "Golden snapshot directory")
	update := fs.Bool("update", false, "Rewrite golden snapshots from the current parse")
	only := fs.String("demo", "", "Only process this fixture filename")
	workDir := fs.String("work-dir", "", "Reusable scratch data root (default: temp)")
	geomDir := fs.String("geometry-dir", "", "Directory of .tri map geometry copied into the scratch cache (enables TTD metrics)")
	geomBundle := fs.String("geometry-bundle", "", "map_geometry.zip sidecar to extract TTD geometry from (default: auto-detected)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	entries, err := os.ReadDir(*dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", *dir, err)
	}
	var demos []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".dem") {
			continue
		}
		if *only != "" && !strings.EqualFold(e.Name(), *only) {
			continue
		}
		demos = append(demos, filepath.Join(*dir, e.Name()))
	}
	sort.Strings(demos)
	if len(demos) == 0 {
		return fmt.Errorf("no .dem fixtures in %s", *dir)
	}

	type report struct {
		Demo      string   `json:"demo"`
		Golden    string   `json:"golden"`
		Updated   bool     `json:"updated"`
		Matches   bool     `json:"matches"`
		Diffs     []string `json:"diffs,omitempty"`
		ElapsedMS int64    `json:"elapsed_ms"`
	}
	var reports []report
	failures := 0
	for _, demoPath := range demos {
		start := time.Now()
		snap, err := corpus.Build(ctx, demoPath, corpus.BuildOptions{
			WorkDir:        *workDir,
			GeometryDir:    *geomDir,
			GeometryBundle: *geomBundle,
		})
		if err != nil {
			return err
		}
		goldenPath := filepath.Join(*goldenDir, strings.TrimSuffix(filepath.Base(demoPath), ".dem")+".json")
		rep := report{Demo: filepath.Base(demoPath), Golden: goldenPath}

		if *update {
			if err := corpus.WriteSnapshot(goldenPath, snap); err != nil {
				return err
			}
			rep.Updated = true
			rep.Matches = true
		} else {
			want, err := corpus.ReadSnapshot(goldenPath)
			if err != nil {
				rep.Diffs = []string{fmt.Sprintf("golden missing or unreadable: %v", err)}
				failures++
			} else {
				rep.Diffs = corpus.Diff(want, snap)
				rep.Matches = len(rep.Diffs) == 0
				if !rep.Matches {
					failures++
				}
			}
		}
		rep.ElapsedMS = time.Since(start).Milliseconds()
		reports = append(reports, rep)
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"fixtures": reports,
		"failures": failures,
		"passed":   len(reports) - failures,
	}); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("%d fixture(s) diverged from golden metrics", failures)
	}
	return nil
}
