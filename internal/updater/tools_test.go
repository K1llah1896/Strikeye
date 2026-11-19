package updater

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func buildTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInstallToolHLAEAndFFmpeg(t *testing.T) {
	hlaeZip := buildTestZip(t, map[string]string{
		"HLAE.exe":           "hlae-bin",
		"AfxHookSource2.dll": "afx-dll",
	})
	ffmpegZip := buildTestZip(t, map[string]string{
		"ffmpeg-master/bin/ffmpeg.exe":  "ffmpeg-bin",
		"ffmpeg-master/bin/ffprobe.exe": "ffprobe-bin",
	})

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hlae-release":
			_ = json.NewEncoder(w).Encode(githubRelease{
				TagName: "v2.180.0",
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{Name: "hlae_2_180_0.zip", BrowserDownloadURL: srv.URL + "/hlae.zip"},
				},
			})
		case "/hlae.zip":
			_, _ = w.Write(hlaeZip)
		case "/ffmpeg-release":
			_ = json.NewEncoder(w).Encode(githubRelease{
				TagName: "latest",
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
					Digest             string `json:"digest"`
				}{
					{Name: "ffmpeg-master-latest-win64-gpl.zip", BrowserDownloadURL: srv.URL + "/ffmpeg.zip"},
				},
			})
		case "/ffmpeg.zip":
			_, _ = w.Write(ffmpegZip)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dataRoot := t.TempDir()

	hlaeRes, err := InstallTool(context.Background(), srv.Client(), InstallToolOptions{
		Tool:       ToolHLAE,
		DataRoot:   dataRoot,
		ReleaseAPI: srv.URL + "/hlae-release",
	}, nil)
	if err != nil {
		t.Fatalf("InstallTool HLAE failed: %v", err)
	}
	if _, err := os.Stat(hlaeRes.HLAEExe); err != nil {
		t.Fatalf("expected HLAE.exe at %q: %v", hlaeRes.HLAEExe, err)
	}
	if _, err := os.Stat(hlaeRes.AfxHookSource2DLL); err != nil {
		t.Fatalf("expected AfxHookSource2.dll at %q: %v", hlaeRes.AfxHookSource2DLL, err)
	}

	ffRes, err := InstallTool(context.Background(), srv.Client(), InstallToolOptions{
		Tool:       ToolFFmpeg,
		DataRoot:   dataRoot,
		ReleaseAPI: srv.URL + "/ffmpeg-release",
	}, nil)
	if err != nil {
		t.Fatalf("InstallTool FFmpeg failed: %v", err)
	}
	if filepath.Base(ffRes.FFmpegExe) != "ffmpeg.exe" || filepath.Base(ffRes.FFprobeExe) != "ffprobe.exe" {
		t.Fatalf("unexpected ffmpeg install paths: %+v", ffRes)
	}
}
