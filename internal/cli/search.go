package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"strikeye/internal/storage"
)

// runSearch queries the full-text library index (players, teams, map, note).
func runSearch(ctx context.Context, dataRoot string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(out)
	query := fs.String("query", "", "Search text (players / teams / map / notes)")
	limit := fs.Int("limit", 20, "Maximum hits")
	reindex := fs.Bool("reindex", false, "Force a full index rebuild")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*query) == "" {
		return fmt.Errorf("--query is required")
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	repo := db.Search()
	rebuilt := 0
	if *reindex {
		n, err := repo.RebuildSearchIndex(ctx)
		if err != nil {
			return err
		}
		rebuilt = n
	} else {
		changed, err := repo.EnsureSearchIndex(ctx)
		if err != nil {
			return err
		}
		if changed {
			var indexed int
			if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM match_search`).Scan(&indexed); err == nil {
				rebuilt = indexed
			}
		}
	}
	hits, err := repo.Query(ctx, *query, *limit)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"query":         *query,
		"hits":          hits,
		"count":         len(hits),
		"index_rebuilt": rebuilt > 0,
		"indexed":       rebuilt,
	})
}
