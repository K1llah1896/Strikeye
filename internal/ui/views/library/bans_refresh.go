package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
	"strikeye/internal/vac"
)

// steamAPIKeyPath holds the Steam Web API key. Like the GCPD cookie it lives in
// cache/ with 0600 so a config backup never carries a credential.
func steamAPIKeyPath(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "steam_api_key.txt")
}

// saveSteamAPIKey stores the key, trimming whitespace.
func saveSteamAPIKey(dataRoot, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("API key 为空")
	}
	if err := os.MkdirAll(filepath.Dir(steamAPIKeyPath(dataRoot)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(steamAPIKeyPath(dataRoot), []byte(key), 0o600)
}

// loadSteamAPIKey prefers the environment so a power user can keep the key out
// of the DataRoot entirely.
func loadSteamAPIKey(dataRoot string) string {
	if env := strings.TrimSpace(os.Getenv("STRIKEYE_STEAM_API_KEY")); env != "" {
		return env
	}
	data, err := os.ReadFile(steamAPIKeyPath(dataRoot))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// banRefreshSummary reports what a refresh pass did.
type banRefreshSummary struct {
	Checked   int      `json:"checked"`
	Refreshed int      `json:"refreshed"`
	Skipped   int      `json:"skipped"`
	Banned    []string `json:"banned"`
}

// refreshBans looks up every distinct player in the library and reports who is
// banned. Without an API key the cached rows are still read, so the badge stays
// correct offline.
func refreshBans(ctx context.Context, db *storage.DB, apiKey string, refresh bool) (banRefreshSummary, error) {
	var sum banRefreshSummary
	if db == nil {
		return sum, fmt.Errorf("storage unavailable")
	}
	seen := map[string]bool{}
	var ids []string
	demos, err := db.Demos().List(ctx)
	if err != nil {
		return sum, err
	}
	for i := range demos {
		if demos[i].ParseStatus != "parsed" {
			continue
		}
		players, err := db.Analysis().ListMatchPlayers(ctx, demos[i].ID)
		if err != nil {
			continue
		}
		for _, p := range players {
			id := strings.TrimSpace(p.SteamID64)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return sum, fmt.Errorf("资料库里还没有已解析比赛的选手")
	}
	svc := vac.NewService(db, apiKey)
	results, err := svc.Lookup(ctx, ids, refresh)
	if err != nil {
		return sum, err
	}
	sum.Checked = len(results)
	for _, r := range results {
		switch r.Cache {
		case "refreshed":
			sum.Refreshed++
		case "skipped":
			sum.Skipped++
		}
		if r.Ban.Risk() {
			sum.Banned = append(sum.Banned, fmt.Sprintf("%s(%s)", r.Ban.SteamID, r.Ban.RiskLabel()))
		}
	}
	return sum, nil
}

// ensureBansAsync refreshes stale ban snapshots dropped away from the UI: when a
// Steam API key is available, the library keeps the badges current without the
// user clicking the shield button. Without a key it does nothing at all.
func (v *View) ensureBansAsync() {
	if v.db == nil {
		return
	}
	key := loadSteamAPIKey(v.dataRoot)
	if key == "" {
		return
	}
	v.mu.Lock()
	if v.bansBusy {
		v.mu.Unlock()
		return
	}
	v.bansBusy = true
	v.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		// refresh=false keeps the service's 24h TTL: only stale or unknown
		// players cost a request.
		_, _ = refreshBans(ctx, v.db, key, false)
		v.mu.Lock()
		v.bansBusy = false
		v.mu.Unlock()
		_ = v.Refresh(context.Background())
	}()
}

// runBanRefreshNow is the goroutine body behind the toolbar button.
func (v *View) runBanRefreshNow() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	key := loadSteamAPIKey(v.dataRoot)
	sum, err := refreshBans(ctx, v.db, key, true)
	v.mu.Lock()
	if err != nil {
		v.statusMsg = "封禁查询失败: " + err.Error()
	} else if len(sum.Banned) == 0 {
		v.statusMsg = fmt.Sprintf("已检查 %d 名选手，无封禁（刷新 %d，跳过 %d）", sum.Checked, sum.Refreshed, sum.Skipped)
	} else {
		v.statusMsg = fmt.Sprintf("已检查 %d 名选手，%d 人有封禁: %s", sum.Checked, len(sum.Banned), strings.Join(sum.Banned, " "))
	}
	v.mu.Unlock()
	_ = v.Refresh(context.Background())
}

// renderAPIKeyPanel asks for a Steam Web API key when a ban refresh was
// requested without one, then re-runs the refresh on save.
func (v *View) renderAPIKeyPanel(c *ui.Context) {
	if !v.showAPIKeyPrompt {
		return
	}
	theme.Card(c, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.RenderIcon(c, theme.IconShield, 15, theme.AccentAmber)
			theme.SectionTitle(c, "Steam Web API Key")
			ui.Spacer(c)
			if theme.ActionButton(c, "关闭").Clicked() {
				v.showAPIKeyPrompt = false
			}
		})
		theme.MutedText(c, "封禁查询需要 Key（steamcommunity.com/dev/apikey）。只保存在本机 cache/steam_api_key.txt，权限 0600；也可以用 STRIKEYE_STEAM_API_KEY 环境变量。")
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.TextInput(c, &v.apiKeyDraft).FillWidth().Placeholder("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
			if theme.AccentButton(c, "保存并查询").Clicked() && strings.TrimSpace(v.apiKeyDraft) != "" {
				if err := saveSteamAPIKey(v.dataRoot, v.apiKeyDraft); err != nil {
					v.statusMsg = "保存 Key 失败: " + err.Error()
					return
				}
				v.showAPIKeyPrompt = false
				v.statusMsg = "Key 已保存，正在查询封禁状态..."
				go v.runBanRefreshNow()
			}
		})
	})
}
