package analysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/profile"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
)

// profileDraft holds the in-progress profile edit for the expanded player. It is
// keyed by SteamID so switching rows does not carry text between players.
type profileDraft struct {
	SteamID     string
	DisplayName string
	Note        string
	Tags        string
	Loaded      bool
	Saved       string
}

// loadProfileDraft seeds the draft from storage the first time a row expands.
func (v *View) loadProfileDraft(steamID string) {
	v.mu.RLock()
	draft := v.profileDraft
	db := v.db
	v.mu.RUnlock()
	if db == nil || draft.Loaded && draft.SteamID == steamID {
		return
	}
	next := profileDraft{SteamID: steamID}
	if p, found, err := db.PlayerProfiles().Get(context.Background(), steamID); err == nil && found {
		next.DisplayName = p.DisplayName
		next.Note = p.Note
		next.Tags = strings.Join(p.Tags, ", ")
	}
	next.Loaded = true
	v.mu.Lock()
	v.profileDraft = next
	v.mu.Unlock()
}

// saveProfileDraft persists the draft through profile.Save so the CLI and the UI
// share one write path.
func (v *View) saveProfileDraft(steamID string) error {
	if v.db == nil {
		return fmt.Errorf("storage unavailable")
	}
	v.mu.RLock()
	draft := v.profileDraft
	v.mu.RUnlock()
	tags := storage.ParseTagList(draft.Tags)
	_, err := profile.Save(context.Background(), v.db, steamID, draft.DisplayName, draft.Note, tags)
	if err != nil {
		return err
	}
	v.mu.Lock()
	draft.Saved = "已保存"
	v.profileDraft = draft
	v.mu.Unlock()
	return nil
}

// renderProfileEditor draws the profile editor row inside the player drawer.
func (v *View) renderProfileEditor(c *ui.Context, p storage.MatchPlayer) {
	v.loadProfileDraft(p.SteamID64)
	v.mu.RLock()
	draft := v.profileDraft
	v.mu.RUnlock()
	if draft.SteamID != p.SteamID64 {
		// Storage was unavailable; show a read-only hint instead of stale text.
		theme.MutedText(c, "档案编辑不可用（存储未就绪）")
		return
	}

	theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, "档案").FontSize(12).Bold().TextColor(theme.TextMuted)
		ui.TextInput(c, &v.profileDraft.DisplayName).Width(140).Placeholder("显示名")
		ui.TextInput(c, &v.profileDraft.Note).FillWidth().Placeholder("备注：角色、习惯、需要盯的点")
		ui.TextInput(c, &v.profileDraft.Tags).Width(160).Placeholder("标签（逗号分隔）")
		if theme.AccentButton(c, "保存档案").Clicked() {
			if err := v.saveProfileDraft(p.SteamID64); err != nil {
				v.mu.Lock()
				v.profileDraft.Saved = "保存失败: " + err.Error()
				v.mu.Unlock()
			}
		}
		if draft.Saved != "" {
			theme.MutedText(c, draft.Saved)
		}
	})
}
