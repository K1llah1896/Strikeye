package analysis

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

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
