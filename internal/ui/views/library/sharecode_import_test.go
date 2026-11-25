package library

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"strikeye/internal/sharecode"
)

func TestImportSharecodeRejectsBadCode(t *testing.T) {
	if _, err := importSharecode(context.Background(), t.TempDir(), "cookie", "not-a-code"); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestImportSharecodeNeedsCookie(t *testing.T) {
	code := sharecode.Encode(3700000000000000001, 3700000000000000002, 9725)
	if _, err := importSharecode(context.Background(), t.TempDir(), "  ", code); err == nil {
		t.Fatal("expected an error without a cookie")
	}
}

func TestImportSharecodeDownloadsMatch(t *testing.T) {
	const matchID = uint64(3700000000000000001)
	code := sharecode.Encode(matchID, 3700000000000000002, 9725)

	var srv *httptest.Server
	page := `<html><body>DATE 2026-11-24 12:00:00 GMT %s</body></html>`
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".bz2") {
			_, _ = w.Write([]byte("bz2"))
			return
		}
		_, _ = w.Write([]byte(strings.ReplaceAll(
			page,
			"%s",
			"<a href=\""+srv.URL+"/730/3700000000000000001_3.dem.bz2\">dl</a>",
		)))
	}))
	defer srv.Close()
	t.Setenv("STRIKEYE_GCPD_BASE", srv.URL)

	root := t.TempDir()
	res, err := importSharecode(context.Background(), root, "steamLoginSecure=abc", code)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !res.Found || res.MatchID != matchID || res.Tab == "" {
		t.Fatalf("result = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "media", "imports", "3700000000000000001_3.dem.bz2")); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}

	// Second call sees the file and reports it as skipped.
	res2, err := importSharecode(context.Background(), root, "steamLoginSecure=abc", code)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if !res2.Skipped {
		t.Fatalf("second result = %+v", res2)
	}
}

func TestImportSharecodeNotInHistory(t *testing.T) {
	code := sharecode.Encode(1234, 5678, 9725)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>no demos here</body></html>"))
	}))
	defer srv.Close()
	t.Setenv("STRIKEYE_GCPD_BASE", srv.URL)
	if _, err := importSharecode(context.Background(), t.TempDir(), "cookie=1", code); err == nil {
		t.Fatal("expected a not-found error")
	}
}
