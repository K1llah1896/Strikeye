package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"strikeye/internal/storage"
	"strikeye/internal/vac"
)

// steamAPIKey resolves the Steam Web API key from the flag or the environment.
func steamAPIKey(flagValue string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("STRIKEYE_STEAM_API_KEY"))
}

// runBans prints cached/refreshed ban state for the requested accounts. When no
// explicit IDs are given it covers every player in one demo, which is what the
// "is this opponent clean" question usually starts from.
func runBans(ctx context.Context, dataRoot string, demoID int64, steamIDs []string, refresh bool, apiKey string, out io.Writer) error {
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	ids := make([]string, 0, len(steamIDs))
	for _, id := range steamIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		demo, err := pickDemo(ctx, db, demoID)
		if err != nil {
			return err
		}
		players, err := db.Analysis().ListMatchPlayers(ctx, demo.ID)
		if err != nil {
			return err
		}
		for _, p := range players {
			if p.SteamID64 != "" {
				ids = append(ids, p.SteamID64)
			}
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("no steam ids to check")
	}

	svc := vac.NewService(db, apiKey)
	results, err := svc.Lookup(ctx, ids, refresh)
	if err != nil {
		return err
	}

	type output struct {
		Checked   int          `json:"checked"`
		FromCache int          `json:"from_cache"`
		Skipped   int          `json:"skipped"`
		AtRisk    int          `json:"at_risk"`
		HasKey    bool         `json:"has_steam_key"`
		Bans      []vac.Result `json:"bans"`
	}
	payload := output{Checked: len(results), HasKey: apiKey != "", Bans: results}
	for _, r := range results {
		switch r.Cache {
		case "hit":
			payload.FromCache++
		case "skipped":
			payload.Skipped++
		}
		if r.Ban.Risk() {
			payload.AtRisk++
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
