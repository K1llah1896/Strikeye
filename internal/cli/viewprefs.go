package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"strikeye/internal/storage"
)

// viewPrefsOptions drives the `viewprefs` subcommand.
type viewPrefsOptions struct {
	mapName string
	offsetX float64
	offsetY float64
	opacity float64
	set     bool
}

// runViewPrefs lists stored per-map 2D alignments, or writes one when --set is
// given together with a map name.
func runViewPrefs(ctx context.Context, db *storage.DB, opts viewPrefsOptions, out io.Writer) error {
	if db == nil {
		return fmt.Errorf("storage unavailable")
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")

	if opts.set {
		if strings.TrimSpace(opts.mapName) == "" {
			return fmt.Errorf("--map is required with --set")
		}
		prefs := storage.MapViewPrefs{
			MapName:      strings.TrimSpace(opts.mapName),
			LowerOffsetX: opts.offsetX,
			LowerOffsetY: opts.offsetY,
			LowerOpacity: opts.opacity,
		}
		if prefs.LowerOpacity <= 0 {
			prefs.LowerOpacity = 1
		}
		if err := db.MapViewPrefs().Upsert(ctx, prefs); err != nil {
			return err
		}
		stored, err := db.MapViewPrefs().Get(ctx, prefs.MapName)
		if err != nil {
			return err
		}
		return enc.Encode(map[string]any{"saved": true, "prefs": stored})
	}

	all, err := db.MapViewPrefs().List(ctx)
	if err != nil {
		return err
	}
	return enc.Encode(map[string]any{"prefs": all, "count": len(all)})
}
