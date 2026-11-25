// Package gsi ingests Counter-Strike 2 Game State Integration payloads so the
// tool can show live match state without touching the game process.
package gsi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Weapon is one entry of a player's weapon list.
type Weapon struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	State string `json:"state"`
	Ammo  int    `json:"ammo"`
}

// Player is one participant as reported by GSI.
type Player struct {
	SteamID  string   `json:"steam_id"`
	Name     string   `json:"name"`
	Team     string   `json:"team"`
	Side     string   `json:"side"`
	Health   int      `json:"health"`
	Armor    int      `json:"armor"`
	Helmet   bool     `json:"helmet"`
	Money    int      `json:"money"`
	Kills    int      `json:"kills"`
	Assists  int      `json:"assists"`
	Deaths   int      `json:"deaths"`
	MVPs     int      `json:"mvps"`
	Score    int      `json:"score"`
	Weapons  []Weapon `json:"weapons"`
	Position string   `json:"position"`
	Forward  string   `json:"forward"`
	Alive    bool     `json:"alive"`
}

// State is the flattened view of one GSI payload.
type State struct {
	Provider      string    `json:"provider"`
	Map           string    `json:"map"`
	Phase         string    `json:"phase"`
	RoundPhase    string    `json:"round_phase"`
	Round         int       `json:"round"`
	TeamScoreCT   int       `json:"team_score_ct"`
	TeamScoreT    int       `json:"team_score_t"`
	Bomb          string    `json:"bomb"`
	RoundTimeLeft float64   `json:"round_time_left"`
	Players       []Player  `json:"players"`
	ReceivedAt    time.Time `json:"received_at"`
}

// flexInt accepts either a JSON number or a quoted number, because GSI payloads
// mix both forms across fields and game versions.
type flexInt int

// UnmarshalJSON implements json.Unmarshaler.
func (f *flexInt) UnmarshalJSON(b []byte) error {
	v, err := parseFlex(string(b))
	if err != nil {
		return err
	}
	*f = flexInt(v)
	return nil
}

// flexFloat accepts either a JSON number or a quoted number.
type flexFloat float64

// UnmarshalJSON implements json.Unmarshaler.
func (f *flexFloat) UnmarshalJSON(b []byte) error {
	v, err := parseFlex(string(b))
	if err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}

// parseFlex parses a possibly quoted numeric literal, treating empty or
// non-numeric values as zero so one odd field cannot drop a whole payload.
func parseFlex(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return 0, nil
	}
	raw = strings.Trim(raw, `"`)
	if raw == "" {
		return 0, nil
	}
	var v float64
	if _, err := fmt.Sscanf(raw, "%g", &v); err != nil {
		return 0, nil
	}
	return v, nil
}

// payload mirrors the subset of the GSI JSON schema we care about.
type payload struct {
	Provider struct {
		Name string `json:"name"`
	} `json:"provider"`
	Map struct {
		Name   string `json:"name"`
		Phase  string `json:"phase"`
		Round  int    `json:"round"`
		TeamCT struct {
			Score int `json:"score"`
		} `json:"team_ct"`
		TeamT struct {
			Score int `json:"score"`
		} `json:"team_t"`
	} `json:"map"`
	Round struct {
		Phase       string    `json:"phase"`
		Bomb        string    `json:"bomb"`
		PhaseEndsIn flexFloat `json:"phase_ends_in"`
	} `json:"round"`
	Player struct {
		SteamID  string `json:"steamid"`
		Name     string `json:"name"`
		Team     string `json:"team"`
		Activity string `json:"activity"`
		State    struct {
			Health     int  `json:"health"`
			Armor      int  `json:"armor"`
			Helmet     bool `json:"helmet"`
			Money      int  `json:"money"`
			RoundKills int  `json:"round_kills"`
			RoundHS    int  `json:"round_killhs"`
			EquipVal   int  `json:"equip_value"`
		} `json:"state"`
		MatchStats struct {
			Kills   int `json:"kills"`
			Assists int `json:"assists"`
			Deaths  int `json:"deaths"`
			MVPs    int `json:"mvps"`
			Score   int `json:"score"`
		} `json:"match_stats"`
		Weapons map[string]struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			State string `json:"state"`
			Ammo  struct {
				Clip int `json:"clip"`
			} `json:"ammo"`
		} `json:"weapons"`
		Position string `json:"position"`
		Forward  string `json:"forward"`
	} `json:"player"`
	AllPlayers map[string]struct {
		Name       string `json:"name"`
		Team       string `json:"team"`
		Health     int    `json:"health"`
		Armor      int    `json:"armor"`
		Helmet     bool   `json:"helmet"`
		Money      int    `json:"money"`
		MatchStats struct {
			Kills   int `json:"kills"`
			Assists int `json:"assists"`
			Deaths  int `json:"deaths"`
			MVPs    int `json:"mvps"`
			Score   int `json:"score"`
		} `json:"match_stats"`
		Weapons map[string]struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			State string `json:"state"`
		} `json:"weapons"`
		Position string `json:"position"`
	} `json:"allplayers"`
}

// Parse decodes one GSI payload into a State.
func Parse(r io.Reader) (State, error) {
	var p payload
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return State{}, fmt.Errorf("decode gsi payload: %w", err)
	}
	st := State{
		Provider:      p.Provider.Name,
		Map:           p.Map.Name,
		Phase:         p.Map.Phase,
		RoundPhase:    p.Round.Phase,
		Round:         int(p.Map.Round),
		TeamScoreCT:   int(p.Map.TeamCT.Score),
		TeamScoreT:    int(p.Map.TeamT.Score),
		Bomb:          p.Round.Bomb,
		RoundTimeLeft: float64(p.Round.PhaseEndsIn),
		Players:       playersFrom(p),
	}
	return st, nil
}

// playersFrom prefers the observed player plus allplayers, falling back to the
// single "player" block when allplayers is absent.
func playersFrom(p payload) []Player {
	out := make([]Player, 0, len(p.AllPlayers)+1)
	seen := map[string]bool{}
	if p.Player.SteamID != "" {
		pl := Player{
			SteamID:  p.Player.SteamID,
			Name:     p.Player.Name,
			Team:     p.Player.Team,
			Health:   p.Player.State.Health,
			Armor:    p.Player.State.Armor,
			Helmet:   p.Player.State.Helmet,
			Money:    p.Player.State.Money,
			Kills:    p.Player.MatchStats.Kills,
			Assists:  p.Player.MatchStats.Assists,
			Deaths:   p.Player.MatchStats.Deaths,
			MVPs:     p.Player.MatchStats.MVPs,
			Score:    p.Player.MatchStats.Score,
			Position: p.Player.Position,
			Forward:  p.Player.Forward,
		}
		for _, w := range p.Player.Weapons {
			pl.Weapons = append(pl.Weapons, Weapon{Name: w.Name, Type: w.Type, State: w.State, Ammo: w.Ammo.Clip})
		}
		pl.Alive = pl.Health > 0
		pl.Side = sideOf(p.Player.Team)
		seen[p.Player.SteamID] = true
		out = append(out, pl)
	}
	for sid, ap := range p.AllPlayers {
		if seen[sid] {
			continue
		}
		pl := Player{
			SteamID:  sid,
			Name:     ap.Name,
			Team:     ap.Team,
			Health:   int(ap.Health),
			Armor:    int(ap.Armor),
			Helmet:   ap.Helmet,
			Money:    int(ap.Money),
			Kills:    int(ap.MatchStats.Kills),
			Assists:  int(ap.MatchStats.Assists),
			Deaths:   int(ap.MatchStats.Deaths),
			MVPs:     int(ap.MatchStats.MVPs),
			Score:    int(ap.MatchStats.Score),
			Position: ap.Position,
			Side:     sideOf(ap.Team),
			Alive:    ap.Health > 0,
		}
		for _, w := range ap.Weapons {
			pl.Weapons = append(pl.Weapons, Weapon{Name: w.Name, Type: w.Type, State: w.State})
		}
		out = append(out, pl)
	}
	return out
}

func sideOf(team string) string {
	switch strings.ToUpper(strings.TrimSpace(team)) {
	case "CT":
		return "CT"
	case "T":
		return "T"
	}
	return ""
}

// Store keeps the latest state and notifies subscribers.
type Store struct {
	mu       sync.RWMutex
	state    State
	updates  int
	subs     []chan State
	onUpdate func(State)
}

// NewStore creates an empty store.
func NewStore() *Store { return &Store{} }

// Apply records a decoded payload and fans it out.
func (s *Store) Apply(st State) {
	st.ReceivedAt = time.Now().UTC()
	s.mu.Lock()
	s.state = st
	s.updates++
	subs := append([]chan State(nil), s.subs...)
	cb := s.onUpdate
	s.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- st:
		default:
		}
	}
	if cb != nil {
		cb(st)
	}
}

// Latest returns the most recent state and how many payloads were applied.
func (s *Store) Latest() (State, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state, s.updates
}

// Subscribe returns a channel fed by every later payload.
func (s *Store) Subscribe() chan State {
	ch := make(chan State, 8)
	s.mu.Lock()
	s.subs = append(s.subs, ch)
	s.mu.Unlock()
	return ch
}

// OnUpdate registers a callback invoked after each apply.
func (s *Store) OnUpdate(fn func(State)) {
	s.mu.Lock()
	s.onUpdate = fn
	s.mu.Unlock()
}

// Handler accepts POSTed GSI payloads. An empty token disables auth.
func (s *Store) Handler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if token != "" && strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		st, err := Parse(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Apply(st)
		w.WriteHeader(http.StatusOK)
	})
}

// Config renders a gamestate_integration cfg pointing CS2 at uri.
func Config(uri, token string) string {
	auth := ""
	if token != "" {
		auth = fmt.Sprintf("\n        \"auth\"\n        {\n            \"token\" \"%s\"\n        }", token)
	}
	return fmt.Sprintf(`"Strikeye GSI Configuration"
{
    "uri" "%s"
    "timeout" "5.0"
    "buffer"  "0.1"
    "throttle" "0.1"
    "heartbeat" "10.0"
    "data"
    {
        "provider"            "1"
        "map"                 "1"
        "round"               "1"
        "player_id"           "1"
        "player_state"        "1"
        "player_weapons"      "1"
        "player_match_stats"  "1"
        "allplayers_id"       "1"
        "allplayers_state"    "1"
        "allplayers_match_stats" "1"
        "allplayers_weapons"  "1"
        "allplayers_position" "1"%s
    }
}
`, uri, auth)
}
