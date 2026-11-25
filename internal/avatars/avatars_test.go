package avatars

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testSteamID = "76561198058500492"

func testServer(t *testing.T, avatarPath string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/"+testSteamID+"/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0"?><profile><steamID64>%s</steamID64><avatarFull><![CDATA[http://%s%s]]></avatarFull></profile>`,
			testSteamID, r.Host, avatarPath)
	})
	mux.HandleFunc("/profiles/76561198000000000/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><profile><steamID64>76561198000000000</steamID64><avatarFull><![CDATA[]]></avatarFull></profile>`)
	})
	mux.HandleFunc(avatarPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("\x89PNG fake bytes"))
	})
	return httptest.NewServer(mux)
}

func newTestClient(url string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 5 * time.Second}, ProfileURL: url + "/profiles/%s/"}
}

func TestFetchDownloadsAndReuses(t *testing.T) {
	srv := testServer(t, "/img/avatar.png")
	defer srv.Close()
	dir := t.TempDir()
	c := newTestClient(srv.URL)

	path, err := c.Fetch(context.Background(), testSteamID, dir, DefaultMaxAge)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if filepath.Base(path) != testSteamID+".png" {
		t.Fatalf("path = %s", path)
	}
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 {
		t.Fatalf("read cached avatar: %v", err)
	}

	// A fresh cache entry must not hit the network: shut the server down and
	// fetch again.
	srv.Close()
	again, err := c.Fetch(context.Background(), testSteamID, dir, DefaultMaxAge)
	if err != nil || again != path {
		t.Fatalf("reuse failed: %v %s", err, again)
	}

	// Stale entries are refetched (here: fails, because the server is gone).
	if _, err := c.Fetch(context.Background(), testSteamID, dir, time.Nanosecond); err == nil {
		t.Fatal("expected a refetch failure after the server stopped")
	}
}

func TestFetchRejectsBadSteamID(t *testing.T) {
	c := newTestClient("http://127.0.0.1:1")
	for _, id := range []string{"", "abc", "7656119805850049", "765611980585004921"} {
		if _, err := c.Fetch(context.Background(), id, t.TempDir(), time.Hour); !errors.Is(err, ErrInvalidSteamID) {
			t.Fatalf("%q: err = %v, want ErrInvalidSteamID", id, err)
		}
	}
}

func TestFetchPrivateProfile(t *testing.T) {
	srv := testServer(t, "/img/avatar.png")
	defer srv.Close()
	c := newTestClient(srv.URL)
	if _, err := c.Fetch(context.Background(), "76561198000000000", t.TempDir(), time.Hour); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestFetchJpegExtension(t *testing.T) {
	srv := testServer(t, "/img/avatar.jpg")
	defer srv.Close()
	c := newTestClient(srv.URL)
	path, err := c.Fetch(context.Background(), testSteamID, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if filepath.Ext(path) != ".jpg" {
		t.Fatalf("extension = %s", filepath.Ext(path))
	}
	if _, ok := Path(filepath.Dir(path), testSteamID); !ok {
		t.Fatal("Path should find the cached jpeg")
	}
}

func TestCacheDirAndValidSteamID(t *testing.T) {
	if got := CacheDir("/data"); got != filepath.Join("/data", "cache", "radar_cards", "portraits") {
		t.Fatalf("cache dir = %s", got)
	}
	if !ValidSteamID64(testSteamID) || ValidSteamID64("123") {
		t.Fatal("steamid validation is wrong")
	}
}

// TestZeroClientUsesDefaultHTTP guards the nil-HTTP path: a struct literal with
// only ProfileURL used to panic in download.
func TestZeroClientUsesDefaultHTTP(t *testing.T) {
	dir := t.TempDir()
	c := &Client{ProfileURL: "http://127.0.0.1:1/profile?steamid=%s"}
	if _, err := c.Fetch(t.Context(), "76561198000000009", dir, 0); err == nil {
		t.Fatal("expected a connection error, not a panic")
	}
}
