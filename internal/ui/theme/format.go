package theme

import (
	"encoding/json"
	"strings"
)

func lookupLabel(raw string, table map[string]string, fallback string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return fallback
	}
	if v, ok := table[key]; ok {
		return v
	}
	return raw
}

var parseStatusLabels = map[string]string{
	"ready":   "已解析",
	"parsing": "解析中",
	"pending": "待解析",
	"failed":  "解析失败",
}

// ParseStatusLabel maps a demo parse_status enum to a localized UI label.
func ParseStatusLabel(status string) string {
	return lookupLabel(status, parseStatusLabels, "待解析")
}

var buyTypeLabels = map[string]string{
	"full_buy":  "长枪局",
	"full":      "长枪局",
	"half_buy":  "半起局",
	"semi":      "半起局",
	"force_buy": "强起局",
	"force":     "强起局",
	"eco":       "Eco",
	"pistol":    "手枪局",
}

// BuyTypeLabel maps a round economy buy-type enum to a localized tactical label.
func BuyTypeLabel(buy string) string {
	return lookupLabel(buy, buyTypeLabels, "-")
}

var winReasonLabels = map[string]string{
	"elimination":       "全歼",
	"bomb_exploded":     "雷包引爆",
	"bomb_defused":      "拆除雷包",
	"time_ran_out":      "时间耗尽",
	"target_saved":      "保枪超时",
	"ct_win_rescue":     "人质救援",
	"t_win_elimination": "全歼",
}

// WinReasonLabel maps a round win-reason enum to a localized tactical label.
func WinReasonLabel(reason string) string {
	return lookupLabel(reason, winReasonLabels, "-")
}

var clipTypeLabels = map[string]string{
	"multi_kill":   "多杀",
	"clutch":       "残局",
	"ace":          "五杀",
	"awp":          "狙击",
	"entry":        "首杀突破",
	"utility":      "道具击杀",
	"highlight":    "高光",
	"wallbang":     "穿射",
	"no_scope":     "盲狙",
	"smoke_kill":   "混烟击杀",
	"flash_assist": "闪光助攻",
}

// ClipTypeLabel maps a highlight clip_type enum to a localized label.
func ClipTypeLabel(clipType string) string {
	return lookupLabel(clipType, clipTypeLabels, "高光")
}

// ParseTags decodes a JSON array string (e.g. `["4K","Headshot"]`) into clean tag labels,
// falling back to comma-separated splitting when the input is plain text.
func ParseTags(tagsJSON string) []string {
	trimmed := strings.TrimSpace(tagsJSON)
	if trimmed == "" || trimmed == "[]" || trimmed == "null" {
		return nil
	}
	var arr []string
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				if t := strings.TrimSpace(item); t != "" {
					out = append(out, t)
				}
			}
			return out
		}
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
