// Package vac checks Steam bans for the accounts that appear in a match library.
//
// Steam only exposes ban state through ISteamUser/GetPlayerBans, which needs an
// API key, so this package never guesses: without a key it reports that the
// check was skipped and leaves cached snapshots untouched. Snapshots are stored
// in SQLite with a checked_at timestamp so the UI can show how fresh a verdict
// is.
package vac

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"strikeye/internal/storage"
)

// DefaultBaseURL is the Steam Web API root.
const DefaultBaseURL = "https://api.steampowered.com"

// DefaultCacheTTL keeps a snapshot for one day: bans change slowly and the Web
// API is rate limited.
const DefaultCacheTTL = 24 * time.Hour

// Client talks to the Steam Web API.
type Client struct {
	HTTP    *http.Client
	APIKey  string
	BaseURL string
}

// NewClient returns a client using the given key.
func NewClient(apiKey string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, APIKey: apiKey, BaseURL: DefaultBaseURL}
}

// Ban is one account's ban state.
type Ban struct {
	SteamID    string `json:"steam_id64"`
	VACBanned  bool   `json:"vac_banned"`
	VACCount   int    `json:"number_of_vac_bans"`
	GameBans   int    `json:"number_of_game_bans"`
	Community  bool   `json:"community_banned"`
	DaysSince  int    `json:"days_since_last_ban"`
	EconomyBan string `json:"economy_ban"`
}

// RiskLabel is a short verdict for UI badges.
func (b Ban) RiskLabel() string {
	switch {
	case b.VACBanned:
		return "VAC 封禁"
	case b.Community:
		return "社区封禁"
	case b.GameBans > 0:
		return "游戏封禁"
	default:
		return "无封禁记录"
	}
}

// Risk reports whether the account carries any ban.
func (b Ban) Risk() bool {
	return b.VACBanned || b.Community || b.GameBans > 0
}

type apiResponse struct {
	Players []struct {
		SteamID          string `json:"SteamId"`
		CommunityBanned  bool   `json:"CommunityBanned"`
		VACBanned        bool   `json:"VACBanned"`
		NumberOfVACBans  int    `json:"NumberOfVACBans"`
		DaysSinceLastBan int    `json:"DaysSinceLastBan"`
		NumberOfGameBans int    `json:"NumberOfGameBans"`
		EconomyBan       string `json:"EconomyBan"`
	} `json:"players"`
}

// Fetch asks Steam for the ban state of every given account.
func (c *Client) Fetch(ctx context.Context, steamIDs []string) ([]Ban, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("steam api key missing")
	}
	ids := make([]string, 0, len(steamIDs))
	for _, id := range steamIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	q := url.Values{}
	q.Set("key", c.APIKey)
	q.Set("steamids", strings.Join(ids, ","))
	endpoint := base + "/ISteamUser/GetPlayerBans/v1/?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam api %s: %s", resp.Status, truncate(string(body), 160))
	}
	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode steam bans: %w", err)
	}
	out := make([]Ban, 0, len(parsed.Players))
	for _, p := range parsed.Players {
		out = append(out, Ban{
			SteamID:    p.SteamID,
			VACBanned:  p.VACBanned,
			VACCount:   p.NumberOfVACBans,
			GameBans:   p.NumberOfGameBans,
			Community:  p.CommunityBanned,
			DaysSince:  p.DaysSinceLastBan,
			EconomyBan: p.EconomyBan,
		})
	}
	return out, nil
}

// Service caches Steam responses in SQLite.
type Service struct {
	DB     *storage.DB
	Client *Client
	TTL    time.Duration
	Now    func() time.Time
}

// NewService wires a service with a one-day cache.
func NewService(db *storage.DB, apiKey string) *Service {
	return &Service{DB: db, Client: NewClient(apiKey), TTL: DefaultCacheTTL, Now: time.Now}
}

// Result is one lookup outcome.
type Result struct {
	Ban   Ban    `json:"ban"`
	Cache string `json:"cache"` // "hit", "miss" or "skipped"
}

// Lookup returns the ban state for steamIDs, refreshing entries older than TTL
// when a key is configured. Without a key it returns cached rows only and marks
// uncached entries as skipped.
func (s *Service) Lookup(ctx context.Context, steamIDs []string, refresh bool) ([]Result, error) {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	repo := s.DB.PlayerBans()
	out := make([]Result, 0, len(steamIDs))
	fetchIDs := make([]string, 0, len(steamIDs))
	index := make(map[string]int, len(steamIDs))
	for _, id := range steamIDs {
		cached, ok, err := repo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		fresh := ok && s.TTL > 0 && now().Sub(cached.CheckedAt) < s.TTL
		if ok && fresh && !refresh {
			out = append(out, Result{Ban: fromStorage(cached), Cache: "hit"})
			continue
		}
		if s.Client == nil || s.Client.APIKey == "" {
			if ok {
				out = append(out, Result{Ban: fromStorage(cached), Cache: "hit"})
			} else {
				out = append(out, Result{Ban: Ban{SteamID: id}, Cache: "skipped"})
			}
			continue
		}
		index[id] = len(out)
		out = append(out, Result{Ban: Ban{SteamID: id}, Cache: "miss"})
		fetchIDs = append(fetchIDs, id)
	}
	if len(fetchIDs) > 0 {
		bans, err := s.Client.Fetch(ctx, fetchIDs)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, b := range bans {
			seen[b.SteamID] = true
			if i, ok := index[b.SteamID]; ok {
				out[i] = Result{Ban: b, Cache: "refreshed"}
			}
			if err := repo.Upsert(ctx, toStorage(b, now())); err != nil {
				return nil, err
			}
		}
		for _, id := range fetchIDs {
			if seen[id] {
				continue
			}
			if i, ok := index[id]; ok {
				out[i] = Result{Ban: Ban{SteamID: id}, Cache: "skipped"}
			}
		}
	}
	return out, nil
}

func toStorage(b Ban, at time.Time) storage.PlayerBans {
	return storage.PlayerBans{
		SteamID64:        b.SteamID,
		VACBanned:        b.VACBanned,
		NumberOfVACBans:  b.VACCount,
		NumberOfGameBans: b.GameBans,
		CommunityBanned:  b.Community,
		DaysSinceLastBan: b.DaysSince,
		EconomyBan:       b.EconomyBan,
		CheckedAt:        at,
	}
}

func fromStorage(p storage.PlayerBans) Ban {
	return Ban{
		SteamID:    p.SteamID64,
		VACBanned:  p.VACBanned,
		VACCount:   p.NumberOfVACBans,
		GameBans:   p.NumberOfGameBans,
		Community:  p.CommunityBanned,
		DaysSince:  p.DaysSinceLastBan,
		EconomyBan: p.EconomyBan,
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
