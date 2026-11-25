package highlights

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/composer"
	"strikeye/internal/ui/theme"
)

// maxAudioTracks caps the editor rows: past three external tracks a montage
// turns into mud, and the mix gets slower than the render itself.
const maxAudioTracks = 3

// audioTrackDraft is one editable external audio track in the export card. Offset
// and volume are text so the input can hold a half-typed value ("-0.", "1.5")
// without rounding it away on every keystroke.
type audioTrackDraft struct {
	Path       string
	OffsetText string
	VolumeText string
	Mute       bool
}

// defaultAudioTracks returns the initial rows, all empty.
func defaultAudioTracks() []audioTrackDraft {
	return make([]audioTrackDraft, maxAudioTracks)
}

// toAudioTracks converts drafts into composer tracks, dropping rows without a
// path and normalising volumes: 0 means "leave at unity", above 4 is clamped.
func toAudioTracks(drafts []audioTrackDraft) []composer.AudioTrack {
	out := make([]composer.AudioTrack, 0, len(drafts))
	for _, d := range drafts {
		path := strings.TrimSpace(d.Path)
		if path == "" {
			continue
		}
		volume := parseTrackFloat(d.VolumeText, composer.DefaultTrackVolume)
		switch {
		case volume <= 0:
			volume = composer.DefaultTrackVolume
		case volume > 4:
			volume = 4
		}
		out = append(out, composer.AudioTrack{
			Path:      path,
			OffsetSec: parseTrackFloat(d.OffsetText, 0),
			Volume:    volume,
			Mute:      d.Mute,
		})
	}
	return out
}

// renderAudioTrackEditor draws the external-track rows. Each row is path, offset,
// volume and a mute switch; rows can be cleared by emptying the path.
func renderAudioTrackEditor(c *ui.Context, drafts *[]audioTrackDraft) {
	theme.MutedText(c, "额外音轨（语音 POV / 现场声）：偏移可为负（裁掉开头），音量 1 为原始大小")
	for i := range *drafts {
		draft := &(*drafts)[i]
		theme.RowCenter(c).FillWidth().Gap(6).Children(func() {
			ui.TextInput(c, &draft.Path).FillWidth().Placeholder("音轨文件路径")
			ui.TextInput(c, &draft.OffsetText).Width(80).Placeholder("偏移s")
			ui.TextInput(c, &draft.VolumeText).Width(70).Placeholder("音量")
			ui.Checkbox(c, &draft.Mute, "静音")
		})
	}
}

// parseTrackFloat parses a user-typed number, falling back to def for empty or
// malformed input so a typo never blocks the export.
func parseTrackFloat(raw string, def float64) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return def
	}
	return val
}
