package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"strikeye/internal/avatars"
	"strikeye/internal/storage"
)

// avatarsResult is the JSON summary printed by `avatars`.
type avatarsResult struct {
	Dir     string          `json:"dir"`
	Fetched []string        `json:"fetched"`
	Cached  []string        `json:"cached"`
	Failed  []avatarsFailed `json:"failed"`
}

type avatarsFailed struct {
	SteamID string `json:"steamid"`
	Error   string `json:"error"`
}

// runAvatars downloads Steam avatars for explicit SteamIDs or every player of a
// demo, caching them under <DataRoot>/cache/radar_cards/portraits.
func runAvatars(ctx context.Context, dataRoot string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("avatars", flag.ContinueOnError)
	fs.SetOutput(out)
	steamIDs := fs.String("steamid", "", "Comma separated SteamID64 list")
	demoID := fs.Int64("demo-id", 0, "Fetch avatars for every player of this demo")
	outDir := fs.String("out-dir", "", "Override the cache directory")
	refresh := fs.Bool("refresh", false, "Ignore cached avatars")
	maxAgeDays := fs.Int("max-age-days", 30, "Reuse cached avatars younger than this")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	var ids []string
	for _, raw := range strings.Split(*steamIDs, ",") {
		if id := strings.TrimSpace(raw); id != "" {
			ids = append(ids, id)
		}
	}
	if *demoID > 0 {
		players, err := db.Analysis().ListMatchPlayers(ctx, *demoID)
		if err != nil {
			return fmt.Errorf("list players: %w", err)
		}
		for _, p := range players {
			ids = append(ids, p.SteamID64)
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("provide --steamid or --demo-id")
	}

	dir := *outDir
	if dir == "" {
		dir = avatars.CacheDir(dataRoot)
	}
	maxAge := time.Duration(*maxAgeDays) * 24 * time.Hour
	if *refresh {
		maxAge = 0
	}

	client := avatars.NewClient()
	res := avatarsResult{Dir: dir, Fetched: []string{}, Cached: []string{}, Failed: []avatarsFailed{}}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		before, hadBefore := avatars.Path(dir, id)
		path, err := client.Fetch(ctx, id, dir, maxAge)
		if err != nil {
			res.Failed = append(res.Failed, avatarsFailed{SteamID: id, Error: err.Error()})
			continue
		}
		if hadBefore && before == path {
			res.Cached = append(res.Cached, path)
		} else {
			res.Fetched = append(res.Fetched, path)
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}
