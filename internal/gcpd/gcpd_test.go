package gcpd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const pageOne = `<html><head><script>
g_sGcContinueToken = '1719999999';
g_sessionID = "0123456789abcdef01234567";
</script></head><body>
<table><tr><td class="val_left">
  <table class="csgo_scoreboard_inner_left"><tbody>
    <tr><td><a href="https://replay129.valve.net/730/3638895521671676017_1102521424.dem.bz2">Download</a></td></tr>
    <tr><td>2026-11-20 18:22:41 GMT</td></tr>
  </tbody></table>
</td></tr>
<tr><td class="val_left">
  <table class="csgo_scoreboard_inner_left"><tbody>
    <tr><td><a href="https://replay131.valve.net/730/3638111111671676017_1102521425.dem.bz2">Download</a></td></tr>
    <tr><td>2026-11-19 20:02:10 GMT</td></tr>
  </tbody></table>
</td></tr>
<tr><td class="val_left">
  <table class="csgo_scoreboard_inner_left"><tbody>
    <tr><td>Demo expired</td></tr>
    <tr><td>2026-11-18 10:00:00 GMT</td></tr>
  </tbody></table>
</td></tr>
</table></body></html>`

const pageTwo = `<html><body><table><tr><td class="val_left">
  <table class="csgo_scoreboard_inner_left"><tbody>
    <tr><td><a href="https://replay17.valve.net/730/3637000000000000000_1102521499.dem.bz2">Download</a></td></tr>
    <tr><td>2026-11-17 07:31:00 GMT</td></tr>
  </tbody></table>
</td></tr></table></body></html>`

func TestParsePage(t *testing.T) {
	got := ParsePage(pageOne, "matchhistorypremier")
	if len(got) != 2 {
		t.Fatalf("matches = %d: %+v", len(got), got)
	}
	if got[0].MatchID != 3638895521671676017 || got[0].Outcome != 1102521424 {
		t.Fatalf("first = %+v", got[0])
	}
	if !got[0].Date.Equal(time.Date(2026, 11, 20, 18, 22, 41, 0, time.UTC)) {
		t.Fatalf("date = %v", got[0].Date)
	}
	if got[0].Filename != "3638895521671676017_1102521424.dem.bz2" || got[0].Type != "matchhistorypremier" {
		t.Fatalf("meta = %+v", got[0])
	}
}

func TestParsePageDedupe(t *testing.T) {
	doubled := pageOne + pageOne
	if got := ParsePage(doubled, "premier"); len(got) != 2 {
		t.Fatalf("duplicates not removed: %+v", got)
	}
}

func TestListFollowsContinueToken(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RawQuery)
		if got := r.Header.Get("Cookie"); got != "steamLoginSecure=abc" {
			t.Errorf("cookie = %q", got)
		}
		if r.URL.Query().Get("continue_token") == "" {
			fmt.Fprint(w, pageOne)
			return
		}
		if r.URL.Query().Get("sessionid") != "0123456789abcdef01234567" || r.URL.Query().Get("ajax") != "1" {
			t.Errorf("bad ajax params: %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, pageTwo)
	}))
	defer srv.Close()

	c := NewClient("steamLoginSecure=abc")
	c.BaseURL = srv.URL
	matches, err := c.List(context.Background(), "matchhistorypremier")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("matches = %d", len(matches))
	}
	if len(paths) != 2 {
		t.Fatalf("pages fetched = %d (%v)", len(paths), paths)
	}
	if !matches[0].Date.After(matches[1].Date) {
		t.Fatalf("not sorted newest first: %v %v", matches[0].Date, matches[1].Date)
	}
}

func TestListRequiresCookie(t *testing.T) {
	c := NewClient("")
	if _, err := c.List(context.Background(), "matchhistorypremier"); err == nil {
		t.Fatal("expected cookie requirement")
	}
}

func TestListRejectsExpiredSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><script>g_steamID = false;</script></html>")
	}))
	defer srv.Close()
	c := NewClient("steamLoginSecure=stale")
	c.BaseURL = srv.URL
	_, err := c.List(context.Background(), "matchhistorypremier")
	if err == nil || !strings.Contains(err.Error(), "not signed in") {
		t.Fatalf("err = %v", err)
	}
}

func TestListRejectsForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := NewClient("steamLoginSecure=abc")
	c.BaseURL = srv.URL
	if _, err := c.List(context.Background(), "matchhistorypremier"); err == nil {
		t.Fatal("expected forbidden error")
	}
}

func TestDownloadWritesArchiveAndReusesIt(t *testing.T) {
	body := []byte("BZh91AY&SY-pretend-bzip2")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := NewClient("steamLoginSecure=abc")
	m := Match{MatchID: 7, Outcome: 9, URL: srv.URL + "/730/7_9.dem.bz2", Filename: "7_9.dem.bz2"}

	path, err := c.Download(context.Background(), m, dir)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if path != filepath.Join(dir, "7_9.dem.bz2") {
		t.Fatalf("path = %s", path)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(body) {
		t.Fatalf("content = %q err=%v", got, err)
	}
	if _, err := c.Download(context.Background(), m, dir); err != nil {
		t.Fatalf("second download: %v", err)
	}
	if hits != 1 {
		t.Fatalf("existing file should be reused, hits = %d", hits)
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Fatal("part file left behind")
	}
}

func TestDownloadEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := NewClient("steamLoginSecure=abc")
	_, err := c.Download(context.Background(), Match{URL: srv.URL, Filename: "x.dem.bz2"}, t.TempDir())
	if err == nil {
		t.Fatal("expected empty body error")
	}
}

func TestReadCookieFile(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.txt")
	if err := os.WriteFile(raw, []byte("Cookie: steamLoginSecure=abc; sessionid=def\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCookieFile(raw)
	if err != nil || got != "steamLoginSecure=abc; sessionid=def" {
		t.Fatalf("raw = %q err=%v", got, err)
	}

	netscape := filepath.Join(dir, "cookies.txt")
	content := "# Netscape HTTP Cookie File\n" +
		".steamcommunity.com\tTRUE\t/\tTRUE\t0\tsteamLoginSecure\tabc\n" +
		"example.com\tTRUE\t/\tFALSE\t0\tsession\tnope\n"
	if err := os.WriteFile(netscape, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ReadCookieFile(netscape)
	if err != nil || got != "steamLoginSecure=abc" {
		t.Fatalf("netscape = %q err=%v", got, err)
	}

	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCookieFile(empty); err == nil {
		t.Fatal("expected empty cookie error")
	}
}
