package library

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadSteamCookie(t *testing.T) {
	root := t.TempDir()
	if err := saveSteamCookie(root, "  steamLoginSecure=abc; sessionid=1  "); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := loadSteamCookie(root); got != "steamLoginSecure=abc; sessionid=1" {
		t.Fatalf("cookie = %q", got)
	}
	info, err := os.Stat(steamCookiePath(root))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cookie file mode = %v, want 0600", info.Mode().Perm())
	}
	if err := saveSteamCookie(root, "   "); err == nil {
		t.Fatal("expected an error for an empty cookie")
	}
}

func TestLoadSteamCookieMissing(t *testing.T) {
	if got := loadSteamCookie(t.TempDir()); got != "" {
		t.Fatalf("cookie = %q", got)
	}
}

func TestRunSteamDownloadWithoutCookie(t *testing.T) {
	if _, err := runSteamDownload(context.Background(), t.TempDir(), "", "matchhistorypremier", 1); err == nil {
		t.Fatal("expected an error without a cookie")
	}
}

// TestRunSteamDownloadPagination drives the whole path against a fake GCPD page:
// one new demo is fetched into media/imports, an existing file is skipped.
func TestRunSteamDownloadPagination(t *testing.T) {
	const matchURL = "https://replay.example/730/3700000000000000001_001.dem.bz2"
	page := `<html><body><a href="` + matchURL + `">download</a> 2026-11-24 12:00:00</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".bz2"):
			_, _ = w.Write([]byte("demo-bz2"))
		default:
			_, _ = w.Write([]byte(page))
		}
	}))
	defer srv.Close()

	root := t.TempDir()
	// The client reads matches from the fake host but downloads from the URL in
	// the page, so rewrite the page to point at the test server.
	page = strings.ReplaceAll(page, "https://replay.example", srv.URL)
	orig := os.Getenv("STRIKEYE_GCPD_BASE")
	t.Setenv("STRIKEYE_GCPD_BASE", srv.URL)
	defer func() { _ = os.Setenv("STRIKEYE_GCPD_BASE", orig) }()

	res, err := runSteamDownload(context.Background(), root, "steamLoginSecure=abc", "matchhistorypremier", 5)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if res.Listed != 1 || len(res.Downloaded) != 1 {
		t.Fatalf("result = %+v", res)
	}
	// The file name is derived from the parsed IDs, so leading zeros in the URL
	// outcome id are dropped ("001" -> "1").
	dest := filepath.Join(root, "media", "imports", "3700000000000000001_1.dem.bz2")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}
	// Second pass must skip the file that already exists.
	res2, err := runSteamDownload(context.Background(), root, "steamLoginSecure=abc", "matchhistorypremier", 5)
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if res2.Skipped != 1 || len(res2.Downloaded) != 0 {
		t.Fatalf("second pass = %+v", res2)
	}
}
