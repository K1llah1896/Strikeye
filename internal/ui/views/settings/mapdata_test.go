package settings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMapDataPaths(t *testing.T) {
	navDir, callouts := mapDataPaths("/root")
	if navDir != filepath.Join("/root", "cache", "map_nav") {
		t.Fatalf("navDir = %q", navDir)
	}
	if callouts != filepath.Join("/root", "cache", "map_data", "callouts.csv") {
		t.Fatalf("callouts = %q", callouts)
	}
}

func TestMapDataStatusReadsCache(t *testing.T) {
	root := t.TempDir()
	navDir, callouts := mapDataPaths(root)
	if err := os.MkdirAll(navDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(navDir, "de_dust2.nav"), []byte("nav"), 0o644); err != nil {
		t.Fatalf("write nav: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(callouts), 0o755); err != nil {
		t.Fatalf("mkdir callouts: %v", err)
	}
	if err := os.WriteFile(callouts, make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("write callouts: %v", err)
	}
	navs, size := mapDataStatus(root)
	if len(navs) != 1 || navs[0] != "de_dust2" {
		t.Fatalf("navs = %v", navs)
	}
	if size != 2048 {
		t.Fatalf("callouts size = %d", size)
	}
}

func TestGeometryCountAndDir(t *testing.T) {
	root := t.TempDir()
	if got := geometryCount(root); got != 0 {
		t.Fatalf("empty dir count = %d", got)
	}
	dir := geometryDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"de_dust2.tri", "de_ancient.tri"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if got := geometryCount(root); got != 2 {
		t.Fatalf("count = %d", got)
	}
}

func TestMapDataStatusMissing(t *testing.T) {
	navs, size := mapDataStatus(t.TempDir())
	if len(navs) != 0 || size != 0 {
		t.Fatalf("expected empty cache, got %v / %d", navs, size)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "缺失"},
		{-1, "缺失"},
		{512, "512 B"},
		{2048, "2.0 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
	}
	for _, tc := range cases {
		if got := humanBytes(tc.in); got != tc.want {
			t.Fatalf("humanBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestFetchCalloutsCreatesCacheDir checks the settings entry point creates the
// directory it reports on. The download itself is covered by internal/mapdata;
// here the network call is exercised against a local server that fails fast, so
// the assertion is about the directory contract, not the payload.
func TestFetchCalloutsCreatesCacheDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTeapot)
	}))
	defer srv.Close()

	root := t.TempDir()
	_, calloutsPath := mapDataPaths(root)
	if err := os.MkdirAll(filepath.Dir(calloutsPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := os.Stat(calloutsPath); !os.IsNotExist(err) {
		t.Fatalf("callout cache should not exist yet: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchCallouts(ctx, root); err == nil {
		t.Fatal("expected a download error for a cancelled context")
	}
	if _, err := os.Stat(calloutsPath); !os.IsNotExist(err) {
		t.Fatalf("failed download must not leave a cache file: %v", err)
	}
}
