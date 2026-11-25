package vac

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"strikeye/internal/storage"
)

func stubServer(t *testing.T, body any, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("steamids"); got != "76561198000000001,76561198000000002" {
			t.Errorf("steamids = %q", got)
		}
		if got := r.URL.Query().Get("key"); got != "test-key" {
			t.Errorf("key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func banResponse() map[string]any {
	return map[string]any{"players": []map[string]any{
		{
			"SteamId": "76561198000000001", "CommunityBanned": false, "VACBanned": true,
			"NumberOfVACBans": 1, "DaysSinceLastBan": 412, "NumberOfGameBans": 0, "EconomyBan": "none",
		},
		{
			"SteamId": "76561198000000002", "CommunityBanned": true, "VACBanned": false,
			"NumberOfVACBans": 0, "DaysSinceLastBan": 0, "NumberOfGameBans": 2, "EconomyBan": "probation",
		},
	}}
}

func TestFetchParsesAndLabelsRisk(t *testing.T) {
	srv := stubServer(t, banResponse(), http.StatusOK)
	c := NewClient("test-key")
	c.BaseURL = srv.URL

	bans, err := c.Fetch(context.Background(), []string{"76561198000000001", "76561198000000002"})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(bans) != 2 {
		t.Fatalf("bans = %d", len(bans))
	}
	if !bans[0].VACBanned || bans[0].DaysSince != 412 || bans[0].RiskLabel() != "VAC 封禁" {
		t.Fatalf("first = %+v", bans[0])
	}
	if bans[1].RiskLabel() != "社区封禁" || bans[1].GameBans != 2 {
		t.Fatalf("second = %+v", bans[1])
	}
	if !bans[0].Risk() || bans[0].Risk() == false {
		t.Fatal("risk flag inconsistent")
	}
	clean := Ban{SteamID: "1"}
	if clean.Risk() || clean.RiskLabel() != "无封禁记录" {
		t.Fatalf("clean = %+v", clean)
	}
}

func TestFetchWithoutKey(t *testing.T) {
	c := NewClient("")
	if _, err := c.Fetch(context.Background(), []string{"1"}); err == nil {
		t.Fatal("expected missing key error")
	}
}

func TestFetchHTTPError(t *testing.T) {
	srv := stubServer(t, map[string]any{"error": "forbidden"}, http.StatusForbidden)
	c := NewClient("test-key")
	c.BaseURL = srv.URL
	if _, err := c.Fetch(context.Background(), []string{"76561198000000001", "76561198000000002"}); err == nil {
		t.Fatal("expected http error")
	}
}

func TestServiceCachesAndRefreshes(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(banResponse())
	}))
	defer srv.Close()

	client := NewClient("test-key")
	client.BaseURL = srv.URL
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	svc := &Service{DB: db, Client: client, TTL: time.Hour, Now: func() time.Time { return now }}

	ids := []string{"76561198000000001", "76561198000000002"}
	first, err := svc.Lookup(context.Background(), ids, false)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	for _, r := range first {
		if r.Cache != "miss" && r.Cache != "refreshed" {
			t.Fatalf("first pass cache = %q", r.Cache)
		}
	}

	second, err := svc.Lookup(context.Background(), ids, false)
	if err != nil {
		t.Fatalf("second lookup: %v", err)
	}
	if calls != 1 {
		t.Fatalf("cache hit should not call steam, calls = %d", calls)
	}
	for _, r := range second {
		if r.Cache != "hit" {
			t.Fatalf("second pass cache = %q", r.Cache)
		}
	}

	now = now.Add(2 * time.Hour)
	if _, err := svc.Lookup(context.Background(), ids, false); err != nil {
		t.Fatalf("stale lookup: %v", err)
	}
	if calls != 2 {
		t.Fatalf("stale entry should refresh, calls = %d", calls)
	}

	stored, ok, err := db.PlayerBans().Get(context.Background(), ids[0])
	if err != nil || !ok {
		t.Fatalf("stored = %+v ok=%v err=%v", stored, ok, err)
	}
	if !stored.VACBanned || stored.DaysSinceLastBan != 412 {
		t.Fatalf("stored = %+v", stored)
	}
	banned, err := db.PlayerBans().BannedFor(context.Background(), ids)
	if err != nil {
		t.Fatalf("banned for: %v", err)
	}
	if len(banned) != 2 {
		t.Fatalf("banned set = %+v", banned)
	}
}

func TestServiceWithoutKeyReturnsCachedOrSkipped(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.PlayerBans().Upsert(context.Background(), storage.PlayerBans{
		SteamID64: "76561198000000001", VACBanned: true, NumberOfVACBans: 1,
		CheckedAt: time.Now().Add(-48 * time.Hour),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewService(db, "")
	res, err := svc.Lookup(context.Background(), []string{"76561198000000001", "76561198000000009"}, false)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if res[0].Cache != "hit" || !res[0].Ban.VACBanned {
		t.Fatalf("cached = %+v", res[0])
	}
	if res[1].Cache != "skipped" || res[1].Ban.Risk() {
		t.Fatalf("skipped = %+v", res[1])
	}
}
