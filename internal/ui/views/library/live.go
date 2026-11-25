package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"strikeye/internal/gsi"
)

// liveFreshness is how old a mirrored GSI state may be before the banner is
// treated as stale.
const liveFreshness = 15 * time.Second

// liveStatePath returns the mirror written by `strikeye-cli gsi`.
func liveStatePath(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "gsi_latest.json")
}

// loadLiveState reads the mirrored GSI state and reports whether it is fresh.
func loadLiveState(dataRoot string, now time.Time, maxAge time.Duration) (gsi.State, bool) {
	path := liveStatePath(dataRoot)
	info, err := os.Stat(path)
	if err != nil {
		return gsi.State{}, false
	}
	if maxAge > 0 && now.Sub(info.ModTime()) > maxAge {
		return gsi.State{}, false
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return gsi.State{}, false
	}
	var st gsi.State
	if err := json.Unmarshal(body, &st); err != nil {
		return gsi.State{}, false
	}
	return st, true
}

// aliveCounts returns how many players are alive per side.
func aliveCounts(st gsi.State) (int, int) {
	ct, tv := 0, 0
	for _, p := range st.Players {
		if !p.Alive {
			continue
		}
		switch p.Side {
		case "CT":
			ct++
		case "T":
			tv++
		}
	}
	return ct, tv
}

// liveHeadline renders the one-line banner text.
func liveHeadline(st gsi.State) string {
	ct, tv := aliveCounts(st)
	phase := st.RoundPhase
	if phase == "" {
		phase = st.Phase
	}
	bomb := st.Bomb
	if bomb == "" || bomb == "carried" || bomb == "dropped" {
		bomb = ""
	}
	head := st.Map
	if st.Round > 0 {
		head += " · 回合 " + itoa(st.Round)
	}
	head += " · " + itoa(st.TeamScoreCT) + "-" + itoa(st.TeamScoreT)
	if phase != "" {
		head += " · " + phase
	}
	if bomb != "" {
		head += " · 炸弹 " + bomb
	}
	head += " · 存活 " + itoa(ct) + "v" + itoa(tv)
	return head
}

// itoa is a tiny int formatter to avoid pulling strconv into the call sites.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// refreshLiveState re-reads the GSI mirror at most every two seconds.
func (v *View) refreshLiveState() {
	now := time.Now()
	if now.Sub(v.liveChecked) < 2*time.Second {
		return
	}
	v.liveChecked = now
	st, ok := loadLiveState(v.dataRoot, now, liveFreshness)
	v.live = st
	v.liveOK = ok
}
