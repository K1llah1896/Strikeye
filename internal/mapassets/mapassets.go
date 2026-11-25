// Package mapassets ships the map datasets a fresh install needs so no user has
// to click "download" before parsing or analysing a match.
//
// Two different mechanisms, chosen by size:
//
//   - Navigation meshes (7.7 MB) and the callout table (780 KB) are embedded in
//     the binaries and written into the DataRoot on startup.
//   - Collision geometry is ~200 MB, so it travels as map_geometry.zip next to
//     the executables and is extracted one map at a time, the first time a map
//     is actually needed.
//
// Both paths verify pinned SHA-256 checksums before anything is unpacked, and
// both are idempotent: an already-installed file of the right size is reused.
// Downloading remains available (see internal/mapdata and internal/mapgeom) as a
// way to pick up newer datasets, never as a prerequisite.
package mapassets

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed all:assets
var bundled embed.FS

// Blob names inside the embedded assets directory.
const (
	NavsAsset     = "assets/navs.zip"
	CalloutsAsset = "assets/callouts.csv"
)

// GeometryBundleName is the sidecar archive shipped next to the executables.
const GeometryBundleName = "map_geometry.zip"

// Pinned checksums of the embedded blobs. They match the constants in
// internal/mapdata (navs.zip from awpy-data release 2000950, callouts from
// sink3x/cs2-server-admin-callouts @ e222d67) and are re-checked at install time
// so a tampered or truncated build input can never reach the cache.
const (
	NavsSHA256     = "2e853e6a54f9f865d636725ee957f10f904b9fa91c8cebd86d05fba3d381a097"
	NavsBytes      = 7683060
	CalloutsSHA256 = "969d9ad87b71824af96e3afbc04b040ed1913a8d0dcdb3107be0be17c999adff"
	CalloutsBytes  = 780350
)

// ErrNotBundled reports that this build carries no embedded blob (a source
// build without `strikeye-cli assets-pack`).
var ErrNotBundled = errors.New("map assets are not bundled in this build")

// ErrChecksum reports a bundled or downloaded blob that does not match the pins.
var ErrChecksum = errors.New("map asset checksum mismatch")

// ErrMapNotBundled reports a map missing from the geometry bundle.
var ErrMapNotBundled = errors.New("map not present in the geometry bundle")

// fsys and the pins are swapped in tests.
var (
	fsys        fs.FS = bundled
	navsPin           = pin{NavsBytes, NavsSHA256}
	calloutsPin       = pin{CalloutsBytes, CalloutsSHA256}
)

// pin is a size and checksum pair a blob must match before it is unpacked.
type pin struct {
	Bytes  int64
	SHA256 string
}

// Report summarises what an Install call materialised.
type Report struct {
	// NavAvailable is false when the build carries no embedded nav archive.
	NavAvailable bool `json:"nav_available"`
	// NavsWritten counts .nav files extracted into the cache.
	NavsWritten int `json:"navs_written"`
	// NavsPresent counts .nav files already valid before the call.
	NavsPresent int `json:"navs_present"`
	// CalloutsWritten reports whether callouts.csv was (re)written.
	CalloutsWritten bool `json:"callouts_written"`
	// GeometryBundle is the path of the sidecar archive, if one was found.
	GeometryBundle string `json:"geometry_bundle,omitempty"`
}

// CachePaths returns the directories the installers write into.
func CachePaths(dataRoot string) (navDir, calloutsPath, geometryDir string) {
	return filepath.Join(dataRoot, "cache", "map_nav"),
		filepath.Join(dataRoot, "cache", "map_data", "callouts.csv"),
		filepath.Join(dataRoot, "cache", "map_geometry")
}

// Available reports whether this build embeds the nav and callout datasets.
func Available() bool {
	_, err := fs.Stat(fsys, NavsAsset)
	return err == nil
}

// Install writes the embedded nav meshes and callout table into dataRoot and
// reports whether a geometry bundle sits next to the executables. It never
// touches the network; a missing blob is reported, not fetched.
func Install(dataRoot, exeDir string) (Report, error) {
	var rep Report
	dataRoot = strings.TrimSpace(dataRoot)
	if dataRoot == "" {
		return rep, fmt.Errorf("install map assets: data root is required")
	}
	navDir, calloutsPath, _ := CachePaths(dataRoot)

	rep.NavAvailable = Available()
	if rep.NavAvailable {
		written, present, err := installNavs(navDir)
		if err != nil {
			return rep, err
		}
		rep.NavsWritten, rep.NavsPresent = written, present
	}
	written, err := installCallouts(calloutsPath)
	if err != nil {
		return rep, err
	}
	rep.CalloutsWritten = written
	if path, ok := FindGeometryBundle(dataRoot, exeDir); ok {
		rep.GeometryBundle = path
	}
	return rep, nil
}

// installNavs extracts every `<map>.nav` from the embedded archive.
func installNavs(navDir string) (written, present int, err error) {
	blob, err := fs.ReadFile(fsys, NavsAsset)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, 0, ErrNotBundled
		}
		return 0, 0, fmt.Errorf("read bundled nav archive: %w", err)
	}
	if err := verifyBlob(NavsAsset, blob, navsPin); err != nil {
		return 0, 0, err
	}
	if err := os.MkdirAll(navDir, 0o755); err != nil {
		return 0, 0, fmt.Errorf("create nav cache: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if err != nil {
		return 0, 0, fmt.Errorf("open bundled nav archive: %w", err)
	}
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if !strings.HasSuffix(name, ".nav") {
			continue
		}
		dest := filepath.Join(navDir, name)
		if info, statErr := os.Stat(dest); statErr == nil && info.Size() == int64(f.UncompressedSize64) {
			present++
			continue
		}
		if err := extractEntry(f, dest); err != nil {
			return written, present, err
		}
		written++
	}
	return written, present, nil
}

// installCallouts writes the embedded callout table when it is missing or has
// the wrong size (a truncated write from an older build).
func installCallouts(path string) (bool, error) {
	blob, err := fs.ReadFile(fsys, CalloutsAsset)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read bundled callouts: %w", err)
	}
	if err := verifyBlob(CalloutsAsset, blob, calloutsPin); err != nil {
		return false, err
	}
	if info, err := os.Stat(path); err == nil && info.Size() == int64(len(blob)) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create callout cache: %w", err)
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return false, fmt.Errorf("write callouts: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}

func verifyBlob(name string, blob []byte, want pin) error {
	if want.Bytes > 0 && int64(len(blob)) != want.Bytes {
		return fmt.Errorf("%w: %s has %d bytes, want %d", ErrChecksum, name, len(blob), want.Bytes)
	}
	sum := sha256.Sum256(blob)
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want.SHA256) {
		return fmt.Errorf("%w: %s is %s, want %s", ErrChecksum, name, got, want.SHA256)
	}
	return nil
}

// BundleDirs lists where a geometry bundle may live, nearest first.
func BundleDirs(dataRoot, exeDir string) []string {
	dirs := []string{}
	if exeDir != "" {
		dirs = append(dirs, exeDir, filepath.Join(exeDir, "assets"))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd, filepath.Join(cwd, "assets"))
	}
	if dataRoot != "" {
		dirs = append(dirs, dataRoot, filepath.Join(dataRoot, "cache"), filepath.Join(dataRoot, "assets"))
	}
	return dirs
}

// FindGeometryBundle locates the sidecar archive next to the executables or in
// the DataRoot.
func FindGeometryBundle(dataRoot, exeDir string) (string, bool) {
	for _, dir := range BundleDirs(dataRoot, exeDir) {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, GeometryBundleName)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// GeometryEntry is one map inside the bundle manifest.
type GeometryEntry struct {
	File   string `json:"file"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type geometryManifest struct {
	FormatVersion int                      `json:"formatVersion"`
	Maps          map[string]GeometryEntry `json:"maps"`
}

// EnsureGeometry returns the cached `.tri` path for mapName, extracting it from
// the sidecar bundle when it is missing or truncated. The bool is false when no
// geometry is available at all (no bundle and nothing cached), which callers
// treat as "geometry-dependent metrics unavailable" rather than an error.
func EnsureGeometry(dataRoot, exeDir, mapName string) (string, bool) {
	bundle, _ := FindGeometryBundle(dataRoot, exeDir)
	return EnsureGeometryFrom(dataRoot, bundle, mapName)
}

// EnsureGeometryFrom is EnsureGeometry with an explicit bundle path (empty means
// "no bundle"), used by callers that already resolved one.
func EnsureGeometryFrom(dataRoot, bundlePath, mapName string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(mapName))
	if name == "" {
		return "", false
	}
	_, _, geometryDir := CachePaths(dataRoot)
	dest := filepath.Join(geometryDir, name+".tri")
	if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
		return dest, true
	}
	if bundlePath == "" {
		return "", false
	}
	if err := ExtractGeometry(bundlePath, geometryDir, []string{name}); err != nil {
		return "", false
	}
	if info, err := os.Stat(dest); err != nil || info.Size() == 0 {
		return "", false
	}
	return dest, true
}

// ExtractGeometry pulls the named maps out of the bundle, verifying each entry
// against the manifest embedded in the archive. Files already present with the
// right size are reused.
func ExtractGeometry(bundlePath, geometryDir string, maps []string) error {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return fmt.Errorf("open geometry bundle: %w", err)
	}
	defer zr.Close()
	manifest, entries, err := readBundleManifest(zr)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(geometryDir, 0o755); err != nil {
		return fmt.Errorf("create geometry cache: %w", err)
	}
	for _, name := range maps {
		key := strings.ToLower(strings.TrimSpace(name))
		entry, ok := manifest.Maps[key]
		if !ok {
			return fmt.Errorf("%w: %s", ErrMapNotBundled, key)
		}
		file, ok := entries[filepath.Base(entry.File)]
		if !ok {
			return fmt.Errorf("%w: %s", ErrMapNotBundled, entry.File)
		}
		dest := filepath.Join(geometryDir, key+".tri")
		if info, statErr := os.Stat(dest); statErr == nil && info.Size() == entry.Size {
			continue
		}
		if err := extractVerified(file, dest, entry); err != nil {
			return err
		}
	}
	return nil
}

// ExtractAllGeometry unpacks every map in the bundle and returns their paths.
func ExtractAllGeometry(bundlePath, geometryDir string) ([]string, error) {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("open geometry bundle: %w", err)
	}
	defer zr.Close()
	manifest, _, err := readBundleManifest(zr)
	if err != nil {
		return nil, err
	}
	names := manifest.MapNames()
	if err := ExtractGeometry(bundlePath, geometryDir, names); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.Join(geometryDir, name+".tri"))
	}
	return out, nil
}

// BundleMaps lists the maps available inside a bundle, sorted.
func BundleMaps(bundlePath string) ([]string, error) {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("open geometry bundle: %w", err)
	}
	defer zr.Close()
	manifest, _, err := readBundleManifest(zr)
	if err != nil {
		return nil, err
	}
	return manifest.MapNames(), nil
}

func (m geometryManifest) MapNames() []string {
	out := make([]string, 0, len(m.Maps))
	for name := range m.Maps {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func readBundleManifest(zr *zip.ReadCloser) (geometryManifest, map[string]*zip.File, error) {
	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		entries[filepath.Base(f.Name)] = f
	}
	mf, ok := entries["manifest.json"]
	if !ok {
		return geometryManifest{}, nil, fmt.Errorf("%w: manifest.json missing", ErrMapNotBundled)
	}
	rc, err := mf.Open()
	if err != nil {
		return geometryManifest{}, nil, err
	}
	defer rc.Close()
	var manifest geometryManifest
	if err := json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&manifest); err != nil {
		return geometryManifest{}, nil, fmt.Errorf("decode geometry bundle manifest: %w", err)
	}
	if manifest.FormatVersion != 1 || len(manifest.Maps) == 0 {
		return geometryManifest{}, nil, fmt.Errorf("unsupported geometry bundle (format %d, %d maps)", manifest.FormatVersion, len(manifest.Maps))
	}
	return manifest, entries, nil
}

func extractVerified(f *zip.File, dest string, entry GeometryEntry) error {
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
	h := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(rc, entry.Size+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return fmt.Errorf("extract %s: %w", f.Name, copyErr)
		}
		return closeErr
	}
	if entry.Size > 0 && written != entry.Size {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: %s has %d bytes, want %d", ErrChecksum, f.Name, written, entry.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, entry.SHA256) {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: %s is %s, want %s", ErrChecksum, f.Name, got, entry.SHA256)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
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
			return fmt.Errorf("extract %s: %w", f.Name, copyErr)
		}
		return closeErr
	}
	if written != int64(f.UncompressedSize64) {
		_ = os.Remove(tmp)
		return fmt.Errorf("extract %s: %d of %d bytes", f.Name, written, f.UncompressedSize64)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ExeDir returns the directory holding the running executable, or "" when it
// cannot be determined (tests, unusual sandboxes).
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}
