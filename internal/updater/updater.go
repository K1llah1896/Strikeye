// Package updater implements in-app version checking, SHA-256 verified package
// downloading, and atomic Windows/Unix hot-update binary replacement.
package updater

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
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultGitHubReleasesURL points to the official Strikeye GitHub Releases API endpoint.
const DefaultGitHubReleasesURL = "https://api.github.com/repos/K1llah1896/Strikeye/releases/latest"

// Asset describes one platform-specific downloadable release artifact.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
}

// Manifest is the remote JSON update descriptor (custom or GitHub Release).
type Manifest struct {
	Version     string           `json:"version"`
	Notes       string           `json:"notes,omitempty"`
	PublishedAt string           `json:"published_at,omitempty"`
	Assets      map[string]Asset `json:"assets"`
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"`
	} `json:"assets"`
}

// CheckResult summarizes an update check against a remote Manifest.
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	HasUpdate      bool
	Notes          string
	Asset          Asset
}

// PlatformKey returns the manifest asset key for the current OS/architecture.
func PlatformKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// CompareVersions compares two semver-like strings (e.g. "0.1.0-phase1" vs "0.2.0").
// Returns -1 if a < b, 0 if equal, +1 if a > b.
func CompareVersions(a, b string) int {
	pa, preA := parseSemver(a)
	pb, preB := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	if preA == preB {
		return 0
	}
	// Release (empty prerelease) is newer than any prerelease of the same numeric version.
	if preA == "" {
		return 1
	}
	if preB == "" {
		return -1
	}
	return strings.Compare(preA, preB)
}

func parseSemver(raw string) ([3]int, string) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	pre := ""
	if idx := strings.IndexAny(s, "-+"); idx >= 0 {
		pre = s[idx+1:]
		s = s[:idx]
	}
	var out [3]int
	parts := strings.Split(s, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		if n, err := strconv.Atoi(parts[i]); err == nil {
			out[i] = n
		}
	}
	return out, pre
}

// CheckUpdate fetches the GitHub Release (or DefaultGitHubReleasesURL when empty)
// and determines whether a newer build exists for PlatformKey().
func CheckUpdate(ctx context.Context, client *http.Client, releaseURL, currentVersion string) (CheckResult, error) {
	u := strings.TrimSpace(releaseURL)
	if u == "" {
		u = DefaultGitHubReleasesURL
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return CheckResult{}, fmt.Errorf("构造更新请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Strikeye-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return CheckResult{}, fmt.Errorf("检查更新失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return CheckResult{}, fmt.Errorf("GitHub 更新接口返回状态 %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return CheckResult{}, fmt.Errorf("读取更新响应失败: %w", err)
	}

	var gh githubRelease
	if err := json.Unmarshal(raw, &gh); err == nil && strings.TrimSpace(gh.TagName) != "" {
		ver := strings.TrimPrefix(strings.TrimSpace(gh.TagName), "v")
		res := CheckResult{
			CurrentVersion: currentVersion,
			LatestVersion:  ver,
			Notes:          strings.TrimSpace(gh.Body),
		}
		if CompareVersions(currentVersion, ver) >= 0 {
			return res, nil
		}
		asset, ok := matchGitHubAsset(gh)
		if !ok {
			return CheckResult{}, fmt.Errorf("GitHub Release (%s) 未包含当前平台 (%s) 的安装包", gh.TagName, PlatformKey())
		}
		res.HasUpdate = true
		res.Asset = asset
		return res, nil
	}

	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return CheckResult{}, fmt.Errorf("解析更新信息失败: %w", err)
	}
	if strings.TrimSpace(m.Version) == "" {
		return CheckResult{}, errors.New("更新信息缺少 version 字段")
	}
	res := CheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  m.Version,
		Notes:          m.Notes,
	}
	if CompareVersions(currentVersion, m.Version) >= 0 {
		return res, nil
	}
	asset, ok := m.Assets[PlatformKey()]
	if !ok || strings.TrimSpace(asset.URL) == "" {
		return CheckResult{}, fmt.Errorf("更新清单未包含当前平台 (%s) 的安装包", PlatformKey())
	}
	res.HasUpdate = true
	res.Asset = asset
	return res, nil
}

func matchGitHubAsset(gh githubRelease) (Asset, bool) {
	goos := strings.ToLower(runtime.GOOS)
	goarch := strings.ToLower(runtime.GOARCH)
	var fallback Asset
	for _, a := range gh.Assets {
		name := strings.ToLower(a.Name)
		if !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".exe") {
			continue
		}
		sha := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(a.Digest)), "sha256:")
		cand := Asset{URL: a.BrowserDownloadURL, SHA256: sha}
		if strings.Contains(name, goos) && strings.Contains(name, goarch) {
			return cand, true
		}
		if goos == "windows" && (strings.HasSuffix(name, ".exe") || strings.Contains(name, "win")) {
			fallback = cand
		}
	}
	if fallback.URL != "" {
		return fallback, true
	}
	return Asset{}, false
}

// DownloadAndApply downloads asset, verifies optional SHA-256, and atomically replaces
// binaries in targetDir using .old backups so running Windows executables can hot-swap.
func DownloadAndApply(
	ctx context.Context,
	client *http.Client,
	asset Asset,
	targetDir string,
	onProgress func(stage string, progress float64),
) error {
	if strings.TrimSpace(asset.URL) == "" {
		return errors.New("安装包 URL 为空")
	}
	if strings.TrimSpace(targetDir) == "" {
		return errors.New("目标目录不能为空")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	report := func(stage string, p float64) {
		if onProgress != nil {
			onProgress(stage, p)
		}
	}

	report("正在连接更新服务器...", 0.05)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载更新包失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载服务器返回状态 %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp(targetDir, ".strikeye-update-*")
	if err != nil {
		return fmt.Errorf("创建临时更新文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	hasher := sha256.New()
	mw := io.MultiWriter(tmpFile, hasher)
	total := resp.ContentLength
	var written int64
	buf := make([]byte, 64*1024)
	lastReport := time.Time{}

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := mw.Write(buf[:n]); wErr != nil {
				_ = tmpFile.Close()
				return fmt.Errorf("写入临时文件失败: %w", wErr)
			}
			written += int64(n)
			if total > 0 && (lastReport.IsZero() || time.Since(lastReport) >= 100*time.Millisecond) {
				lastReport = time.Now()
				p := 0.05 + 0.70*(float64(written)/float64(total))
				report(fmt.Sprintf("正在下载更新包 (%.1f MB)...", float64(written)/(1024*1024)), p)
			}
		}
		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("读取更新流失败: %w", rErr)
		}
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if expected := strings.ToLower(strings.TrimSpace(asset.SHA256)); expected != "" {
		report("正在校验 SHA-256 签名...", 0.80)
		actual := hex.EncodeToString(hasher.Sum(nil))
		if actual != expected {
			return fmt.Errorf("SHA-256 校验失败: got %s, want %s", actual, expected)
		}
	}

	report("正在热替换可执行文件...", 0.88)
	lowerURL := strings.ToLower(asset.URL)
	if strings.HasSuffix(lowerURL, ".zip") || isZipFile(tmpPath) {
		if err := applyZipPackage(tmpPath, targetDir); err != nil {
			return err
		}
	} else {
		mainBin := "strikeye"
		if runtime.GOOS == "windows" {
			mainBin = "strikeye.exe"
		}
		if err := replaceBinaryFromFile(tmpPath, filepath.Join(targetDir, mainBin)); err != nil {
			return err
		}
	}

	report("热更新完成，请重启应用", 1.0)
	return nil
}

func isZipFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [4]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return false
	}
	return hdr[0] == 'P' && hdr[1] == 'K' && hdr[2] == 3 && hdr[3] == 4
}

func applyZipPackage(zipPath, targetDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开更新压缩包失败: %w", err)
	}
	defer zr.Close()

	allowed := map[string]bool{
		"strikeye.exe":       true,
		"strikeye-agent.exe": true,
		"strikeye-cli.exe":   true,
		"strikeye":           true,
		"strikeye-agent":     true,
		"strikeye-cli":       true,
	}
	// Data sidecars shipped next to the executables: the bundled collision
	// geometry archive and the FFmpeg/HLAE archives. They are not binaries, so
	// they are staged with 0644 and an older release without them simply leaves
	// the existing copies in place.
	dataFiles := map[string]bool{
		"map_geometry.zip":             true,
		"ffmpeg-windows-amd64.tar.zst": true,
		"ffmpeg-windows-amd64.zip":     true,
		"hlae-windows-amd64.zip":       true,
	}

	replaced := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if !allowed[base] && !dataFiles[base] {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		stageFile, err := os.CreateTemp(targetDir, ".stage-"+base+"-*")
		if err != nil {
			_ = rc.Close()
			return err
		}
		stagePath := stageFile.Name()
		_, copyErr := io.Copy(stageFile, rc)
		_ = rc.Close()
		_ = stageFile.Close()
		if copyErr != nil {
			_ = os.Remove(stagePath)
			return copyErr
		}
		dst := filepath.Join(targetDir, base)
		if dataFiles[base] {
			err = copyFile(stagePath, dst, 0o644)
		} else {
			err = replaceBinaryFromFile(stagePath, dst)
		}
		_ = os.Remove(stagePath)
		if err != nil {
			return err
		}
		replaced++
	}
	if replaced == 0 {
		return errors.New("更新包中未找到有效的 Strikeye 可执行文件")
	}
	return nil
}

func replaceBinaryFromFile(srcPath, dstPath string) error {
	oldPath := dstPath + ".old"
	_ = os.Remove(oldPath)

	hadExisting := false
	if _, err := os.Stat(dstPath); err == nil {
		hadExisting = true
		if err := os.Rename(dstPath, oldPath); err != nil {
			return fmt.Errorf("备份旧文件 %s 失败: %w", filepath.Base(dstPath), err)
		}
	}

	if err := copyFile(srcPath, dstPath, 0o755); err != nil {
		if hadExisting {
			_ = os.Rename(oldPath, dstPath)
		}
		return fmt.Errorf("写入新文件 %s 失败: %w", filepath.Base(dstPath), err)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// CleanupOldBinaries removes leftover *.old backup executables from previous hot updates.
func CleanupOldBinaries(dir string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".old") {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// RelaunchCurrentExecutable starts a fresh copy of the current executable path.
func RelaunchCurrentExecutable(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	return cmd.Start()
}
