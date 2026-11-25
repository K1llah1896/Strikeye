package mapgeom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultBaseURL points at the pinned cs2-map-geometry dataset version.
// The dataset is collision geometry generated from CS2 map files by akiver.
const DefaultBaseURL = "https://gitlab.com/akiver/cs2-map-geometry/-/raw/v1"

// Manifest describes every geometry file and its checksum.
type Manifest struct {
	FormatVersion int                      `json:"formatVersion"`
	Maps          map[string]ManifestEntry `json:"maps"`
}

// ManifestEntry is one map's file metadata.
type ManifestEntry struct {
	File   string `json:"file"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// FetchManifest downloads and validates the dataset manifest.
func FetchManifest(ctx context.Context, client *http.Client, baseURL string) (Manifest, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/manifest.json", nil)
	if err != nil {
		return Manifest{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("fetch map geometry manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("fetch map geometry manifest: HTTP %d", resp.StatusCode)
	}
	var m Manifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("decode map geometry manifest: %w", err)
	}
	if m.FormatVersion != 1 || len(m.Maps) == 0 {
		return Manifest{}, fmt.Errorf("unsupported map geometry manifest (format %d, %d maps)", m.FormatVersion, len(m.Maps))
	}
	return m, nil
}

// MapNames returns the maps listed in the manifest, sorted.
func (m Manifest) MapNames() []string {
	names := make([]string, 0, len(m.Maps))
	for name := range m.Maps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// EnsureMaps downloads missing or stale geometry files into cacheDir and returns
// their paths. Files matching the manifest checksum are reused.
func EnsureMaps(ctx context.Context, client *http.Client, cacheDir, baseURL string, m Manifest, maps []string, progress func(string)) ([]string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if len(maps) == 0 {
		return nil, fmt.Errorf("no maps requested")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create geometry cache: %w", err)
	}
	var out []string
	for _, name := range maps {
		entry, ok := m.Maps[name]
		if !ok {
			return out, fmt.Errorf("map %q is not in the geometry manifest", name)
		}
		path := filepath.Join(cacheDir, name+Extension)
		if ok, err := fileMatches(path, entry); err == nil && ok {
			out = append(out, path)
			continue
		}
		if progress != nil {
			progress(name)
		}
		if err := downloadFile(ctx, client, strings.TrimRight(baseURL, "/")+"/"+entry.File, path, entry); err != nil {
			return out, err
		}
		out = append(out, path)
	}
	return out, nil
}

func fileMatches(path string, entry ManifestEntry) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if entry.Size > 0 && info.Size() != entry.Size {
		return false, nil
	}
	sum, err := sha256File(path)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(sum, entry.SHA256), nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string, entry ManifestEntry) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", entry.File, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", entry.File, resp.StatusCode)
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(f, h), resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("download %s: %w", entry.File, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if entry.Size > 0 && written != entry.Size {
		_ = os.Remove(tmp)
		return fmt.Errorf("download %s: got %d bytes, want %d", entry.File, written, entry.Size)
	}
	if sum := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(sum, entry.SHA256) {
		_ = os.Remove(tmp)
		return fmt.Errorf("download %s: checksum mismatch", entry.File)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// CachedMaps lists map names with a geometry file present in cacheDir.
func CachedMaps(cacheDir string) ([]string, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), Extension) {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), Extension))
	}
	sort.Strings(names)
	return names, nil
}
