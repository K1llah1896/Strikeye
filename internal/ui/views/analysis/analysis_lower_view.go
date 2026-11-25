package analysis

import (
	"context"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
)

// setStatusMessage records a transient status line shown in the header.
func (v *View) setStatusMessage(msg string) {
	v.mu.Lock()
	v.statusMsg = msg
	v.mu.Unlock()
}

// defaultLowerOpacity keeps the underlay readable while the current floor stays
// dominant.
const defaultLowerOpacity = 0.55

// loadLowerViewPrefs reads the stored alignment for a map and applies it to the
// canvases. Missing rows fall back to centred, fully opaque rendering.
func (v *View) loadLowerViewPrefs(mapName string) {
	if v.db == nil || mapName == "" {
		return
	}
	prefs, err := v.db.MapViewPrefs().Get(context.Background(), mapName)
	if err != nil {
		v.setStatusMessage("读取下层对齐失败: " + err.Error())
		return
	}
	v.mu.Lock()
	v.lowerOffsetX, v.lowerOffsetY, v.lowerOpacity = prefs.LowerOffsetX, prefs.LowerOffsetY, prefs.LowerOpacity
	v.mu.Unlock()
	v.applyLowerView()
}

// applyLowerView pushes the current offsets onto both canvases.
func (v *View) applyLowerView() {
	v.mu.Lock()
	x, y, op := v.lowerOffsetX, v.lowerOffsetY, v.lowerOpacity
	radar, heat := v.radarCanvas, v.heatmapCanvas
	v.mu.Unlock()
	if radar != nil {
		radar.SetLowerView(x, y, op)
	}
	if heat != nil {
		heat.SetLowerView(x, y, op)
	}
}

// saveLowerViewPrefs persists the alignment for the demo's map.
func (v *View) saveLowerViewPrefs(mapName string) {
	if v.db == nil || mapName == "" {
		return
	}
	v.mu.Lock()
	prefs := storage.MapViewPrefs{
		MapName:      mapName,
		LowerOffsetX: v.lowerOffsetX,
		LowerOffsetY: v.lowerOffsetY,
		LowerOpacity: v.lowerOpacity,
	}
	v.mu.Unlock()
	if err := v.db.MapViewPrefs().Upsert(context.Background(), prefs); err != nil {
		v.setStatusMessage("保存下层对齐失败: " + err.Error())
	}
}

// renderLowerViewControls draws the offset/opacity row used by maps with a
// lower level. It reports whether any control changed.
func (v *View) renderLowerViewControls(c *ui.Context, mapName string) bool {
	if mapName == "" {
		return false
	}
	changed := false
	theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
		theme.MutedText(c, "下层 X").Width(56)
		if ui.Slider(c, &v.lowerOffsetX, -160, 160).Width(150).Changed() {
			changed = true
		}
		theme.MutedText(c, "Y").Width(20)
		if ui.Slider(c, &v.lowerOffsetY, -160, 160).Width(150).Changed() {
			changed = true
		}
		theme.MutedText(c, "不透明度").Width(64)
		if ui.Slider(c, &v.lowerOpacity, 0.15, 1.0).Width(150).Changed() {
			changed = true
		}
		if theme.ActionButton(c, "重置").Clicked() {
			v.mu.Lock()
			v.lowerOffsetX, v.lowerOffsetY, v.lowerOpacity = 0, 0, 1
			v.mu.Unlock()
			changed = true
		}
		if theme.ActionButton(c, "下层半透明").Clicked() {
			v.mu.Lock()
			v.lowerOpacity = defaultLowerOpacity
			v.mu.Unlock()
			changed = true
		}
	})
	if changed {
		v.applyLowerView()
		v.saveLowerViewPrefs(mapName)
	}
	return changed
}
