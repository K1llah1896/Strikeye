package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/gsi"
	"strikeye/internal/ui/theme"
)

// gsiConfigPath is where the CLI writes (and CS2 reads from) the integration
// config; the settings card writes the same file so nobody has to run the CLI.
func gsiConfigPath(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "gsi", "gamestate_integration_strikeye.cfg")
}

// gsiStatePath is the last-state mirror written by `strikeye-cli gsi`.
func gsiStatePath(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "gsi_latest.json")
}

// writeGSIConfig renders the KeyValues config for one listen address + token.
func writeGSIConfig(dataRoot, listen, token string) (string, error) {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		listen = "127.0.0.1:3000"
	}
	if !strings.Contains(listen, ":") {
		return "", fmt.Errorf("监听地址需要 host:port 形式")
	}
	path := gsiConfigPath(dataRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	body := gsi.Config("http://"+listen+"/", strings.TrimSpace(token))
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// loadGSIState reads the mirrored state, rejecting anything older than maxAge so
// a stale file from yesterday is not shown as "live".
func loadGSIState(dataRoot string, now time.Time, maxAge time.Duration) (gsi.State, bool) {
	data, err := os.ReadFile(gsiStatePath(dataRoot))
	if err != nil {
		return gsi.State{}, false
	}
	var state gsi.State
	if err := json.Unmarshal(data, &state); err != nil {
		return gsi.State{}, false
	}
	if state.ReceivedAt.IsZero() || now.Sub(state.ReceivedAt) > maxAge {
		return gsi.State{}, false
	}
	return state, true
}

// receiver exposes the in-process listener to the card.
func (v *View) receiverOf() *gsiReceiver {
	if v.gsiReceiver == nil {
		v.gsiReceiver = &gsiReceiver{}
	}
	return v.gsiReceiver
}

// Close stops the in-process GSI listener.
func (v *View) Close() {
	if v.gsiReceiver != nil {
		_ = v.gsiReceiver.stop()
	}
}

// renderGSICard draws the GSI setup + status card.
func (v *View) renderGSICard(c *ui.Context) {
	state, live := loadGSIState(v.dataRoot, time.Now(), 30*time.Second)
	theme.BentoCard(c, ui.Color{}, 12, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.RenderIcon(c, theme.IconRadar, 15, theme.StatusGreen)
			theme.SectionTitle(c, "CS2 实时数据（GSI）")
			ui.Spacer(c)
			if live {
				theme.PillBadge(c, "live", theme.StatusGreen)
			} else {
				theme.PillBadge(c, "无活动流", theme.TextMuted)
			}
		})
		theme.MutedText(c, "生成配置后复制到 CS2 的 game/csgo/cfg/ 目录；游戏内状态会写入 cache/gsi_latest.json，资料库顶部横幅据此显示实时比分。")
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			ui.TextInput(c, &v.gsiListen).Width(180).Placeholder("127.0.0.1:3000")
			ui.TextInput(c, &v.gsiToken).Width(200).Placeholder("token（可选）")
			if theme.ActionButton(c, "生成 GSI 配置").Clicked() {
				path, err := writeGSIConfig(v.dataRoot, v.gsiListen, v.gsiToken)
				if err != nil {
					v.setStatus("生成失败: " + err.Error())
				} else {
					v.setStatus("已写入 " + path)
				}
			}
			recv := v.receiverOf()
			if addr, running := recv.running(); running {
				if theme.ActionButton(c, "停止接收端").Clicked() {
					if err := recv.stop(); err != nil {
						v.setStatus(err.Error())
					} else {
						v.setStatus("接收端已停止（" + addr + "）")
					}
				}
			} else if theme.ActionButton(c, "启动内置接收端").Clicked() {
				addr, err := recv.start(v.gsiListen, strings.TrimSpace(v.gsiToken), v.dataRootOf())
				if err != nil {
					v.setStatus(err.Error())
				} else {
					v.setStatus("接收端已启动：" + addr + "（配置里的 uri 需指向同一地址）")
				}
			}
		})
		if live {
			theme.StatText(c, fmt.Sprintf("%s · 回合 %d · %d-%d · %s · 存活 %d",
				state.Map, state.Round, state.TeamScoreCT, state.TeamScoreT, state.RoundPhase, alivePlayers(state))).
				FontSize(12).TextColor(theme.TextPrimary)
		}
		if v.gsiMsg != "" {
			theme.MutedText(c, v.gsiMsg)
		}
	})
}

// alivePlayers counts players reporting health above zero.
func alivePlayers(state gsi.State) int {
	alive := 0
	for _, p := range state.Players {
		if p.Health > 0 {
			alive++
		}
	}
	return alive
}
