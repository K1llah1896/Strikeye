package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/gcpd"
	"strikeye/internal/sharecode"
	"strikeye/internal/ui/theme"
)

// sharecodeImportResult reports one share-code lookup.
type sharecodeImportResult struct {
	MatchID   uint64 `json:"match_id"`
	ShareCode string `json:"share_code"`
	Tab       string `json:"tab"`
	Found     bool   `json:"found"`
	Path      string `json:"path,omitempty"`
	Skipped   bool   `json:"skipped"`
}

// importSharecode resolves a share code to its match id, finds that match in the
// user's match history (any tab) and downloads it. The share code is the only
// handle most players ever exchange, so it deserves a first-class input.
func importSharecode(ctx context.Context, dataRoot, cookie, code string) (sharecodeImportResult, error) {
	match, err := sharecode.Decode(code)
	if err != nil {
		return sharecodeImportResult{}, err
	}
	res := sharecodeImportResult{MatchID: match.MatchID, ShareCode: match.ShareCode}
	if strings.TrimSpace(cookie) == "" {
		return res, fmt.Errorf("需要 Steam Cookie（分享码需要从 match history 定位比赛）")
	}
	outDir := filepath.Join(dataRoot, "media", "imports")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return res, fmt.Errorf("create import dir: %w", err)
	}
	client := gcpd.NewClient(cookie)
	if base := strings.TrimSpace(os.Getenv("STRIKEYE_GCPD_BASE")); base != "" {
		client.BaseURL = base
	}
	for _, tab := range gcpd.Tabs {
		matches, err := client.List(ctx, tab)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if m.MatchID != match.MatchID {
				continue
			}
			res.Found = true
			res.Tab = tab
			dest := filepath.Join(outDir, m.Filename)
			if st, statErr := os.Stat(dest); statErr == nil && st.Size() > 0 {
				res.Skipped = true
				res.Path = dest
				return res, nil
			}
			path, err := client.Download(ctx, m, outDir)
			if err != nil {
				return res, err
			}
			res.Path = path
			return res, nil
		}
	}
	return res, fmt.Errorf("分享码对应的比赛不在你的 match history 里（可能需要先在游戏里下载过该场，或 Cookie 已过期）")
}

// renderSharecodeImport draws the share-code row inside the Steam download panel.
func (v *View) renderSharecodeImport(c *ui.Context) {
	theme.RowCenter(c).FillWidth().Gap(6).Children(func() {
		ui.TextInput(c, &v.sharecodeDraft).FillWidth().Placeholder("分享码 CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx")
		if theme.ActionButton(c, "按分享码导入").Clicked() && !v.steamBusy {
			code := strings.TrimSpace(v.sharecodeDraft)
			if code == "" {
				return
			}
			if err := saveSteamCookie(v.dataRoot, v.steamCookieDraft); err != nil && strings.TrimSpace(v.steamCookieDraft) != "" {
				v.statusMsg = "保存 Cookie 失败: " + err.Error()
				return
			}
			v.steamBusy = true
			v.statusMsg = "正在用分享码定位比赛..."
			go v.runSharecodeImport(code)
		}
	})
}

// runSharecodeImport is the goroutine body behind the share-code button.
func (v *View) runSharecodeImport(code string) {
	ctx, cancel := context.WithTimeout(context.Background(), steamDownloadTimeout)
	defer cancel()
	res, err := importSharecode(ctx, v.dataRoot, loadSteamCookie(v.dataRoot), code)
	v.mu.Lock()
	v.steamBusy = false
	switch {
	case err != nil:
		v.statusMsg = "分享码导入失败: " + err.Error()
	case res.Skipped:
		v.statusMsg = "该场已导入: " + filepath.Base(res.Path)
	default:
		v.statusMsg = fmt.Sprintf("已下载比赛 %d（%s）到 %s", res.MatchID, res.Tab, filepath.Base(res.Path))
	}
	v.mu.Unlock()
	_ = v.Refresh(context.Background())
}
