package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
	"strikeye/internal/voice"
)

// voiceBarWidth is how many characters one round timeline occupies; 60 keeps a
// 40s round readable at 1 char ≈ 0.7s.
const voiceBarWidth = 60

// voiceRow is one speaker's line in the voice timeline.
type voiceRow struct {
	Label   string  `json:"label"`
	XUID    uint64  `json:"xuid"`
	Bar     string  `json:"bar"`
	Blocks  int     `json:"blocks"`
	Seconds float64 `json:"seconds"`
}

// voiceBar renders activity blocks as a fixed-width bar: █ while talking, · in
// silence. Blocks outside [startTick, endTick] are clipped, not dropped, so a
// sentence that starts before the round still shows up.
func voiceBar(blocks []voice.Activity, startTick, endTick int64, width int) string {
	if width <= 0 {
		width = voiceBarWidth
	}
	if endTick <= startTick {
		return strings.Repeat("·", width)
	}
	out := []rune(strings.Repeat("·", width))
	span := float64(endTick - startTick)
	for _, b := range blocks {
		lo := b.StartTick
		hi := b.EndTick
		if hi < startTick || lo > endTick {
			continue
		}
		if lo < startTick {
			lo = startTick
		}
		if hi > endTick {
			hi = endTick
		}
		first := int(float64(lo-startTick) / span * float64(width))
		last := int(float64(hi-startTick) / span * float64(width))
		if first < 0 {
			first = 0
		}
		if last >= width {
			last = width - 1
		}
		for i := first; i <= last; i++ {
			out[i] = '█'
		}
	}
	return string(out)
}

// voiceRows builds one row per speaker, ordered by talking time.
func voiceRows(blocks []voice.Activity, startTick, endTick int64, tickRate float64, names map[uint64]string, width int) []voiceRow {
	order, byXUID := voice.BySpeaker(voice.FilterRange(blocks, startTick, endTick))
	rows := make([]voiceRow, 0, len(order))
	for _, xuid := range order {
		list := byXUID[xuid]
		secs := 0.0
		if tickRate > 0 {
			for _, b := range list {
				secs += float64(b.EndTick-b.StartTick+1) / tickRate
			}
		}
		label := names[xuid]
		if label == "" {
			label = "XUID " + strconv.FormatUint(xuid, 10)
		}
		rows = append(rows, voiceRow{
			Label:   label,
			XUID:    xuid,
			Bar:     voiceBar(list, startTick, endTick, width),
			Blocks:  len(list),
			Seconds: secs,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Seconds > rows[j].Seconds })
	return rows
}

// voiceCapturePath resolves the .cs2voice for a demo, accepting both the
// canonical demo_<id> name and the older source-file name.
func voiceCapturePath(dataRoot string, demo *storage.Demo) string {
	dir := filepath.Join(dataRoot, "media", "voice")
	candidates := []string{filepath.Join(dir, fmt.Sprintf("demo_%d.cs2voice", demo.ID))}
	if demo.Filename != "" {
		candidates = append(candidates, filepath.Join(dir, demo.Filename+".cs2voice"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.Size() > 0 {
			return c
		}
	}
	return ""
}

// loadVoiceActivity reads the capture and groups it into speak bursts. It is
// best-effort: a demo without voice simply yields an empty timeline.
func loadVoiceActivity(dataRoot string, demo *storage.Demo) ([]voice.Activity, error) {
	if demo == nil {
		return nil, nil
	}
	path := voiceCapturePath(dataRoot, demo)
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	frames, err := voice.ReadContainer(f)
	if err != nil {
		return nil, err
	}
	return voice.BuildActivity(frames, voice.DefaultActivityJoinTicks), nil
}

// ensureVoiceActivityAsync loads voice activity once per demo, in the background.
func (v *View) ensureVoiceActivityAsync() {
	v.mu.RLock()
	demo := v.demo
	ready := v.voiceReady
	loading := v.voiceLoading
	v.mu.RUnlock()
	if demo == nil || ready || loading {
		return
	}
	v.mu.Lock()
	v.voiceLoading = true
	v.voiceSeq++
	seq := v.voiceSeq
	dataRoot := v.dataRoot
	demoCopy := *demo
	v.mu.Unlock()

	go func() {
		blocks, err := loadVoiceActivity(dataRoot, &demoCopy)
		v.mu.Lock()
		defer v.mu.Unlock()
		if seq != v.voiceSeq {
			return
		}
		v.voiceLoading = false
		v.voiceReady = true
		v.voiceErr = err
		v.voiceBlocks = blocks
	}()
}

// renderVoiceTimeline shows who talked when across the selected round.
func (v *View) renderVoiceTimeline(c *ui.Context) {
	v.mu.RLock()
	demo := v.demo
	blocks := v.voiceBlocks
	err := v.voiceErr
	ready := v.voiceReady
	loading := v.voiceLoading
	roundNum := v.replayRoundNum
	rounds := v.rounds
	players := v.players
	tickRate := 0.0
	if demo != nil {
		tickRate = demo.TickRate
	}
	v.mu.RUnlock()

	var startTick, endTick int64
	for _, r := range rounds {
		if int(r.RoundNumber) == roundNum {
			startTick, endTick = r.FreezeEndTick, r.EndTick
			if startTick <= 0 {
				startTick = r.StartTick
			}
			break
		}
	}

	theme.Card(c, func() {
		theme.SectionTitle(c, "语音时间轴")
		switch {
		case loading:
			theme.MutedText(c, "读取语音捕获…")
		case err != nil:
			theme.MutedText(c, fmt.Sprintf("语音读取失败：%v", err))
		case len(blocks) == 0:
			theme.MutedText(c, "本场没有语音捕获（Valve 匹配 demo 默认不录语音；POV demo 可用 `strikeye-cli voice` 提取）")
		case startTick <= 0 || endTick <= startTick:
			theme.MutedText(c, "选择回合后显示语音时间轴")
		default:
			names := map[uint64]string{}
			for _, p := range players {
				id, parseErr := strconv.ParseUint(p.SteamID64, 10, 64)
				if parseErr != nil {
					continue
				}
				name := p.PlayerName
				if p.DisplayName != "" {
					name = p.DisplayName
				}
				names[id] = name
			}
			rows := voiceRows(blocks, startTick, endTick, tickRate, names, voiceBarWidth)
			if len(rows) == 0 {
				theme.MutedText(c, "本回合没有语音活动")
				return
			}
			theme.MutedText(c, fmt.Sprintf("R%d · %d 人发言 · █ 说话 / · 静默", roundNum, len(rows)))
			for _, row := range rows {
				theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
					ui.Text(c, fmt.Sprintf("%-12s", truncateLabel(row.Label, 12))).Font("Consolas").FontSize(12).TextColor(theme.TextPrimary)
					ui.Text(c, row.Bar).Font("Consolas").FontSize(14).TextColor(theme.AccentAmber)
					theme.MutedText(c, fmt.Sprintf("%.1fs / %d 段", row.Seconds, row.Blocks))
				})
			}
			if !ready {
				theme.PillBadge(c, "刷新中", theme.AccentCyan)
			}
		}
	})
}

// truncateLabel keeps a name inside its column without cutting a wide glyph.
func truncateLabel(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}
