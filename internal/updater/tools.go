package updater

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ToolHLAE   = "hlae"
	ToolFFmpeg = "ffmpeg"

	defaultHLAEReleaseAPI   = "https://api.github.com/repos/advancedfx/advancedfx/releases/latest"
	defaultFFmpegReleaseAPI = "https://api.github.com/repos/BtbN/FFmpeg-Builds/releases/latest"
	fallbackFFmpegZipURL    = "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"
)

// InstallToolOptions configures downloading and extracting an external tool.
type InstallToolOptions struct {
	Tool       string
	DataRoot   string
	ReleaseAPI string // optional override for tests
}

// InstallToolResult reports the installed binary paths under <DataRoot>/tools.
type InstallToolResult struct {
	HLAEExe           string
	AfxHookSource2DLL string
	FFmpegExe         string
	FFprobeExe        string
}

// InstallTool downloads the official archive for opts.Tool and extracts it into
// <DataRoot>/tools/<tool>/ while reporting progress to onProgress.
func InstallTool(
	ctx context.Context,
	client *http.Client,
	opts InstallToolOptions,
	onProgress func(stage string, progress float64),
) (InstallToolResult, error) {
	if strings.TrimSpace(opts.DataRoot) == "" {
		return InstallToolResult{}, errors.New("DataRoot 不能为空")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	report := func(stage string, p float64) {
		if onProgress != nil {
			onProgress(stage, p)
		}
	}

	switch opts.Tool {
	case ToolHLAE:
		return installHLAE(ctx, client, opts, report)
	case ToolFFmpeg:
		return installFFmpeg(ctx, client, opts, report)
	default:
		return InstallToolResult{}, fmt.Errorf("不支持的工具类型: %s", opts.Tool)
	}
}

func installHLAE(ctx context.Context, client *http.Client, opts InstallToolOptions, report func(string, float64)) (InstallToolResult, error) {
	apiURL := strings.TrimSpace(opts.ReleaseAPI)
	if apiURL == "" {
		apiURL = defaultHLAEReleaseAPI
	}
	report("正在获取 HLAE 最新版本信息...", 0.05)
	zipURL, err := resolveGitHubZipAsset(ctx, client, apiURL, func(name string) bool {
		l := strings.ToLower(name)
		return strings.HasSuffix(l, ".zip") && strings.Contains(l, "hlae") && !strings.Contains(l, "symbol") && !strings.Contains(l, "pdb")
	})
	if err != nil {
		return InstallToolResult{}, fmt.Errorf("获取 HLAE 下载地址失败: %w", err)
	}

	destDir := filepath.Join(filepath.Clean(opts.DataRoot), "tools", "hlae")
	if err := downloadAndExtractZip(ctx, client, zipURL, destDir, "HLAE", report); err != nil {
		return InstallToolResult{}, err
	}

	var res InstallToolResult
	_ = filepath.WalkDir(destDir, func(path string, d os.DirEntry, wErr error) error {
		if wErr != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(d.Name()) {
		case "hlae.exe":
			res.HLAEExe = path
		case "afxhooksource2.dll":
			res.AfxHookSource2DLL = path
		}
		return nil
	})
	if res.HLAEExe == "" {
		return res, errors.New("解压完成但未找到 HLAE.exe")
	}
	report("HLAE 安装完成", 1.0)
	return res, nil
}

func installFFmpeg(ctx context.Context, client *http.Client, opts InstallToolOptions, report func(string, float64)) (InstallToolResult, error) {
	apiURL := strings.TrimSpace(opts.ReleaseAPI)
	if apiURL == "" {
		apiURL = defaultFFmpegReleaseAPI
	}
	report("正在获取 FFmpeg 最新版本信息...", 0.05)
	zipURL, err := resolveGitHubZipAsset(ctx, client, apiURL, func(name string) bool {
		l := strings.ToLower(name)
		return strings.HasSuffix(l, ".zip") && strings.Contains(l, "win64") && strings.Contains(l, "gpl") && !strings.Contains(l, "lgpl") && !strings.Contains(l, "shared")
	})
	if err != nil {
		if opts.ReleaseAPI != "" {
			return InstallToolResult{}, fmt.Errorf("获取 FFmpeg 下载地址失败: %w", err)
		}
		zipURL = fallbackFFmpegZipURL
	}

	destDir := filepath.Join(filepath.Clean(opts.DataRoot), "tools", "ffmpeg")
	if err := downloadAndExtractZip(ctx, client, zipURL, destDir, "FFmpeg", report); err != nil {
		return InstallToolResult{}, err
	}

	var res InstallToolResult
	_ = filepath.WalkDir(destDir, func(path string, d os.DirEntry, wErr error) error {
		if wErr != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(d.Name()) {
		case "ffmpeg.exe", "ffmpeg":
			if res.FFmpegExe == "" || strings.HasSuffix(strings.ToLower(d.Name()), ".exe") {
				res.FFmpegExe = path
			}
		case "ffprobe.exe", "ffprobe":
			if res.FFprobeExe == "" || strings.HasSuffix(strings.ToLower(d.Name()), ".exe") {
				res.FFprobeExe = path
			}
		}
		return nil
	})
	if res.FFmpegExe == "" || res.FFprobeExe == "" {
		return res, errors.New("解压完成但未找到完整的 ffmpeg / ffprobe 可执行文件")
	}
	report("FFmpeg & FFprobe 安装完成", 1.0)
	return res, nil
}

func resolveGitHubZipAsset(ctx context.Context, client *http.Client, apiURL string, match func(name string) bool) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Strikeye-ToolInstaller")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&rel); err != nil {
		return "", err
	}
	for _, a := range rel.Assets {
		if match(a.Name) && strings.TrimSpace(a.BrowserDownloadURL) != "" {
			return a.BrowserDownloadURL, nil
		}
	}
	// Fallback: any .zip asset in release
	for _, a := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".zip") && strings.TrimSpace(a.BrowserDownloadURL) != "" {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", errors.New("未在 Release 中找到匹配的 zip 资源")
}

func downloadAndExtractZip(ctx context.Context, client *http.Client, zipURL, destDir, label string, report func(string, float64)) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("创建工具目录失败: %w", err)
	}
	tmpFile, err := os.CreateTemp(destDir, ".download-*.zip")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zipURL, nil)
	if err != nil {
		_ = tmpFile.Close()
		return err
	}
	req.Header.Set("User-Agent", "Strikeye-ToolInstaller")
	resp, err := client.Do(req)
	if err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("下载 %s 失败: %w", label, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = tmpFile.Close()
		return fmt.Errorf("下载 %s 返回 HTTP %d", label, resp.StatusCode)
	}

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 64*1024)
	lastReport := time.Time{}

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := tmpFile.Write(buf[:n]); wErr != nil {
				_ = tmpFile.Close()
				return wErr
			}
			written += int64(n)
			if lastReport.IsZero() || time.Since(lastReport) >= 120*time.Millisecond {
				lastReport = time.Now()
				p := 0.45
				if total > 0 {
					p = 0.08 + 0.72*(float64(written)/float64(total))
				}
				report(fmt.Sprintf("正在下载 %s (%.1f MB)...", label, float64(written)/(1024*1024)), p)
			}
		}
		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			_ = tmpFile.Close()
			return rErr
		}
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	report(fmt.Sprintf("正在解压 %s...", label), 0.86)
	return extractZipSafe(tmpPath, destDir)
}

func extractZipSafe(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开压缩包失败: %w", err)
	}
	defer zr.Close()

	cleanDest := filepath.Clean(destDir)
	prefix := cleanDest + string(os.PathSeparator)

	for _, f := range zr.File {
		target := filepath.Clean(filepath.Join(cleanDest, f.Name))
		if target != cleanDest && !strings.HasPrefix(target, prefix) {
			return fmt.Errorf("检测到非法压缩包路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		_ = rc.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
