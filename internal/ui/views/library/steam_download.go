package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/gcpd"
	"strikeye/internal/ui/theme"
)

// steamDownloadTimeout bounds one download batch: the demo archives are 100–400MB
// each and Valve throttles.
const steamDownloadTimeout = 30 * time.Minute

// steamCookiePath is where the pasted Cookie header is kept. It lives beside the
// GSI state rather than in config.json so a config backup or sync never carries
// a session credential.
func steamCookiePath(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "gcpd_cookie.txt")
}

// saveSteamCookie stores the pasted cookie header, trimming whitespace.
func saveSteamCookie(dataRoot, cookie string) error {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return fmt.Errorf("cookie 为空")
	}
	if err := os.MkdirAll(filepath.Dir(steamCookiePath(dataRoot)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(steamCookiePath(dataRoot), []byte(cookie), 0o600)
}

// loadSteamCookie reads a previously saved cookie header.
func loadSteamCookie(dataRoot string) string {
	data, err := os.ReadFile(steamCookiePath(dataRoot))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// steamDownloadResult summarises one batch download.
type steamDownloadResult struct {
	Tab        string   `json:"tab"`
	Listed     int      `json:"listed"`
	Downloaded []string `json:"downloaded"`
	Skipped    int      `json:"skipped"`
	Failed     []string `json:"failed"`
	OutDir     string   `json:"out_dir"`
}

// runSteamDownload lists the given tab and downloads up to limit new demos into
// <DataRoot>/media/imports, which the watcher then parses.
func runSteamDownload(ctx context.Context, dataRoot, cookie, tab string, limit int) (steamDownloadResult, error) {
	res := steamDownloadResult{Tab: tab}
	if strings.TrimSpace(cookie) == "" {
		return res, fmt.Errorf("需要 Steam Cookie")
	}
	outDir := filepath.Join(dataRoot, "media", "imports")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return res, fmt.Errorf("create import dir: %w", err)
	}
	client := gcpd.NewClient(cookie)
	// Mirrors the CLI: tests and self-hosted mirrors override the base URL.
	if base := strings.TrimSpace(os.Getenv("STRIKEYE_GCPD_BASE")); base != "" {
		client.BaseURL = base
	}
	matches, err := client.List(ctx, tab)
	if err != nil {
		return res, err
	}
	res.Listed = len(matches)
	res.OutDir = outDir
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	for _, m := range matches {
		dest := filepath.Join(outDir, m.Filename)
		if st, statErr := os.Stat(dest); statErr == nil && st.Size() > 0 {
			res.Skipped++
			continue
		}
		if _, err := client.Download(ctx, m, outDir); err != nil {
			res.Failed = append(res.Failed, fmt.Sprintf("%s: %v", m.Filename, err))
			continue
		}
		res.Downloaded = append(res.Downloaded, m.Filename)
	}
	return res, nil
}

// renderSteamDownload draws the download panel that turns a pasted cookie into
// freshly imported demos.
func (v *View) renderSteamDownload(c *ui.Context) {
	if !v.showSteamDownload {
		return
	}
	theme.Card(c, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.RenderIcon(c, theme.IconDownload, 15, theme.AccentCyan)
			theme.SectionTitle(c, "从 Steam 下载比赛")
			ui.Spacer(c)
			if theme.ActionButton(c, "关闭").Clicked() {
				v.showSteamDownload = false
			}
		})
		theme.MutedText(c, "在浏览器打开 steamcommunity.com 并登录后，把整个 Cookie 请求头粘贴到下面（只保存在本机 cache/ 下，权限 0600）。下载的 .bz2 会进入 media/imports 并自动解析。")
		ui.TextInput(c, &v.steamCookieDraft).FillWidth().Placeholder("Cookie: steamLoginSecure=...; sessionid=...")
		v.renderSharecodeImport(c)
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.Select(c, &v.steamTab, gcpd.Tabs).Width(240)
			if theme.ActionButton(c, "下载并导入").Clicked() && !v.steamBusy {
				if err := saveSteamCookie(v.dataRoot, v.steamCookieDraft); err != nil {
					v.statusMsg = "保存 Cookie 失败: " + err.Error()
					return
				}
				v.steamBusy = true
				v.statusMsg = "正在从 Steam 拉取比赛列表..."
				go v.runSteamDownloadAsync()
			}
			if v.steamBusy {
				theme.PillBadge(c, "下载中...", theme.AccentAmber)
			}
			ui.Spacer(c)
			if theme.ActionButton(c, "扫描并刷新").Clicked() {
				go v.scanAndRefresh(context.Background())
			}
		})
	})
}

// runSteamDownloadAsync runs the batch off the render thread and reuses the
// status line for progress.
func (v *View) runSteamDownloadAsync() {
	ctx, cancel := context.WithTimeout(context.Background(), steamDownloadTimeout)
	defer cancel()
	res, err := runSteamDownload(ctx, v.dataRoot, loadSteamCookie(v.dataRoot), v.steamTab, v.steamLimit)
	v.mu.Lock()
	v.steamBusy = false
	if err != nil {
		v.statusMsg = "下载失败: " + err.Error()
	} else {
		v.statusMsg = fmt.Sprintf("已下载 %d 场（跳过 %d，失败 %d）到 %s",
			len(res.Downloaded), res.Skipped, len(res.Failed), res.OutDir)
	}
	v.mu.Unlock()
	_ = v.Refresh(context.Background())
}
