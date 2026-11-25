// Package mapdata downloads CS2 map assets beyond collision geometry: the
// navigation meshes published by pnxenopoulos/awpy-data and the callout place
// table published by sink3x/cs2-server-admin-callouts.
//
// Neither dataset ships inside Strikeye: both are fetched on demand by an
// explicit CLI command, verified against pinned checksums, and cached under the
// DataRoot so offline parsing keeps working afterwards.
package mapdata

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NavRelease pins the awpy-data release used for navigation meshes.
const NavRelease = "2000950"

// NavBaseURL is the download root of the pinned awpy-data release. Each release
// is pinned to a CS2 ClientVersion, so checksums stay stable.
const NavBaseURL = "https://github.com/pnxenopoulos/awpy-data/releases/download/" + NavRelease

// CalloutsURL pins the callout table to one commit of
// sink3x/cs2-server-admin-callouts (MIT).
const CalloutsURL = "https://raw.githubusercontent.com/sink3x/cs2-server-admin-callouts/e222d6755591a874802a97f8f72b6c7295f59f90/schematic/data/nav_places.csv"

// CalloutsSHA256 is the expected checksum and size of CalloutsURL.
const (
	CalloutsSHA256 = "969d9ad87b71824af96e3afbc04b040ed1913a8d0dcdb3107be0be17c999adff"
	CalloutsSize   = 780350
)

// File is one downloadable artifact with its pinned checksum.
type File struct {
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// NavManifest describes an awpy-data release.
type NavManifest struct {
	Artifacts     map[string]File `json:"artifacts"`
	Maps          []string        `json:"maps"`
	ReleaseKey    string          `json:"release_key"`
	PatchVersion  string          `json:"patch_version"`
	ClientVersion string          `json:"client_version"`
	VersionDate   string          `json:"version_date"`
}

// ErrNoAsset reports a missing artifact entry.
var ErrNoAsset = errors.New("map data asset missing")

// FetchNavManifest downloads and validates the release manifest.
func FetchNavManifest(ctx context.Context, client *http.Client, baseURL string) (NavManifest, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = NavBaseURL
	}
	body, err := getLimited(ctx, client, strings.TrimRight(baseURL, "/")+"/manifest.json", 1<<20)
	if err != nil {
		return NavManifest{}, fmt.Errorf("fetch nav manifest: %w", err)
	}
	var m NavManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return NavManifest{}, fmt.Errorf("decode nav manifest: %w", err)
	}
	if len(m.Maps) == 0 || len(m.Artifacts) == 0 {
		return NavManifest{}, fmt.Errorf("nav manifest has no maps or artifacts")
	}
	return m, nil
}

// NavArchive returns the navs.zip artifact entry.
func (m NavManifest) NavArchive() (File, error) {
	f, ok := m.Artifacts["navs.zip"]
	if !ok {
		return File{}, fmt.Errorf("%w: navs.zip", ErrNoAsset)
	}
	return f, nil
}

// Maps returns the maps in the release, sorted.
func (m NavManifest) SortedMaps() []string {
	out := append([]string(nil), m.Maps...)
	sort.Strings(out)
	return out
}

// HasMap reports whether the release ships the given map.
func (m NavManifest) HasMap(name string) bool {
	for _, n := range m.Maps {
		if n == name {
			return true
		}
	}
	return false
}

// EnsureNavs downloads navs.zip (verified) into cacheDir and extracts the
// requested maps, returning their .nav paths in request order.
func EnsureNavs(ctx context.Context, client *http.Client, cacheDir, baseURL string, m NavManifest, maps []string, progress func(string)) ([]string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if len(maps) == 0 {
		return nil, fmt.Errorf("no maps requested")
	}
	for _, name := range maps {
		if !m.HasMap(name) {
			return nil, fmt.Errorf("map %q is not in nav release %s", name, m.ReleaseKey)
		}
	}
	asset, err := m.NavArchive()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create nav cache: %w", err)
	}
	archivePath := filepath.Join(cacheDir, "navs.zip")
	if ok, err := fileMatches(archivePath, asset); err != nil || !ok {
		if progress != nil {
			progress("navs.zip")
		}
		url := strings.TrimRight(baseURL, "/")
		if baseURL == "" {
			url = NavBaseURL
		}
		if err := downloadVerified(ctx, client, url+"/navs.zip", archivePath, asset); err != nil {
			return nil, err
		}
	}
	return extractNavs(archivePath, cacheDir, maps)
}

// extractNavs pulls the requested `<map>.nav` entries out of the archive.
func extractNavs(archivePath, cacheDir string, maps []string) ([]string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open nav archive: %w", err)
	}
	defer zr.Close()
	want := map[string]string{}
	for _, name := range maps {
		want[name+".nav"] = name
	}
	paths := map[string]string{}
	for _, f := range zr.File {
		mapName, ok := want[f.Name]
		if !ok {
			continue
		}
		dest := filepath.Join(cacheDir, mapName+".nav")
		if info, err := os.Stat(dest); err == nil && info.Size() == int64(f.UncompressedSize64) {
			paths[mapName] = dest
			continue
		}
		if err := extractEntry(f, dest); err != nil {
			return nil, err
		}
		paths[mapName] = dest
	}
	out := make([]string, 0, len(maps))
	for _, name := range maps {
		p, ok := paths[name]
		if !ok {
			return out, fmt.Errorf("%w: %s.nav not in archive", ErrNoAsset, name)
		}
		out = append(out, p)
	}
	return out, nil
}

func extractEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	tmp := dest + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if written != int64(f.UncompressedSize64) {
		_ = os.Remove(tmp)
		return fmt.Errorf("extract %s: got %d bytes, want %d", f.Name, written, f.UncompressedSize64)
	}
	return os.Rename(tmp, dest)
}

// CachedNavs lists map names with a .nav file in cacheDir.
func CachedNavs(cacheDir string) ([]string, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".nav") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".nav"))
	}
	sort.Strings(names)
	return names, nil
}

// EnsureCallouts downloads the callout table into dest, verified against the
// pinned checksum. An existing file with the right size is reused.
func EnsureCallouts(ctx context.Context, client *http.Client, dest string) (string, error) {
	return ensureCalloutsFrom(ctx, client, CalloutsURL, dest, File{Bytes: CalloutsSize, SHA256: CalloutsSHA256})
}

// ensureCalloutsFrom is EnsureCallouts with an injectable URL for tests.
func ensureCalloutsFrom(ctx context.Context, client *http.Client, url, dest string, want File) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if info, err := os.Stat(dest); err == nil && want.Bytes > 0 && info.Size() == want.Bytes {
		return dest, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := downloadVerified(ctx, client, url, dest, want); err != nil {
		return "", err
	}
	return dest, nil
}

func getLimited(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func fileMatches(path string, want File) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if want.Bytes > 0 && info.Size() != want.Bytes {
		return false, nil
	}
	sum, err := sha256File(path)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(sum, want.SHA256), nil
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

func downloadVerified(ctx context.Context, client *http.Client, url, dest string, want File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", filepath.Base(dest), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", filepath.Base(dest), resp.StatusCode)
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
		return fmt.Errorf("download %s: %w", filepath.Base(dest), copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if want.Bytes > 0 && written != want.Bytes {
		_ = os.Remove(tmp)
		return fmt.Errorf("download %s: got %d bytes, want %d", filepath.Base(dest), written, want.Bytes)
	}
	if sum := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(sum, want.SHA256) {
		_ = os.Remove(tmp)
		return fmt.Errorf("download %s: checksum mismatch", filepath.Base(dest))
	}
	return os.Rename(tmp, dest)
}
