package gsi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixture = `{
  "provider": {"name": "Counter-Strike: Global Offensive", "appid": 730},
  "map": {"name": "de_dust2", "phase": "live", "round": 7,
          "team_ct": {"score": 3}, "team_t": {"score": 3}},
  "round": {"phase": "live", "bomb": "planted", "phase_ends_in": "21.4"},
  "player": {"steamid": "76561198000000001", "name": "NiKo", "team": "CT",
    "state": {"health": 87, "armor": 100, "helmet": true, "money": 2350},
    "match_stats": {"kills": 12, "assists": 3, "deaths": 6, "mvps": 2, "score": 24},
    "weapons": {"weapon_0": {"name": "weapon_ak47", "type": "Rifle", "state": "active",
                             "ammo": {"clip": 27, "reserve": 90}}},
    "position": "1200.00, 800.00, -60.00", "forward": "0.70, 0.70, 0.00"},
  "allplayers": {
    "76561198000000002": {"name": "karrigan", "team": "T", "health": 100, "armor": 0,
      "match_stats": {"kills": 5, "assists": 1, "deaths": 9, "mvps": 0, "score": 11},
      "weapons": {"weapon_0": {"name": "weapon_glock", "type": "Pistol", "state": "active"}},
      "position": "-900.00, 1500.00, -80.00"},
    "76561198000000003": {"name": "m0NESY", "team": "CT", "health": 0, "armor": 0,
      "match_stats": {"kills": 9, "assists": 0, "deaths": 8, "mvps": 1, "score": 19},
      "weapons": {}}
  }
}`

func TestParsePayload(t *testing.T) {
	st, err := Parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if st.Map != "de_dust2" || st.Round != 7 || st.TeamScoreCT != 3 || st.TeamScoreT != 3 {
		t.Fatalf("state = %+v", st)
	}
	if st.RoundPhase != "live" || st.Bomb != "planted" || st.RoundTimeLeft != 21.4 {
		t.Fatalf("round = %+v", st)
	}
	if len(st.Players) != 3 {
		t.Fatalf("players = %+v", st.Players)
	}
	var niko *Player
	for i := range st.Players {
		if st.Players[i].Name == "NiKo" {
			niko = &st.Players[i]
		}
	}
	if niko == nil {
		t.Fatal("NiKo missing")
	}
	if niko.Side != "CT" || !niko.Alive || niko.Kills != 12 || niko.Money != 2350 {
		t.Fatalf("niko = %+v", niko)
	}
	if len(niko.Weapons) != 1 || niko.Weapons[0].Name != "weapon_ak47" || niko.Weapons[0].Ammo != 27 {
		t.Fatalf("weapons = %+v", niko.Weapons)
	}
	for _, p := range st.Players {
		if p.Name == "m0NESY" && p.Alive {
			t.Fatalf("dead player flagged alive: %+v", p)
		}
		if p.Name == "karrigan" && p.Side != "T" {
			t.Fatalf("karrigan side = %q", p.Side)
		}
	}
}

func TestHandlerStoresAndAuthorises(t *testing.T) {
	store := NewStore()
	h := store.Handler("secret")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fixture)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorised status = %d", rec.Code)
	}
	if _, n := store.Latest(); n != 0 {
		t.Fatalf("payload accepted without token")
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fixture))
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	st, n := store.Latest()
	if n != 1 || st.Map != "de_dust2" || st.ReceivedAt.IsZero() {
		t.Fatalf("stored = %+v n=%d", st, n)
	}

	// Updates fan out to subscribers and callbacks.
	ch := store.Subscribe()
	got := 0
	store.OnUpdate(func(s State) { got++ })
	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fixture))
	req2.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(httptest.NewRecorder(), req2)
	select {
	case <-ch:
	default:
		t.Fatal("subscriber not notified")
	}
	if got != 1 {
		t.Fatalf("callback count = %d", got)
	}

	// Wrong method and broken payload are rejected.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", rec.Code)
	}
	bad := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{not json"))
	bad.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, bad)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad payload status = %d", rec.Code)
	}
}

func TestConfigRendering(t *testing.T) {
	plain := Config("http://127.0.0.1:32123/", "")
	// The game expects Valve KeyValues, so the config is checked structurally
	// rather than as JSON.
	for _, want := range []string{
		`"uri" "http://127.0.0.1:32123/"`,
		`"player_weapons"      "1"`,
		`"allplayers_position" "1"`,
		`"heartbeat" "10.0"`,
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("config missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, `"auth"`) {
		t.Fatalf("auth block should be omitted without a token:\n%s", plain)
	}

	withToken := Config("http://127.0.0.1:32123/", "abc")
	if !strings.Contains(withToken, `"token" "abc"`) {
		t.Fatalf("token missing:\n%s", withToken)
	}
	if strings.Count(withToken, "{") != strings.Count(withToken, "}") {
		t.Fatalf("unbalanced braces:\n%s", withToken)
	}
}
