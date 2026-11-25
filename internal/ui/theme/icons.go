package theme

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"strikeye/assets/weapons"
)

func mustIconSVG(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	// Navigation icons
	IconLibrary    = mustIconSVG(`<rect x="3" y="4" width="7" height="7" rx="1.5"/><rect x="14" y="4" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="6" rx="1.5"/><rect x="14" y="14" width="7" height="6" rx="1.5"/>`)
	IconSpectator  = mustIconSVG(`<path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6Z"/><circle cx="12" cy="12" r="2.75"/>`)
	IconAnalysis   = mustIconSVG(`<path d="M3 3v18h18"/><path d="m7 14 4-4 3 3 6-6"/>`)
	IconHighlights = mustIconSVG(`<path d="m12 3 1.9 4.6L18.5 9.5l-4.6 1.9L12 16l-1.9-4.6L5.5 9.5l4.6-1.9Z"/><path d="M19 16v4"/><path d="M21 18h-4"/>`)
	IconSettings   = mustIconSVG(`<path d="M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1Z"/>`)

	// Action & status icons
	IconSearch   = mustIconSVG(`<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>`)
	IconPlus     = mustIconSVG(`<path d="M12 5v14"/><path d="M5 12h14"/>`)
	IconPlay     = mustIconSVG(`<polygon points="6 4 20 12 6 20 6 4"/>`)
	IconFilm     = mustIconSVG(`<rect x="2" y="4" width="20" height="16" rx="2.5"/><path d="M7 4v16"/><path d="M17 4v16"/><path d="M2 12h20"/>`)
	IconRadar    = mustIconSVG(`<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5"/><circle cx="12" cy="12" r="1.5"/><path d="M12 12 18.5 5.5"/>`)
	IconSliders  = mustIconSVG(`<line x1="4" y1="6" x2="20" y2="6"/><line x1="4" y1="12" x2="20" y2="12"/><line x1="4" y1="18" x2="20" y2="18"/><circle cx="9" cy="6" r="2"/><circle cx="15" cy="12" r="2"/><circle cx="10" cy="18" r="2"/>`)
	IconRefresh  = mustIconSVG(`<path d="M21 12a9 9 0 1 1-2.64-6.36"/><polyline points="21 3 21 9 15 9"/>`)
	IconDownload = mustIconSVG(`<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>`)
	IconFolder   = mustIconSVG(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>`)
	IconStar     = mustIconSVG(`<polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>`)
	IconCheck    = mustIconSVG(`<polyline points="20 6 9 17 4 12"/>`)
	IconAlert    = mustIconSVG(`<path d="M12 9v4"/><path d="M12 17h.01"/><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/>`)
	IconBolt     = mustIconSVG(`<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/>`)
	IconShield   = mustIconSVG(`<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z"/>`)
	IconBell     = mustIconSVG(`<path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/>`)
	IconGitHub   = mustIconSVG(`<path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4"/><path d="M9 18c-4.51 2-5-2-7-2"/>`)
)

// RenderIcon renders a crisp vector SVG icon sized and tinted to col.
func RenderIcon(c *ui.Context, svg *ui.SVG, size float32, col ui.Color) ui.Element {
	if svg == nil {
		return ui.Box(c).Width(size).Height(size)
	}
	return ui.Icon(c, svg).Width(size).Height(size).TextColor(col).Shrink(0)
}

var (
	weaponSVGMu    sync.RWMutex
	weaponSVGCache = map[string]*ui.SVG{}
)

// CachedWeaponSVG lazily parses and caches an official CS2 weapon or killfeed modifier SVG.
func CachedWeaponSVG(name string) *ui.SVG {
	key := weapons.NormalizeWeaponKey(name)
	if key == "" {
		return nil
	}
	weaponSVGMu.RLock()
	svg, ok := weaponSVGCache[key]
	weaponSVGMu.RUnlock()
	if ok {
		return svg
	}

	var parsed *ui.SVG
	if raw, found := weapons.LoadSVG(key); found {
		if s, err := ui.ParseSVG(raw); err == nil {
			parsed = s
		}
	}
	weaponSVGMu.Lock()
	weaponSVGCache[key] = parsed
	weaponSVGMu.Unlock()
	return parsed
}

// RenderWeaponIcon renders a weapon or killfeed modifier SVG preserving its natural aspect ratio.
func RenderWeaponIcon(c *ui.Context, name string, height float32, col ui.Color) bool {
	svg := CachedWeaponSVG(name)
	if svg == nil {
		return false
	}
	w, h := ui.ImageSize(svg)
	dispW := height
	if w > 0 && h > 0 {
		dispW = height * (w / h)
	}
	ui.Icon(c, svg).Width(dispW).Height(height).TextColor(col).Shrink(0)
	return true
}
