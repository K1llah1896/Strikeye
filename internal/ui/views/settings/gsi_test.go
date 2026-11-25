package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"strikeye/internal/gsi"
)

func TestWriteGSIConfigFile(t *testing.T) {
	root := t.TempDir()
	path, err := writeGSIConfig(root, "127.0.0.1:3000", "tok")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if path != gsiConfigPath(root) {
		t.Fatalf("path = %q", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `"uri" "http://127.0.0.1:3000/"`) {
		t.Fatalf("uri missing: %s", text)
	}
	if !strings.Contains(text, `"token" "tok"`) {
		t.Fatalf("token missing: %s", text)
	}
	if _, err := writeGSIConfig(root, "nohostport", ""); err == nil {
		t.Fatal("expected an error for a listen address without a port")
	}
}

func TestLoadGSIStateFreshness(t *testing.T) {
	root := t.TempDir()
	if _, ok := loadGSIState(root, time.Now(), time.Minute); ok {
		t.Fatal("missing file must not be live")
	}

	state := gsi.State{Map: "de_dust2", Round: 3, TeamScoreCT: 2, TeamScoreT: 1, RoundPhase: "live", ReceivedAt: time.Now()}
	if err := writeJSON(gsiStatePath(root), state); err != nil {
		t.Fatalf("write state: %v", err)
	}
	got, ok := loadGSIState(root, time.Now(), time.Minute)
	if !ok || got.Map != "de_dust2" || got.Round != 3 {
		t.Fatalf("state = %+v ok=%v", got, ok)
	}
	// Older than the window: stale, not live.
	if _, ok := loadGSIState(root, time.Now().Add(2*time.Minute), time.Minute); ok {
		t.Fatal("stale state must not be live")
	}
}

func TestAlivePlayers(t *testing.T) {
	state := gsi.State{Players: []gsi.Player{{Health: 100}, {Health: 0}, {Health: 37}}}
	if got := alivePlayers(state); got != 2 {
		t.Fatalf("alive = %d", got)
	}
}

// writeJSON is a test helper for seeding the mirrored state file.
func writeJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
