package updater

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
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	if CompareVersions("0.1.0-phase1", "0.1.0") >= 0 {
		t.Fatalf("expected 0.1.0-phase1 < 0.1.0")
	}
	if CompareVersions("0.1.0", "0.2.0") >= 0 {
		t.Fatalf("expected 0.1.0 < 0.2.0")
	}
	if CompareVersions("v0.2.0", "0.2.0") != 0 {
		t.Fatalf("expected v0.2.0 == 0.2.0")
	}
	if CompareVersions("1.0.0", "0.9.9") <= 0 {
		t.Fatalf("expected 1.0.0 > 0.9.9")
	}
}

func TestCheckUpdateAndDownloadZipHotSwap(t *testing.T) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	targetBin := "strikeye"
	if runtime.GOOS == "windows" {
		targetBin = "strikeye.exe"
	}
	fw, err := zw.Create("dist/" + targetBin)
	if err != nil {
		t.Fatal(err)
	}
	newContent := []byte("new-strikeye-binary-v0.2.0")
	if _, err := fw.Write(newContent); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(zipBuf.Bytes())
	shaHex := hex.EncodeToString(sum[:])

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			_ = json.NewEncoder(w).Encode(Manifest{
				Version: "0.2.0",
				Notes:   "Hot update test",
				Assets: map[string]Asset{
					PlatformKey(): {
						URL:    srv.URL + "/package.zip",
						SHA256: shaHex,
					},
				},
			})
		case "/package.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBuf.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res, err := CheckUpdate(context.Background(), srv.Client(), srv.URL+"/manifest.json", "0.1.0-phase1")
	if err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}
	if !res.HasUpdate || res.LatestVersion != "0.2.0" {
		t.Fatalf("expected update to 0.2.0, got %+v", res)
	}

	tmpDir := t.TempDir()
	dstBin := filepath.Join(tmpDir, targetBin)
	if err := os.WriteFile(dstBin, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	var lastProg float64
	err = DownloadAndApply(context.Background(), srv.Client(), res.Asset, tmpDir, func(_ string, p float64) {
		lastProg = p
	})
	if err != nil {
		t.Fatalf("DownloadAndApply failed: %v", err)
	}
	if lastProg != 1.0 {
		t.Fatalf("expected final progress 1.0, got %v", lastProg)
	}

	got, err := os.ReadFile(dstBin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newContent) {
		t.Fatalf("binary not updated: got %q", got)
	}
	if _, err := os.Stat(dstBin + ".old"); err != nil {
		t.Fatalf("expected .old backup before cleanup: %v", err)
	}
	CleanupOldBinaries(tmpDir)
	if _, err := os.Stat(dstBin + ".old"); !os.IsNotExist(err) {
		t.Fatalf("expected .old backup removed by CleanupOldBinaries")
	}
}

func TestCheckUpdateGitHubReleasesAPI(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.3.0",
			Body:    "GitHub Release notes",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
				Digest             string `json:"digest"`
			}{
				{
					Name:               "strikeye-" + PlatformKey() + ".zip",
					BrowserDownloadURL: srv.URL + "/strikeye-" + PlatformKey() + ".zip",
					Digest:             "sha256:abcd1234",
				},
			},
		})
	}))
	defer srv.Close()

	res, err := CheckUpdate(context.Background(), srv.Client(), srv.URL, "0.1.0-phase1")
	if err != nil {
		t.Fatalf("CheckUpdate GitHub failed: %v", err)
	}
	if !res.HasUpdate || res.LatestVersion != "0.3.0" || res.Asset.SHA256 != "abcd1234" {
		t.Fatalf("unexpected GitHub CheckResult: %+v", res)
	}
}
