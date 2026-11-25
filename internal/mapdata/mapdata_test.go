package mapdata

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildNavArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func testServer(t *testing.T, navs []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	manifest := NavManifest{
		Artifacts:     map[string]File{"navs.zip": {Bytes: int64(len(navs)), SHA256: sha(navs)}},
		Maps:          []string{"de_dust2", "de_ancient"},
		ReleaseKey:    "test",
		ClientVersion: "0",
	}
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(manifest)
	})
	mux.HandleFunc("/navs.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(navs)
	})
	return httptest.NewServer(mux)
}

func TestFetchManifestAndExtractNavs(t *testing.T) {
	archive := buildNavArchive(t, map[string][]byte{
		"de_dust2.nav":   []byte("dust2-nav-bytes"),
		"de_ancient.nav": []byte("ancient"),
		"cs_office.nav":  []byte("office"),
	})
	srv := testServer(t, archive)
	defer srv.Close()

	ctx := context.Background()
	manifest, err := FetchNavManifest(ctx, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("FetchNavManifest: %v", err)
	}
	if !manifest.HasMap("de_dust2") || manifest.HasMap("cs_office") {
		t.Fatalf("HasMap wrong: %+v", manifest.Maps)
	}
	if got := manifest.SortedMaps(); got[0] != "de_ancient" {
		t.Fatalf("SortedMaps = %v", got)
	}

	cacheDir := t.TempDir()
	paths, err := EnsureNavs(ctx, srv.Client(), cacheDir, srv.URL, manifest, []string{"de_dust2"}, func(name string) {})
	if err != nil {
		t.Fatalf("EnsureNavs: %v", err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "de_dust2.nav" {
		t.Fatalf("paths = %v", paths)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil || string(data) != "dust2-nav-bytes" {
		t.Fatalf("extracted nav = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "navs.zip")); err != nil {
		t.Fatalf("archive not cached: %v", err)
	}

	// Second call reuses the cached archive and extracted file.
	if _, err := EnsureNavs(ctx, srv.Client(), cacheDir, srv.URL, manifest, []string{"de_ancient"}, nil); err != nil {
		t.Fatalf("second EnsureNavs: %v", err)
	}
	cached, err := CachedNavs(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cached, ",") != "de_ancient,de_dust2" {
		t.Fatalf("cached = %v", cached)
	}
}

func TestEnsureNavsRejectsUnknownMapAndBadChecksum(t *testing.T) {
	archive := buildNavArchive(t, map[string][]byte{"de_dust2.nav": []byte("x")})
	srv := testServer(t, archive)
	defer srv.Close()
	ctx := context.Background()
	manifest, err := FetchNavManifest(ctx, srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureNavs(ctx, srv.Client(), t.TempDir(), srv.URL, manifest, []string{"de_mirage"}, nil); err == nil {
		t.Fatal("expected unknown map error")
	}

	manifest.Artifacts["navs.zip"] = File{Bytes: int64(len(archive)), SHA256: strings.Repeat("0", 64)}
	dir := t.TempDir()
	if _, err := EnsureNavs(ctx, srv.Client(), dir, srv.URL, manifest, []string{"de_dust2"}, nil); err == nil {
		t.Fatal("expected checksum mismatch")
	}
	if _, err := os.Stat(filepath.Join(dir, "navs.zip")); !os.IsNotExist(err) {
		t.Fatalf("partial archive kept: %v", err)
	}
}

func TestEnsureCalloutsFrom(t *testing.T) {
	body := []byte("map,name,x1,y1,z1,x2,y2,z2\nde_dust2,TSpawn,0,0,0,1,1,1\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "map_data", "callouts.csv")
	path, err := ensureCalloutsFrom(context.Background(), srv.Client(), srv.URL+"/callouts.csv", dest, File{Bytes: int64(len(body)), SHA256: sha(body)})
	if err != nil {
		t.Fatalf("ensureCalloutsFrom: %v", err)
	}
	if path != dest {
		t.Fatalf("path = %s", path)
	}
	if got, err := os.ReadFile(dest); err != nil || !bytes.Equal(got, body) {
		t.Fatalf("dest content = %q, %v", got, err)
	}
}
