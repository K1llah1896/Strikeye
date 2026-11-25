package analysis

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/storage"
	"strikeye/internal/ui/components"
	"strikeye/internal/ui/theme"
)

// brushMinStep is the minimum normalised distance between recorded points; it
// keeps strokes small without visibly breaking the line.
const brushMinStep float32 = 0.004

// brushColor is the colour used for new freehand strokes.
const brushColor = "amber"

// applyStrokes rebuilds the canvas strokes for the current round (round 0
// strokes apply to every round).
func (v *View) applyStrokes() {
	v.mu.RLock()
	strokes := strokesForRound(v.zoneStrokes, v.zoneRound)
	pending := append([]components.StrokePoint(nil), v.brushPts...)
	canvas := v.radarCanvas
	brush := v.brushActive
	v.mu.RUnlock()
	if canvas == nil {
		return
	}
	canvas.SetStrokes(strokes)
	canvas.SetBrush(brush, v.handleBrushPoint)
	if len(strokes) > 0 || len(pending) > 0 {
		v.renderReplayFrameAsync(0, false)
	}
}

// strokesForRound converts stored rows into canvas strokes, including the
// match-wide drawings stored with round number 0.
func strokesForRound(rows []storage.MapStroke, round int) []components.Stroke {
	out := make([]components.Stroke, 0, len(rows))
	for _, row := range rows {
		if row.RoundNumber != 0 && row.RoundNumber != round {
			continue
		}
		if len(row.Points) < 2 {
			continue
		}
		points := make([]components.StrokePoint, 0, len(row.Points))
		for _, p := range row.Points {
			points = append(points, components.StrokePoint{X: p.X, Y: p.Y})
		}
		out = append(out, components.Stroke{Points: points, Color: row.Color})
	}
	return out
}

// handleBrushPoint is called while freehand mode is active. It accumulates the
// current stroke and persists it on release.
func (v *View) handleBrushPoint(x, y float32, pressed bool) {
	v.mu.RLock()
	active := v.brushActive
	demo := v.demo
	db := v.db
	round := v.zoneRound
	v.mu.RUnlock()
	if !active || demo == nil || db == nil {
		return
	}

	v.mu.Lock()
	if pressed {
		appendPoint := true
		if n := len(v.brushPts); n > 0 {
			dx := v.brushPts[n-1].X - x
			dy := v.brushPts[n-1].Y - y
			if dx*dx+dy*dy < brushMinStep*brushMinStep {
				appendPoint = false
			}
		}
		if appendPoint {
			v.brushPts = append(v.brushPts, components.StrokePoint{X: x, Y: y})
		}
		v.mu.Unlock()
		return
	}

	points := append([]components.StrokePoint(nil), v.brushPts...)
	v.brushPts = nil
	v.mu.Unlock()
	if len(points) < 2 {
		return
	}

	demoID := demo.ID
	go func() {
		ctx := context.Background()
		stroke := storage.MapStroke{DemoID: demoID, RoundNumber: round, Color: brushColor}
		for _, p := range points {
			stroke.Points = append(stroke.Points, storage.StrokePoint{X: p.X, Y: p.Y})
		}
		if _, err := db.MapStrokes().Add(ctx, stroke); err != nil {
			return
		}
		rows, err := db.MapStrokes().List(ctx, demoID)
		if err != nil {
			return
		}
		v.mu.Lock()
		v.zoneStrokes = rows
		v.mu.Unlock()
		v.applyStrokes()
		v.invalidate()
	}()
}

// toggleBrush switches freehand drawing on or off, discarding any stroke that
// is still in progress.
func (v *View) toggleBrush() {
	v.mu.Lock()
	v.brushActive = !v.brushActive
	v.brushPts = nil
	active := v.brushActive
	v.mu.Unlock()
	if canvas := v.radarCanvas; canvas != nil {
		canvas.SetBrush(active, v.handleBrushPoint)
	}
	v.renderReplayFrameAsync(0, false)
	v.invalidate()
}

// clearRoundStrokes deletes every freehand stroke of the current round and
// repaints the canvas.
func (v *View) clearRoundStrokes() {
	v.mu.RLock()
	db := v.db
	demo := v.demo
	round := v.zoneRound
	v.mu.RUnlock()
	if db == nil || demo == nil {
		return
	}
	demoID := demo.ID
	go func() {
		ctx := context.Background()
		if _, err := db.MapStrokes().DeleteByRound(ctx, demoID, round); err != nil {
			return
		}
		rows, err := db.MapStrokes().List(ctx, demoID)
		if err != nil {
			return
		}
		v.mu.Lock()
		v.zoneStrokes = rows
		v.mu.Unlock()
		v.applyStrokes()
		v.invalidate()
	}()
}

// renderStrokeControls draws the freehand toolbar inside the zones tab.
func (v *View) renderStrokeControls(c *ui.Context) {
	v.mu.RLock()
	active := v.brushActive
	count := len(strokesForRound(v.zoneStrokes, v.zoneRound))
	round := v.zoneRound
	v.mu.RUnlock()

	label := "✎ 自由笔刷: 关"
	if active {
		label = "✎ 自由笔刷: 开（拖动绘制）"
	}
	theme.RowCenter(c).FillWidth().Gap(10).Children(func() {
		if theme.ActionButton(c, label).Clicked() {
			v.toggleBrush()
		}
		if count > 0 {
			if theme.ActionButton(c, fmt.Sprintf("清除本回合笔迹（%d）", count)).Clicked() {
				v.clearRoundStrokes()
			}
		}
		theme.MutedText(c, fmt.Sprintf("笔迹绑定 demo 与回合（当前回合 %d，回合 0 的笔迹对所有回合可见）。", round))
	})
}
