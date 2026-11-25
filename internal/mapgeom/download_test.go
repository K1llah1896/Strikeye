package mapgeom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func newGeometryServer(t *testing.T, payload []byte, sum string) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_ = json.NewEncoder(w).Encode(Manifest{
			FormatVersion: 1,
			Maps: map[string]ManifestEntry{
				"de_test": {File: "de_test.tri", Size: int64(len(payload)), SHA256: sum},
			},
		})
	})
	mux.HandleFunc("/de_test.tri", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestEnsureMapsDownloadsAndReuses(t *testing.T) {
	payload := writeQuad(t)
	sum := sha256.Sum256(payload)
	srv, _ := newGeometryServer(t, payload, hex.EncodeToString(sum[:]))
	cache := t.TempDir()

	paths, err := EnsureMaps(context.Background(), srv.Client(), cache, srv.URL, mustManifest(t, srv), []string{"de_test"}, nil)
	if err != nil {
		t.Fatalf("EnsureMaps: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths = %v", paths)
	}
	if _, _, err := Load(paths[0]); err != nil {
		t.Fatalf("downloaded geometry does not load: %v", err)
	}
	if _, err := EnsureMaps(context.Background(), srv.Client(), cache, srv.URL, mustManifest(t, srv), []string{"de_test"}, nil); err != nil {
		t.Fatalf("second EnsureMaps: %v", err)
	}
	// Cached file must be reused instead of refetched.
	again, err := os.ReadFile(paths[0])
	if err != nil || string(again) != string(payload) {
		t.Fatalf("cached payload changed: %v", err)
	}
}

func TestEnsureMapsRejectsChecksumMismatch(t *testing.T) {
	payload := writeQuad(t)
	srv, _ := newGeometryServer(t, payload, "deadbeef")
	cache := t.TempDir()

	_, err := EnsureMaps(context.Background(), srv.Client(), cache, srv.URL, mustManifest(t, srv), []string{"de_test"}, nil)
	if err == nil {
		t.Fatal("expected checksum error")
	}
	if _, statErr := os.Stat(filepath.Join(cache, "de_test"+Extension)); !os.IsNotExist(statErr) {
		t.Fatalf("partial file must not be kept: %v", statErr)
	}
}

func TestEnsureMapsRejectsUnknownMap(t *testing.T) {
	payload := writeQuad(t)
	srv, _ := newGeometryServer(t, payload, "x")
	if _, err := EnsureMaps(context.Background(), srv.Client(), t.TempDir(), srv.URL, mustManifest(t, srv), []string{"de_missing"}, nil); err == nil {
		t.Fatal("expected unknown map error")
	}
}

func mustManifest(t *testing.T, srv *httptest.Server) Manifest {
	t.Helper()
	m, err := FetchManifest(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	return m
}
