package analysis

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"strikeye/internal/avatars"
	"strikeye/internal/storage"
)

func TestRefreshAvatarsSkipsWithoutPlayers(t *testing.T) {
	v := &View{dataRoot: t.TempDir()}
	v.refreshAvatars()
	v.mu.RLock()
	busy := v.avatarBusy
	v.mu.RUnlock()
	if busy {
		t.Fatal("refresh must not start without players")
	}
}

// TestRefreshAvatarsDownloadsAndReloads drives the background path against a fake
// Steam profile XML and asserts the bitmaps land in the view.
func TestRefreshAvatarsDownloadsAndReloads(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot resolve test file path")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	pngBytes, err := os.ReadFile(filepath.Join(repoRoot, "assets", "maps", "de_dust2.png"))
	if err != nil {
		t.Skipf("no radar asset to reuse as an avatar: %v", err)
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/profile":
			_, _ = w.Write([]byte(`<profile><avatarFull><![CDATA[` + srv.URL + `/avatar.png]]></avatarFull></profile>`))
		case r.URL.Path == "/avatar.png":
			_, _ = w.Write(pngBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dataRoot := t.TempDir()
	players := []storage.MatchPlayer{{SteamID64: "76561198000000001", PlayerName: "KSC"}}
	// Seed the cache through the avatars client with the fake profile endpoint so
	// the view's loader has something to decode.
	client := &avatars.Client{ProfileURL: srv.URL + "/profile?steamid=%s"}
	if _, err := client.Fetch(t.Context(), players[0].SteamID64, avatars.CacheDir(dataRoot), 0); err != nil {
		t.Fatalf("seed avatar: %v", err)
	}

	v := &View{dataRoot: dataRoot, players: players, avatars: loadAvatars(dataRoot, players)}
	if _, ok := v.avatarFor(players[0].SteamID64); !ok {
		t.Fatal("seeded avatar not decoded")
	}
}

// TestEnsureAvatarsAsyncFetchesMissing covers the zero-click path: opening a demo
// with an empty avatar cache must fetch the portrait on its own.
func TestEnsureAvatarsAsyncFetchesMissing(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot resolve test file path")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	pngBytes, err := os.ReadFile(filepath.Join(repoRoot, "assets", "maps", "de_dust2.png"))
	if err != nil {
		t.Skipf("no image asset to reuse as an avatar: %v", err)
	}
	hits := make(chan string, 4)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/profile":
			hits <- r.URL.Query().Get("steamid")
			_, _ = w.Write([]byte(`<profile><avatarFull><![CDATA[` + srv.URL + `/avatar.png]]></avatarFull></profile>`))
		case "/avatar.png":
			_, _ = w.Write(pngBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dataRoot := t.TempDir()
	sid := "76561198000000042"
	players := []storage.MatchPlayer{{SteamID64: sid, PlayerName: "KSC"}}
	v := &View{
		dataRoot:        dataRoot,
		players:         players,
		avatarAutoFetch: true,
		avatarClient:    &avatars.Client{ProfileURL: srv.URL + "/profile?steamid=%s"},
	}
	v.ensureAvatarsAsync()

	select {
	case got := <-hits:
		if got != sid {
			t.Fatalf("fetched %q, want %q", got, sid)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no avatar fetch was attempted")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := v.avatarFor(sid); ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("avatar never decoded into the view")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestEnsureAvatarsAsyncRespectsDisabled keeps render tests hermetic.
func TestEnsureAvatarsAsyncRespectsDisabled(t *testing.T) {
	v := &View{
		dataRoot: t.TempDir(),
		players:  []storage.MatchPlayer{{SteamID64: "76561198000000042"}},
		// avatarAutoFetch left false
	}
	v.ensureAvatarsAsync()
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.avatarBusy {
		t.Fatal("auto fetch must not start when disabled")
	}
}
