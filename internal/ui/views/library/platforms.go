package library

import (
	"strings"

	"strikeye/internal/watcher"
)

// allPlatformsOption is the sentinel shown as the first platform filter entry.
const allPlatformsOption = "全部平台"

// platformFilterOptions derives filter entries from the canonical source
// registry so every persisted platform value has a selectable option.
func platformFilterOptions() []string {
	ids := watcher.PlatformIDs()
	out := make([]string, 0, len(ids)+1)
	out = append(out, allPlatformsOption)
	out = append(out, ids...)
	return out
}

// platformMatches compares stored and selected platforms after normalization so
// legacy labels such as "perfectworld" still match "perfect_world".
func platformMatches(stored, selected string) bool {
	return strings.EqualFold(watcher.NormalizePlatform(stored), watcher.NormalizePlatform(selected))
}
