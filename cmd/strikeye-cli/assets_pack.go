package main

import (
	"archive/zip"
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
	"time"

	"strikeye/internal/mapassets"
	"strikeye/internal/mapdata"
	"strikeye/internal/mapgeom"
	"strikeye/internal/toolbundle"
	"strikeye/internal/updater"
)

type assetsPackOptions struct {
	AssetDir    string
	GeometryOut string
	ToolsOut    string
	SkipNav     bool
	SkipGeom    bool
	SkipTools   bool
	BaseURL     string
	ToolsAPI    string
}

type packResult struct {
	AssetDir     string   `json:"asset_dir"`
	GeometryZip  string   `json:"geometry_zip,omitempty"`
	Navs         string   `json:"navs,omitempty"`
	Callouts     string   `json:"callouts,omitempty"`
	Maps         []string `json:"maps,omitempty"`
	GeometrySize int64    `json:"geometry_size,omitempty"`
	ToolsDir     string   `json:"tools_dir,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	ElapsedMS    int64    `json:"elapsed_ms"`
}

// runAssetsPack downloads the pinned map datasets into the build inputs used by
// go:embed (nav meshes + callouts) and into the sidecar geometry archive.
func runAssetsPack(ctx context.Context, opts assetsPackOptions, out io.Writer) error {
	started := time.Now()
	client := &http.Client{Timeout: 10 * time.Minute}
	assetDir := opts.AssetDir
	if assetDir == "" {
		assetDir = filepath.Join("internal", "mapassets", "assets")
	}
	res := packResult{AssetDir: assetDir}
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return fmt.Errorf("create asset dir: %w", err)
	}

	if !opts.SkipNav {
		if err := packNavs(ctx, client, assetDir); err != nil {
			return err
		}
		res.Navs = filepath.Join(assetDir, "navs.zip")
		calloutsPath := filepath.Join(assetDir, "callouts.csv")
		if err := packCallouts(ctx, client, calloutsPath); err != nil {
			return err
		}
		res.Callouts = calloutsPath
	}

	if !opts.SkipGeom {
		geomOut := opts.GeometryOut
		if geomOut == "" {
			geomOut = "assets"
		}
		if err := os.MkdirAll(geomOut, 0o755); err != nil {
			return fmt.Errorf("create geometry dir: %w", err)
		}
		bundle := filepath.Join(geomOut, mapassets.GeometryBundleName)
		maps, size, err := packGeometry(ctx, client, opts.BaseURL, bundle)
		if err != nil {
			return err
		}
		res.GeometryZip, res.Maps, res.GeometrySize = bundle, maps, size
	}
	if !opts.SkipTools {
		toolsOut := opts.ToolsOut
		if toolsOut == "" {
			toolsOut = "assets"
		}
		names, err := packTools(ctx, client, toolsOut, opts.ToolsAPI, true)
		if err != nil {
			return err
		}
		res.ToolsDir, res.Tools = toolsOut, names
	}
	res.ElapsedMS = time.Since(started).Milliseconds()
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// packTools downloads the external tool archives that Strikeye would otherwise
// fetch on demand, so the release package can ship them as sidecars.
func packTools(ctx context.Context, client *http.Client, outDir, api string, usePinned bool) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create tools dir: %w", err)
	}
	var written []string
	for _, tool := range []string{updater.ToolFFmpeg, updater.ToolHLAE} {
		name := toolbundle.ArchiveName(tool)
		dest := filepath.Join(outDir, name)
		if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
			written = append(written, name)
			continue
		}
		fmt.Fprintf(os.Stderr, "fetching %s archive...\n", tool)
		var fetchErr error
		if url, ok := toolbundle.PinnedURL(tool); ok && usePinned {
			fetchErr = updater.DownloadArchiveURL(ctx, client, url, dest, nil)
		} else {
			fetchErr = updater.DownloadToolArchive(ctx, client, tool, dest, api, nil)
		}
		if fetchErr != nil {
			return written, fmt.Errorf("fetch %s archive: %w", tool, fetchErr)
		}
		written = append(written, name)
	}
	return written, nil
}

func packNavs(ctx context.Context, client *http.Client, assetDir string) error {
	navsURL := mapdata.NavBaseURL + "/navs.zip"
	dest := filepath.Join(assetDir, "navs.zip")
	if ok, err := fileMatchesPins(dest, mapassets.NavsBytes, mapassets.NavsSHA256); err == nil && ok {
		return nil
	}
	if err := downloadPinned(ctx, client, navsURL, dest, mapassets.NavsBytes, mapassets.NavsSHA256); err != nil {
		return fmt.Errorf("fetch navs.zip: %w", err)
	}
	return nil
}

func packCallouts(ctx context.Context, client *http.Client, dest string) error {
	if ok, err := fileMatchesPins(dest, mapassets.CalloutsBytes, mapassets.CalloutsSHA256); err == nil && ok {
		return nil
	}
	if err := downloadPinned(ctx, client, mapdata.CalloutsURL, dest, mapassets.CalloutsBytes, mapassets.CalloutsSHA256); err != nil {
		return fmt.Errorf("fetch callouts.csv: %w", err)
	}
	return nil
}

func packGeometry(ctx context.Context, client *http.Client, baseURL, bundlePath string) ([]string, int64, error) {
	if baseURL == "" {
		baseURL = mapgeom.DefaultBaseURL
	}
	manifest, err := mapgeom.FetchManifest(ctx, client, baseURL)
	if err != nil {
		return nil, 0, err
	}
	names := manifest.MapNames()
	workDir, err := os.MkdirTemp("", "strikeye-geom-")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(workDir)

	base := strings.TrimRight(baseURL, "/")
	for _, name := range names {
		entry := manifest.Maps[name]
		dest := filepath.Join(workDir, name+mapgeom.Extension)
		if err := downloadPinned(ctx, client, base+"/"+entry.File, dest, entry.Size, entry.SHA256); err != nil {
			return nil, 0, fmt.Errorf("fetch geometry %s: %w", name, err)
		}
	}

	tmp := bundlePath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, 0, err
	}
	zw := zip.NewWriter(f)
	// Ship the dataset manifest verbatim so the extractor can verify each map
	// without a second network call.
	if err := writeJSONToZip(zw, "manifest.json", manifest); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return nil, 0, err
	}
	for _, name := range names {
		if err := writeFileToZip(zw, name+mapgeom.Extension, filepath.Join(workDir, name+mapgeom.Extension)); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return nil, 0, err
		}
	}
	if err := zw.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return nil, 0, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return nil, 0, err
	}
	if err := os.Rename(tmp, bundlePath); err != nil {
		_ = os.Remove(tmp)
		return nil, 0, err
	}
	info, err := os.Stat(bundlePath)
	if err != nil {
		return nil, 0, err
	}
	sort.Strings(names)
	return names, info.Size(), nil
}

func writeJSONToZip(zw *zip.Writer, name string, v any) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeFileToZip(zw *zip.Writer, name, path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

func fileMatchesPins(path string, wantBytes int64, wantSHA string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if wantBytes > 0 && info.Size() != wantBytes {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), wantSHA), nil
}

func downloadPinned(ctx context.Context, client *http.Client, url, dest string, wantBytes int64, wantSHA string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(f, h), resp.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if wantBytes > 0 && written != wantBytes {
		_ = os.Remove(tmp)
		return fmt.Errorf("got %d bytes, want %d", written, wantBytes)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, wantSHA) {
		_ = os.Remove(tmp)
		return fmt.Errorf("checksum mismatch: got %s", got)
	}
	return os.Rename(tmp, dest)
}
