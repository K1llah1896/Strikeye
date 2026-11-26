package cli

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"strikeye/internal/gsi"
)

const gsiPayload = `{"provider":{"name":"Counter-Strike: Global Offensive"},
 "map":{"name":"de_mirage","phase":"live","round":4,"team_ct":{"score":2},"team_t":{"score":1}},
 "round":{"phase":"live","bomb":"carried","phase_ends_in":"45.0"},
 "player":{"steamid":"76561198000000001","name":"ZywOo","team":"CT","state":{"health":100,"armor":100,"helmet":true,"money":800},
   "match_stats":{"kills":7,"assists":2,"deaths":3,"mvps":1,"score":14}},
 "allplayers":{"76561198000000002":{"name":"apEX","team":"CT","health":42,"money":150,"match_stats":{"kills":3,"deaths":5}}}}`

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// waitForListen polls until addr accepts TCP connections.
func waitForListen(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("listener never came up on %s: %v", addr, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRunGSIWritesConfigAndState(t *testing.T) {
	dataRoot := t.TempDir()
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- runGSI(ctx, dataRoot, gsiOptions{listen: addr, token: "tok"}, &out)
	}()

	configPath := filepath.Join(dataRoot, "cache", "gsi", "gamestate_integration_strikeye.cfg")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(configPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("config not written: %s (out=%s)", configPath, out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(cfg), "http://"+addr+"/") || !strings.Contains(string(cfg), `"token" "tok"`) {
		t.Fatalf("config = %s", cfg)
	}

	// The config file is written before the listener accepts: wait for the port
	// to answer so the POST does not race the bind.
	waitForListen(t, addr)

	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/", strings.NewReader(gsiPayload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post: %v (out=%s)", err, out.String())
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	latestPath := filepath.Join(dataRoot, "cache", "gsi_latest.json")
	deadline = time.Now().Add(5 * time.Second)
	for {
		body, err := os.ReadFile(latestPath)
		if err == nil && bytes.Contains(body, []byte(`"de_mirage"`)) {
			if !bytes.Contains(body, []byte(`"ZywOo"`)) || !bytes.Contains(body, []byte(`"apEX"`)) {
				t.Fatalf("state missing players: %s", body)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("state not mirrored (out=%s)", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runGSI returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runGSI did not stop")
	}
	if !strings.Contains(out.String(), `"map":"de_mirage"`) {
		t.Fatalf("summary line missing: %s", out.String())
	}
}

func TestRunGSIOnceTimesOut(t *testing.T) {
	dataRoot := t.TempDir()
	addr := freeAddr(t)
	var out bytes.Buffer
	err := runGSI(context.Background(), dataRoot, gsiOptions{listen: addr, once: true, timeout: 150 * time.Millisecond}, &out)
	if err == nil || !strings.Contains(err.Error(), "no gsi payload") {
		t.Fatalf("err = %v out=%s", err, out.String())
	}
}

func TestNormalizeListen(t *testing.T) {
	cases := map[string]string{
		":32123":          "127.0.0.1:32123",
		"0.0.0.0:8080":    "127.0.0.1:8080",
		"192.168.1.5:900": "192.168.1.5:900",
		"garbage":         "garbage",
	}
	for in, want := range cases {
		if got := normalizeListen(in); got != want {
			t.Fatalf("normalizeListen(%q) = %q want %q", in, got, want)
		}
	}
}

func TestGSISummaryCountsAlive(t *testing.T) {
	st, err := parsePayloadForTest()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sum := gsiSummary(st)
	if sum["alive_ct"] != 2 || sum["alive_t"] != 0 {
		t.Fatalf("summary = %v", sum)
	}
	if sum["score"] != "2-1" {
		t.Fatalf("score = %v", sum["score"])
	}
	if fmt.Sprint(sum["map"]) != "de_mirage" {
		t.Fatalf("map = %v", sum["map"])
	}
}

func parsePayloadForTest() (gsi.State, error) {
	return gsi.Parse(strings.NewReader(gsiPayload))
}
