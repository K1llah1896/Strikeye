package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"strikeye/internal/gcpd"
)

type downloadOutput struct {
	Tab        string       `json:"tab"`
	Listed     int          `json:"listed"`
	Downloaded int          `json:"downloaded"`
	Reused     int          `json:"reused"`
	Skipped    int          `json:"skipped"`
	OutDir     string       `json:"out_dir"`
	Matches    []gcpd.Match `json:"matches"`
}

// runDownload lists GCPD matches for the given tabs and saves the demo archives
// into outDir, which defaults to the watcher import directory so the demos are
// picked up automatically.
func runDownload(ctx context.Context, dataRoot, cookieFile string, tabs []string, limit int, outDir string, out io.Writer) error {
	cookie := strings.TrimSpace(cookieFile)
	if st, err := os.Stat(cookieFile); err == nil && !st.IsDir() {
		loaded, err := gcpd.ReadCookieFile(cookieFile)
		if err != nil {
			return err
		}
		cookie = loaded
	}
	if strings.TrimSpace(cookie) == "" {
		return fmt.Errorf("no steam cookie: pass --cookies <file|cookie-header>")
	}

	if outDir == "" {
		outDir = filepath.Join(dataRoot, "media", "imports")
	}
	client := gcpd.NewClient(cookie)
	if base := strings.TrimSpace(os.Getenv("STRIKEYE_GCPD_BASE")); base != "" {
		client.BaseURL = base
	}

	var results []downloadOutput
	for _, tab := range tabs {
		matches, err := client.List(ctx, tab)
		if err != nil {
			return err
		}
		res := downloadOutput{Tab: tab, Listed: len(matches), OutDir: outDir}
		if limit > 0 && len(matches) > limit {
			matches = matches[:limit]
		}
		for _, m := range matches {
			existed := false
			if st, err := os.Stat(filepath.Join(outDir, m.Filename)); err == nil && st.Size() > 0 {
				existed = true
			}
			path, err := client.Download(ctx, m, outDir)
			if err != nil {
				return fmt.Errorf("download %s: %w", m.Filename, err)
			}
			m.URL = path
			if existed {
				res.Reused++
			} else {
				res.Downloaded++
			}
			res.Matches = append(res.Matches, m)
		}
		if limit > 0 {
			res.Skipped = res.Listed - len(res.Matches)
		}
		results = append(results, res)
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"results": results})
}
