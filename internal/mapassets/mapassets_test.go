package mapassets

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// pinFor returns the pin a synthetic blob must satisfy, keeping the production
// verification path intact while letting tests use small payloads.
func pinFor(blob []byte) pin {
	sum := sha256.Sum256(blob)
	return pin{Bytes: int64(len(blob)), SHA256: hex.EncodeToString(sum[:])}
}

func withFakeAssets(t *testing.T, navsZip, callouts []byte) {
	t.Helper()
	fake := fstest.MapFS{}
	if navsZip != nil {
		fake[NavsAsset] = &fstest.MapFile{Data: navsZip}
	}
	if callouts != nil {
		fake[CalloutsAsset] = &fstest.MapFile{Data: callouts}
	}
	prevFS, prevNavs, prevCallouts := fsys, navsPin, calloutsPin
	fsys = fake
	if navsZip != nil {
		navsPin = pinFor(navsZip)
	}
	if callouts != nil {
		calloutsPin = pinFor(callouts)
	}
	t.Cleanup(func() { fsys, navsPin, calloutsPin = prevFS, prevNavs, prevCallouts })
}

// buildNavZip returns a navs.zip-shaped archive with the given entries.
func buildNavZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

const sampleCallouts = "map,name,x1,y1,z1,x2,y2,z2\nde_dust2,A Site,-2476,3239,0,-1000,2000,300\n"

func TestInstallWritesNavsAndCallouts(t *testing.T) {
	navZip := buildNavZip(t, map[string][]byte{
		"de_dust2.nav":  []byte("dust2-nav-bytes"),
		"de_mirage.nav": []byte("mirage-nav-bytes"),
		"README.txt":    []byte("ignored"),
	})
	withFakeAssets(t, navZip, []byte(sampleCallouts))
	dataRoot := t.TempDir()

	rep, err := Install(dataRoot, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !rep.NavAvailable || rep.NavsWritten != 2 || rep.CalloutsWritten != true {
		t.Fatalf("unexpected report: %+v", rep)
	}
	navDir, calloutsPath, _ := CachePaths(dataRoot)
	for _, name := range []string{"de_dust2.nav", "de_mirage.nav"} {
		body, err := os.ReadFile(filepath.Join(navDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if len(body) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
	if _, err := os.Stat(filepath.Join(navDir, "README.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("non-nav entry should not be extracted, got %v", err)
	}
	body, err := os.ReadFile(calloutsPath)
	if err != nil {
		t.Fatalf("read callouts: %v", err)
	}
	if string(body) != sampleCallouts {
		t.Fatalf("callouts mismatch: %q", body)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	navZip := buildNavZip(t, map[string][]byte{"de_dust2.nav": []byte("dust2-nav-bytes")})
	withFakeAssets(t, navZip, []byte(sampleCallouts))
	dataRoot := t.TempDir()

	if _, err := Install(dataRoot, ""); err != nil {
		t.Fatalf("first install: %v", err)
	}
	rep, err := Install(dataRoot, "")
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if rep.NavsWritten != 0 || rep.NavsPresent != 1 {
		t.Fatalf("second install should reuse, got %+v", rep)
	}
	if rep.CalloutsWritten {
		t.Fatalf("callouts should not be rewritten, got %+v", rep)
	}
}

func TestInstallReplacesTruncatedCallouts(t *testing.T) {
	withFakeAssets(t, buildNavZip(t, map[string][]byte{"de_dust2.nav": []byte("x")}), []byte(sampleCallouts))
	dataRoot := t.TempDir()
	_, calloutsPath, _ := CachePaths(dataRoot)
	if err := os.MkdirAll(filepath.Dir(calloutsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(calloutsPath, []byte("map,name\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Install(dataRoot, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !rep.CalloutsWritten {
		t.Fatalf("truncated callouts should be rewritten, got %+v", rep)
	}
	body, err := os.ReadFile(calloutsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != sampleCallouts {
		t.Fatalf("callouts not replaced: %q", body)
	}
}

func TestInstallRejectsTamperedBlob(t *testing.T) {
	navZip := buildNavZip(t, map[string][]byte{"de_dust2.nav": []byte("dust2-nav-bytes")})
	fake := fstest.MapFS{
		NavsAsset:     &fstest.MapFile{Data: navZip},
		CalloutsAsset: &fstest.MapFile{Data: []byte(sampleCallouts)},
	}
	prevFS, prevPins := fsys, []pin{navsPin, calloutsPin}
	fsys = fake
	navsPin = pin{Bytes: int64(len(navZip)), SHA256: "deadbeef"}
	t.Cleanup(func() { fsys, navsPin, calloutsPin = prevFS, prevPins[0], prevPins[1] })

	dataRoot := t.TempDir()
	if _, err := Install(dataRoot, ""); !errors.Is(err, ErrChecksum) {
		t.Fatalf("want ErrChecksum, got %v", err)
	}
	navDir, calloutsPath, _ := CachePaths(dataRoot)
	if entries, err := os.ReadDir(navDir); err == nil && len(entries) > 0 {
		t.Fatalf("no nav files should be written on checksum failure: %v", entries)
	}
	if _, err := os.Stat(calloutsPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("callouts should not be written after nav failure, got %v", err)
	}
}

func TestInstallReportsMissingBundle(t *testing.T) {
	withFakeAssets(t, buildNavZip(t, map[string][]byte{"de_dust2.nav": []byte("x")}), []byte(sampleCallouts))
	rep, err := Install(t.TempDir(), "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if rep.GeometryBundle != "" {
		t.Fatalf("no bundle expected, got %q", rep.GeometryBundle)
	}
}

// writeGeometryBundle builds a sidecar archive with a manifest and the given maps.
func writeGeometryBundle(t *testing.T, dir string, maps map[string][]byte, corrupt map[string]bool) string {
	t.Helper()
	manifest := geometryManifest{FormatVersion: 1, Maps: map[string]GeometryEntry{}}
	for name, body := range maps {
		sum := sha256.Sum256(body)
		sha := hex.EncodeToString(sum[:])
		if corrupt[name] {
			sha = "00" + sha[2:]
		}
		manifest.Maps[name] = GeometryEntry{File: name + ".tri", Size: int64(len(body)), SHA256: sha}
	}
	path := filepath.Join(dir, GeometryBundleName)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	mw, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(mw).Encode(manifest); err != nil {
		t.Fatal(err)
	}
	for name, body := range maps {
		w, err := zw.Create(name + ".tri")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnsureGeometryExtractsFromBundle(t *testing.T) {
	exeDir := t.TempDir()
	bundle := writeGeometryBundle(t, exeDir, map[string][]byte{
		"de_dust2":  []byte("tri-dust2"),
		"de_mirage": []byte("tri-mirage"),
	}, nil)
	dataRoot := t.TempDir()

	path, ok := EnsureGeometry(dataRoot, exeDir, "de_dust2")
	if !ok {
		t.Fatal("EnsureGeometry reported no geometry")
	}
	want := filepath.Join(dataRoot, "cache", "map_geometry", "de_dust2.tri")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "tri-dust2" {
		t.Fatalf("geometry body = %q", body)
	}
	// A second call must not re-extract (and must keep working without the bundle).
	if err := os.Remove(bundle); err != nil {
		t.Fatal(err)
	}
	if _, ok := EnsureGeometry(dataRoot, exeDir, "de_dust2"); !ok {
		t.Fatal("cached geometry should be reused after the bundle is gone")
	}
	maps, err := BundleMaps(filepath.Join(exeDir, GeometryBundleName))
	if err == nil {
		t.Fatalf("bundle was removed, expected an error, got %v", maps)
	}
}

func TestEnsureGeometryMissingMap(t *testing.T) {
	exeDir := t.TempDir()
	writeGeometryBundle(t, exeDir, map[string][]byte{"de_dust2": []byte("tri")}, nil)
	if path, ok := EnsureGeometry(t.TempDir(), exeDir, "de_nuke"); ok {
		t.Fatalf("expected no geometry for absent map, got %q", path)
	}
}

func TestEnsureGeometryRejectsCorruptEntry(t *testing.T) {
	exeDir := t.TempDir()
	writeGeometryBundle(t, exeDir, map[string][]byte{"de_dust2": []byte("tri")}, map[string]bool{"de_dust2": true})
	dataRoot := t.TempDir()
	if path, ok := EnsureGeometry(dataRoot, exeDir, "de_dust2"); ok {
		t.Fatalf("checksum mismatch must not yield geometry, got %q", path)
	}
	leftovers, err := filepath.Glob(filepath.Join(dataRoot, "cache", "map_geometry", "*"))
	if err == nil && len(leftovers) > 0 {
		t.Fatalf("no files should survive a checksum failure: %v", leftovers)
	}
}

func TestExtractAllGeometry(t *testing.T) {
	exeDir := t.TempDir()
	bundle := writeGeometryBundle(t, exeDir, map[string][]byte{
		"de_dust2": []byte("a"),
		"de_nuke":  []byte("bb"),
	}, nil)
	geometryDir := filepath.Join(t.TempDir(), "cache", "map_geometry")
	paths, err := ExtractAllGeometry(bundle, geometryDir)
	if err != nil {
		t.Fatalf("ExtractAllGeometry: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("want 2 paths, got %v", paths)
	}
	maps, err := BundleMaps(bundle)
	if err != nil {
		t.Fatalf("BundleMaps: %v", err)
	}
	if len(maps) != 2 || maps[0] != "de_dust2" || maps[1] != "de_nuke" {
		t.Fatalf("BundleMaps = %v", maps)
	}
}

func TestFindGeometryBundlePrefersExeDir(t *testing.T) {
	exeDir := t.TempDir()
	dataRoot := t.TempDir()
	exeBundle := writeGeometryBundle(t, exeDir, map[string][]byte{"de_dust2": []byte("a")}, nil)
	writeGeometryBundle(t, dataRoot, map[string][]byte{"de_dust2": []byte("b")}, nil)

	got, ok := FindGeometryBundle(dataRoot, exeDir)
	if !ok || got != exeBundle {
		t.Fatalf("FindGeometryBundle = %q (ok=%t), want %q", got, ok, exeBundle)
	}
}

func TestAvailableWithoutBlobs(t *testing.T) {
	withFakeAssets(t, nil, nil)
	if Available() {
		t.Fatal("Available must be false when no nav archive is embedded")
	}
	rep, err := Install(t.TempDir(), "")
	if err != nil {
		t.Fatalf("Install without navs should still write callouts-clean report: %v", err)
	}
	if rep.NavAvailable {
		t.Fatalf("NavAvailable should be false, got %+v", rep)
	}
}

// TestEmbeddedNavsParse extracts a real nav mesh from the bundled archive when
// this build carries one, proving the shipped data is valid for the parser.
func TestEmbeddedNavsParse(t *testing.T) {
	if !Available() {
		t.Skip("no bundled nav archive in this build (run `strikeye-cli assets-pack`)")
	}
	dataRoot := t.TempDir()
	rep, err := Install(dataRoot, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if rep.NavsWritten == 0 {
		t.Fatalf("expected nav meshes to be extracted, got %+v", rep)
	}
	navDir, _, _ := CachePaths(dataRoot)
	info, err := os.Stat(filepath.Join(navDir, "de_dust2.nav"))
	if err != nil {
		t.Fatalf("de_dust2.nav: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("de_dust2.nav is empty")
	}
}
