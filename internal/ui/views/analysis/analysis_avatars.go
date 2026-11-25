package analysis

import (
	"context"
	"fmt"
	"image"
	"os"
	"time"

	"strikeye/internal/avatars"
	"strikeye/internal/storage"

	"github.com/egoist/mygo/ui"
)

// loadAvatars decodes cached player avatars for the current roster. Missing or
// undecodable files are simply skipped so the table falls back to the status
// dot.
func loadAvatars(dataRoot string, players []storage.MatchPlayer) map[string]*ui.Bitmap {
	out := make(map[string]*ui.Bitmap, len(players))
	dir := avatars.CacheDir(dataRoot)
	for _, p := range players {
		path, ok := avatars.Path(dir, p.SteamID64)
		if !ok {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil || img == nil {
			continue
		}
		out[p.SteamID64] = ui.NewBitmap(img)
	}
	return out
}

// avatarFor returns the decoded avatar for a player, if one was cached.
func (v *View) avatarFor(steamID string) (*ui.Bitmap, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	img, ok := v.avatars[steamID]
	return img, ok
}

// ensureAvatarsAsync fetches portraits for the current roster that are not in
// the cache yet. Opening a demo is all a user ever does: no button, no dialog.
func (v *View) ensureAvatarsAsync() {
	v.mu.RLock()
	players := append([]storage.MatchPlayer(nil), v.players...)
	dataRoot := v.dataRoot
	busy := v.avatarBusy
	have := len(v.avatars)
	v.mu.RUnlock()
	if !v.avatarAutoFetch || busy || len(players) == 0 || have == len(players) {
		return
	}
	dir := avatars.CacheDir(dataRoot)
	missing := false
	for _, p := range players {
		if !avatars.ValidSteamID64(p.SteamID64) {
			continue
		}
		if _, ok := avatars.Path(dir, p.SteamID64); !ok {
			missing = true
			break
		}
	}
	if !missing {
		return
	}
	v.mu.Lock()
	v.avatarBusy = true
	v.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		client := v.avatarClient
		if client == nil {
			client = avatars.NewClient()
		}
		fetched := 0
		for _, p := range players {
			if !avatars.ValidSteamID64(p.SteamID64) {
				continue
			}
			if _, ok := avatars.Path(dir, p.SteamID64); ok {
				continue
			}
			if _, err := client.Fetch(ctx, p.SteamID64, dir, avatars.DefaultMaxAge); err == nil {
				fetched++
			}
		}
		bitmaps := loadAvatars(dataRoot, players)
		v.mu.Lock()
		v.avatars = bitmaps
		v.avatarBusy = false
		if fetched > 0 {
			v.avatarMsg = fmt.Sprintf("头像：已自动补齐 %d 个", fetched)
		}
		v.mu.Unlock()
		v.invalidate()
	}()
}

// refreshAvatars re-fetches every roster avatar past the 30-day TTL in the
// background, then reloads the bitmaps. Without this the only way to update an
// avatar is deleting the cache directory by hand.
func (v *View) refreshAvatars() {
	v.mu.RLock()
	players := append([]storage.MatchPlayer(nil), v.players...)
	dataRoot := v.dataRoot
	busy := v.avatarBusy
	v.mu.RUnlock()
	if busy || len(players) == 0 {
		return
	}
	v.mu.Lock()
	v.avatarBusy = true
	v.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		client := avatars.NewClient()
		dir := avatars.CacheDir(dataRoot)
		fetched, cached := 0, 0
		for _, p := range players {
			if !avatars.ValidSteamID64(p.SteamID64) {
				continue
			}
			before, hadBefore := avatars.Path(dir, p.SteamID64)
			path, err := client.Fetch(ctx, p.SteamID64, dir, 0)
			if err != nil {
				continue
			}
			if hadBefore && path == before {
				cached++
			} else {
				fetched++
			}
		}
		bitmaps := loadAvatars(dataRoot, players)
		v.mu.Lock()
		v.avatars = bitmaps
		v.avatarBusy = false
		v.avatarMsg = fmt.Sprintf("头像：新下载 %d，已缓存 %d", fetched, cached)
		v.mu.Unlock()
	}()
}
