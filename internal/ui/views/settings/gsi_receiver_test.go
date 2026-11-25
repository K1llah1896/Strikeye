package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/gsi"
	"strikeye/internal/storage"
	"strikeye/internal/ui/renderutil"
)

// TestGSIReceiverServesAndMirrors starts the in-process listener on an ephemeral
// port, posts a state and checks both the mirrored file and the store.
func TestGSIReceiverServesAndMirrors(t *testing.T) {
	dataRoot := t.TempDir()
	recv := &gsiReceiver{}
	addr, err := recv.start("127.0.0.1:0", "tok", dataRoot)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = recv.stop() })
	if _, running := recv.running(); !running {
		t.Fatal("receiver not reported as running")
	}

	payload := `{"map":{"name":"de_dust2","phase":"live","round":4},
		"round":{"phase":"live","bomb":"carried"},
		"player":{"state":{"health":87}},
		"allplayers":{"76561198000000001":{"name":"KSCERATO","state":{"health":87,"armor":100}}}}`
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	deadline := time.Now().Add(2 * time.Second)
	path := gsiStatePath(dataRoot)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mirror: %v", err)
	}
	var st gsi.State
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if st.Map != "de_dust2" || st.Round != 4 || len(st.Players) != 1 {
		t.Fatalf("mirrored state = %+v", st)
	}

	// A second start on the same receiver reports the running address.
	if again, err := recv.start("127.0.0.1:0", "", dataRoot); err == nil || again != addr {
		t.Fatalf("restart = %q err=%v", again, err)
	}
	if err := recv.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, running := recv.running(); running {
		t.Fatal("receiver still running after stop")
	}
}

// TestGSIReceiverRejectsBadAddress covers the listen error path.
func TestGSIReceiverRejectsBadAddress(t *testing.T) {
	recv := &gsiReceiver{}
	if _, err := recv.start("127.0.0.1:99999", "", t.TempDir()); err == nil {
		t.Fatal("expected listen error")
	}
}

// TestRenderGSICardShowsReceiverState renders the card with the listener running.
func TestRenderGSICardShowsReceiverState(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	dataRoot := t.TempDir()
	v := NewView(db, nil, dataRoot)
	addr, err := v.receiverOf().start("127.0.0.1:0", "", dataRoot)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer v.Close()
	if addr == "" {
		t.Fatal("empty addr")
	}
	cfg, err := writeGSIConfig(dataRoot, addr, "")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("config file: %v", err)
	}
	if err := writeGSIStateMirror(gsiStatePath(dataRoot), gsi.State{Map: "de_ancient", Round: 2, ReceivedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("mirror: %v", err)
	}
	if _, ok := loadGSIState(dataRoot, time.Now(), 30*time.Second); !ok {
		t.Fatal("state not fresh")
	}
	renderutil.AssertRendered(t, 40, func(c *ui.Context) { v.Render(c) })
	_ = context.Background
}
