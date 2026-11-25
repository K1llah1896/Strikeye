package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strikeye/internal/toolbundle"
)

// TestPackToolsReusesExistingArchives pins the packaging behaviour: a
// re-run must never re-download a sidecar that is already on disk, and it must
// report the archive names the release workflow zips up.
func TestPackToolsReusesExistingArchives(t *testing.T) {
	outDir := t.TempDir()
	for _, name := range []string{toolbundle.FFmpegArchiveName, toolbundle.HLAAArchiveName} {
		path := filepath.Join(outDir, name)
		if err := os.WriteFile(path, []byte("already-fetched"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(filepath.Join(outDir, toolbundle.FFmpegArchiveName))
	if err != nil {
		t.Fatal(err)
	}
	names, err := packTools(context.Background(), nil, outDir, "", true)
	if err != nil {
		t.Fatalf("packTools: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected two archives, got %v", names)
	}
	after, err := os.Stat(filepath.Join(outDir, toolbundle.FFmpegArchiveName))
	if err != nil {
		t.Fatal(err)
	}
	if after.ModTime() != before.ModTime() || after.Size() != before.Size() {
		t.Fatal("existing archive was rewritten instead of reused")
	}
	body, err := os.ReadFile(filepath.Join(outDir, toolbundle.HLAAArchiveName))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "already-fetched" {
		t.Fatalf("hlae archive changed: %q", string(body))
	}
}

func TestPackToolsRequiresDownloadWhenMissing(t *testing.T) {
	// No archives on disk and no client: the fetcher must report an error
	// rather than silently succeeding with nothing to ship.
	client := &http.Client{Timeout: 50 * time.Millisecond}
	if _, err := packTools(context.Background(), client, t.TempDir(), "http://127.0.0.1:1/", false); err == nil {
		t.Fatal("expected a fetch failure for an unpopulated tools dir")
	}
}
