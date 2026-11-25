package export

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"strikeye/internal/storage"
)

// BatchFilter narrows which matches a batch export covers. Zero values mean
// "no constraint"; ParsedOnly defaults to true via Normalize.
type BatchFilter struct {
	Team       string
	Map        string
	Limit      int
	ParsedOnly bool
	normalized bool
}

// Normalize trims and lowercases the textual filters and applies defaults.
func (f BatchFilter) Normalize() BatchFilter {
	f.Team = strings.TrimSpace(f.Team)
	f.Map = strings.TrimSpace(f.Map)
	if f.Limit < 0 {
		f.Limit = 0
	}
	f.ParsedOnly = true
	return f
}

// BatchItem records one match of a batch export.
type BatchItem struct {
	DemoID int64    `json:"demo_id"`
	TeamA  string   `json:"team_a"`
	TeamB  string   `json:"team_b"`
	Map    string   `json:"map"`
	Dir    string   `json:"dir"`
	Files  []string `json:"files,omitempty"`
	Err    string   `json:"error,omitempty"`
}

// BatchResult is the summary of a batch export.
type BatchResult struct {
	Dir      string      `json:"dir"`
	Format   string      `json:"format"`
	Filter   string      `json:"filter"`
	Exported int         `json:"exported"`
	Failed   int         `json:"failed"`
	Items    []BatchItem `json:"items"`
	Index    string      `json:"index"`
}

func matchFilter(demo storage.Demo, f BatchFilter) bool {
	if f.ParsedOnly && demo.ParseStatus != "parsed" {
		return false
	}
	if f.Map != "" {
		if demo.MapName == nil || !strings.EqualFold(*demo.MapName, f.Map) {
			return false
		}
	}
	if f.Team != "" {
		if !strings.EqualFold(demo.TeamAName, f.Team) && !strings.EqualFold(demo.TeamBName, f.Team) {
			return false
		}
	}
	return true
}

// Batch writes one export directory per matching match plus an index file.
// Each match lands in <outDir>/demo_<id>/ so files never collide.
func Batch(ctx context.Context, db *storage.DB, outDir string, formats Formats, filter BatchFilter, formatLabel string) (BatchResult, error) {
	filter = filter.Normalize()
	if outDir == "" {
		return BatchResult{}, fmt.Errorf("output directory is required")
	}
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return BatchResult{}, err
	}
	sort.Slice(demos, func(i, j int) bool { return demos[i].ID < demos[j].ID })
	res := BatchResult{Dir: outDir, Format: formatLabel, Filter: describeFilter(filter)}
	for _, demo := range demos {
		if !matchFilter(demo, filter) {
			continue
		}
		if filter.Limit > 0 && len(res.Items) >= filter.Limit {
			break
		}
		item := BatchItem{DemoID: demo.ID, TeamA: demo.TeamAName, TeamB: demo.TeamBName}
		if demo.MapName != nil {
			item.Map = *demo.MapName
		}
		item.Dir = filepath.Join(outDir, fmt.Sprintf("demo_%d", demo.ID))
		bundle, err := LoadMatchBundle(ctx, db, demo.ID)
		if err != nil {
			item.Err = err.Error()
			res.Failed++
			res.Items = append(res.Items, item)
			continue
		}
		files, err := WriteMatchBundle(bundle, item.Dir, formats)
		if err != nil {
			item.Err = err.Error()
			res.Failed++
			res.Items = append(res.Items, item)
			continue
		}
		item.Files = files
		res.Exported++
		res.Items = append(res.Items, item)
	}
	if len(res.Items) == 0 {
		return res, fmt.Errorf("no matches matched filter %s", res.Filter)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return res, fmt.Errorf("create export dir: %w", err)
	}
	res.Index = filepath.Join(outDir, "export_index.json")
	if err := writeBatchIndex(res.Index, res); err != nil {
		return res, err
	}
	if res.Exported == 0 {
		return res, fmt.Errorf("all %d matches failed to export (first error: %s)", res.Failed, res.Items[0].Err)
	}
	return res, nil
}

func describeFilter(f BatchFilter) string {
	parts := []string{"status=parsed"}
	if f.Team != "" {
		parts = append(parts, "team="+f.Team)
	}
	if f.Map != "" {
		parts = append(parts, "map="+f.Map)
	}
	if f.Limit > 0 {
		parts = append(parts, fmt.Sprintf("limit=%d", f.Limit))
	}
	return strings.Join(parts, ",")
}

func writeBatchIndex(path string, res BatchResult) error {
	blob, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	blob = append(blob, '\n')
	tmp := path + ".part"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return fmt.Errorf("write export index: %w", err)
	}
	return os.Rename(tmp, path)
}
