package components

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
	"strikeye/assets/maps"
	"strikeye/internal/parser"
	"strikeye/internal/ui/theme"
)

// HeatmapPoint represents a world-space sample for KDE heatmap rendering.
type HeatmapPoint struct {
	X      float64
	Y      float64
	Z      float64
	Weight float64
}

// ReplayLayers selects which combat overlays are drawn on the replay canvas.
type ReplayLayers struct {
	Shots    bool
	Deaths   bool
	Bomb     bool
	Route    bool
	KillFeed bool
}

// DefaultReplayLayers enables every combat overlay.
func DefaultReplayLayers() ReplayLayers {
	return ReplayLayers{Shots: true, Deaths: true, Bomb: true, Route: true, KillFeed: true}
}

// killFeedWindowSeconds bounds how long a kill stays on the feed.
const killFeedWindowSeconds = 6.0

// killFeedMaxRows caps how many recent kills are listed.
const killFeedMaxRows = 4

// Pin is a tactical annotation drawn at a fixed world position (a callout
// centroid), independent of the replay tick.
type Pin struct {
	X     float64
	Y     float64
	Label string
	// Color is one of "amber", "red", "green", "blue".
	Color string
}

// StrokePoint is one sampled point of a freehand annotation in normalised
// [0,1] radar space.
type StrokePoint struct {
	X float32
	Y float32
}

// Stroke is a freehand drawing overlaid on the radar.
type Stroke struct {
	Points []StrokePoint
	// Color is one of "amber", "red", "green", "blue".
	Color string
}

// BrushHandler receives pointer samples while freehand mode is active.
// x/y are normalised to [0,1]; pressed reports whether the primary button is
// held. It is called at most once per frame and only while the pointer is over
// the canvas.
type BrushHandler func(x, y float32, pressed bool)

// Radar2DCanvas renders 32Hz CS2RPL01 tactical replay frames and KDE heatmaps
// onto a MyGo ui.Bitmap backed by assets/maps or a procedural coordinate grid.
type Radar2DCanvas struct {
	mu           sync.Mutex
	bmpMu        sync.Mutex
	mapName      string
	lowerLayer   bool
	meta         maps.MapMeta
	bgImage      image.Image
	bgCache      *image.RGBA
	bitmap       *ui.Bitmap // owned and mutated exclusively on the UI main thread in RenderUI
	pendingImg   image.Image
	pendingDirty bool
	size         int
	layers       ReplayLayers
	pins         []Pin
	strokes      []Stroke
	brush        bool
	onBrush      BrushHandler
	slotNames    map[uint8]string
	tickRate     float64
	lowerOffsetX float64
	lowerOffsetY float64
	lowerOpacity float64
}

// NewRadar2DCanvas constructs a square 2D radar canvas of size x size pixels.
func NewRadar2DCanvas(size int) *Radar2DCanvas {
	if size <= 0 {
		size = 640
	}
	return &Radar2DCanvas{size: size, layers: DefaultReplayLayers()}
}

// SetSlotNames supplies the slot -> player label mapping used by the kill feed.
// Names are copied so later view-model mutations cannot race the render thread.
func (r *Radar2DCanvas) SetSlotNames(names map[uint8]string) {
	copied := make(map[uint8]string, len(names))
	for slot, name := range names {
		copied[slot] = name
	}
	r.mu.Lock()
	r.slotNames = copied
	r.mu.Unlock()
}

// SetLowerView adjusts how the lower radar level is composited: offsets are in
// canvas pixels and opacity in (0,1]. Values are clamped so a bad call cannot
// push the underlay off-canvas or hide it entirely.
func (r *Radar2DCanvas) SetLowerView(offsetX, offsetY, opacity float64) {
	if opacity <= 0 {
		opacity = 1
	}
	if opacity > 1 {
		opacity = 1
	}
	if offsetX > 512 {
		offsetX = 512
	} else if offsetX < -512 {
		offsetX = -512
	}
	if offsetY > 512 {
		offsetY = 512
	} else if offsetY < -512 {
		offsetY = -512
	}
	r.mu.Lock()
	if r.lowerOffsetX == offsetX && r.lowerOffsetY == offsetY && r.lowerOpacity == opacity {
		r.mu.Unlock()
		return
	}
	r.lowerOffsetX, r.lowerOffsetY, r.lowerOpacity = offsetX, offsetY, opacity
	r.bgCache = nil
	r.mu.Unlock()
}

// SetTickRate sets the demo tick rate used for time-based overlays.
func (r *Radar2DCanvas) SetTickRate(rate float64) {
	if rate <= 0 {
		return
	}
	r.mu.Lock()
	r.tickRate = rate
	r.mu.Unlock()
}

// SetLayers updates which combat overlays RenderReplayFrame draws.
func (r *Radar2DCanvas) SetLayers(layers ReplayLayers) {
	r.mu.Lock()
	r.layers = layers
	r.mu.Unlock()
}

// SetPins replaces the annotation pins drawn on top of the replay frame.
func (r *Radar2DCanvas) SetPins(pins []Pin) {
	r.mu.Lock()
	r.pins = pins
	r.mu.Unlock()
}

// SetStrokes replaces the freehand drawings drawn on top of the replay frame.
func (r *Radar2DCanvas) SetStrokes(strokes []Stroke) {
	r.mu.Lock()
	r.strokes = strokes
	r.mu.Unlock()
}

// SetBrush enables freehand input and installs the sampling callback.
func (r *Radar2DCanvas) SetBrush(active bool, onPoint BrushHandler) {
	r.mu.Lock()
	r.brush = active
	r.onBrush = onPoint
	r.mu.Unlock()
}

// ConfigureMap loads the radar background and projection metadata for mapName,
// and immediately renders the base radar backdrop so empty replay states are never blank.
func (r *Radar2DCanvas) ConfigureMap(mapName string, lower bool, fallbackBounds [4]float64) {
	r.mu.Lock()
	if r.bgCache != nil && r.mapName == mapName && r.lowerLayer == lower {
		r.mu.Unlock()
		return
	}
	r.mapName = mapName
	r.lowerLayer = lower
	if meta, ok := maps.LookupMap(mapName); ok {
		r.meta = meta
	} else {
		r.meta = maps.DefaultFallbackMeta(fallbackBounds[0], fallbackBounds[1], fallbackBounds[2], fallbackBounds[3])
	}
	r.bgImage = nil
	r.bgCache = nil
	if raw, ok := maps.LoadRadarPNG(mapName, lower); ok {
		if decoded, err := png.Decode(bytes.NewReader(raw)); err == nil {
			r.bgImage = decoded
		}
	}
	r.ensureBgCacheLocked()
	r.mu.Unlock()
	r.RenderReplayFrame(parser.ReplayRoundData{}, -1)
}

func (r *Radar2DCanvas) ensureBgCacheLocked() {
	if r.bgCache != nil {
		return
	}
	rgba := image.NewRGBA(image.Rect(0, 0, r.size, r.size))
	// Fast obsidian fill
	for i := 0; i < len(rgba.Pix); i += 4 {
		rgba.Pix[i] = 11
		rgba.Pix[i+1] = 15
		rgba.Pix[i+2] = 23
		rgba.Pix[i+3] = 255
	}
	if r.bgImage != nil {
		if r.lowerLayer && (r.lowerOffsetX != 0 || r.lowerOffsetY != 0 || (r.lowerOpacity > 0 && r.lowerOpacity < 1)) {
			r.blitLowerUnderlayLocked(rgba)
			r.bgCache = rgba
			return
		}
		xdraw.ApproxBiLinear.Scale(rgba, rgba.Bounds(), r.bgImage, r.bgImage.Bounds(), xdraw.Over, nil)
		r.bgCache = rgba
		return
	}
	dc := gg.NewContextForRGBA(rgba)
	r.drawBackground(dc)
	r.bgCache = rgba
}

// blitLowerUnderlayLocked draws the lower level scaled, offset and alpha-blended
// so it reads as a translucent underlay below the current floor.
func (r *Radar2DCanvas) blitLowerUnderlayLocked(dst *image.RGBA) {
	scaled := image.NewRGBA(dst.Bounds())
	xdraw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), r.bgImage, r.bgImage.Bounds(), xdraw.Src, nil)

	alpha := r.lowerOpacity
	if alpha <= 0 || alpha > 1 {
		alpha = 1
	}
	dx := int(math.Round(r.lowerOffsetX))
	dy := int(math.Round(r.lowerOffsetY))
	rect := image.Rect(dx, dy, dst.Bounds().Dx()+dx, dst.Bounds().Dy()+dy)
	mask := image.NewUniform(color.Alpha{A: uint8(math.Round(alpha * 255))})
	// Over with a uniform alpha mask degrades to a full replace at alpha 1.0.
	xdraw.DrawMask(dst, rect, scaled, image.Point{}, mask, image.Point{}, xdraw.Over)
}

func (r *Radar2DCanvas) newFrameContextLocked() *gg.Context {
	r.ensureBgCacheLocked()
	dst := image.NewRGBA(image.Rect(0, 0, r.size, r.size))
	copy(dst.Pix, r.bgCache.Pix)
	return gg.NewContextForRGBA(dst)
}

func (r *Radar2DCanvas) drawBackground(dc *gg.Context) {
	s := float64(r.size)
	dc.SetHexColor("#0B0F17")
	dc.Clear()
	if r.bgImage != nil {
		b := r.bgImage.Bounds()
		sx := s / float64(b.Dx())
		sy := s / float64(b.Dy())
		dc.Push()
		dc.Scale(sx, sy)
		dc.DrawImage(r.bgImage, 0, 0)
		dc.Pop()
		return
	}
	// Procedural tactical grid fallback for custom/unbundled maps.
	dc.SetRGBA(0.22, 0.28, 0.38, 0.45)
	dc.SetLineWidth(1)
	step := s / 8.0
	for i := 1; i < 8; i++ {
		p := float64(i) * step
		dc.DrawLine(p, 0, p, s)
		dc.DrawLine(0, p, s, p)
	}
	dc.Stroke()
}

func validRadarPt(px, py, s float64) bool {
	if math.IsNaN(px) || math.IsInf(px, 0) || math.IsNaN(py) || math.IsInf(py, 0) {
		return false
	}
	return px >= -s && px <= 2*s && py >= -s && py <= 2*s
}

// RenderReplayFrame rasterizes one 32Hz ReplayFrame plus active grenades/smokes/infernos
// and stages the resulting RGBA image for main-thread GPU bitmap upload.
func (r *Radar2DCanvas) RenderReplayFrame(round parser.ReplayRoundData, frameIdx int) image.Image {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := float64(r.size)
	dc := r.newFrameContextLocked()

	if frameIdx >= 0 && frameIdx < len(round.Frames) {
		frame := round.Frames[frameIdx]
		tick := frame.Tick

		// 1. Smokes (144 CS2 units radius)
		rad := (144.0 / math.Max(1, r.meta.Scale)) * (s / 1024.0)
		if !math.IsNaN(rad) && !math.IsInf(rad, 0) && rad > 0 && rad <= s {
			for _, sm := range round.Smokes {
				if tick < sm.StartTick || tick > sm.EndTick {
					continue
				}
				px, py := maps.WorldToRadar(r.meta, float64(sm.Pos.X), float64(sm.Pos.Y), s, s)
				if !validRadarPt(px, py, s) {
					continue
				}
				dc.SetRGBA(0.75, 0.80, 0.88, 0.45)
				dc.DrawCircle(px, py, rad)
				dc.Fill()
				dc.SetRGBA(0.90, 0.95, 1.0, 0.75)
				dc.SetLineWidth(1.5)
				dc.DrawCircle(px, py, rad)
				dc.Stroke()
			}
		}

		// 2. Infernos / Molotov convex hulls
		for _, inf := range round.Infernos {
			if tick < inf.StartTick || tick > inf.EndTick || len(inf.Hull) < 3 {
				continue
			}
			ptsAdded := 0
			for _, pt := range inf.Hull {
				px, py := maps.WorldToRadar(r.meta, float64(pt.X), float64(pt.Y), s, s)
				if !validRadarPt(px, py, s) {
					continue
				}
				if ptsAdded == 0 {
					dc.MoveTo(px, py)
				} else {
					dc.LineTo(px, py)
				}
				ptsAdded++
			}
			if ptsAdded >= 3 {
				dc.ClosePath()
				dc.SetRGBA(0.96, 0.38, 0.12, 0.42)
				dc.FillPreserve()
				dc.SetRGBA(0.98, 0.55, 0.15, 0.85)
				dc.SetLineWidth(1.5)
				dc.Stroke()
			} else if ptsAdded > 0 {
				dc.ClearPath()
			}
		}

		// 3. Grenade flight arcs
		for _, g := range round.Grenades {
			if tick < g.ThrowTick || tick > g.EndTick || len(g.Points) < 2 {
				continue
			}
			ptsAdded := 0
			for _, pt := range g.Points {
				if pt.Tick > tick {
					break
				}
				px, py := maps.WorldToRadar(r.meta, float64(pt.Pos.X), float64(pt.Pos.Y), s, s)
				if !validRadarPt(px, py, s) {
					continue
				}
				if ptsAdded == 0 {
					dc.MoveTo(px, py)
				} else {
					dc.LineTo(px, py)
				}
				ptsAdded++
			}
			if ptsAdded >= 2 {
				dc.SetRGBA(0.22, 0.74, 0.97, 0.85)
				dc.SetLineWidth(1.8)
				dc.Stroke()
			} else if ptsAdded > 0 {
				dc.ClearPath()
			}
		}

		// 3.4 Movement trails, so a round reads as a push instead of dots.
		if r.layers.Route {
			r.drawRoutesLocked(dc, round, tick, s)
		}

		// 3.5 Combat layers: shots, deaths, C4.
		if r.layers.Shots {
			r.drawShotsLocked(dc, round, tick, s)
		}
		if r.layers.Bomb {
			r.drawBombsLocked(dc, round, tick, s)
		}
		if r.layers.Deaths {
			r.drawDeathsLocked(dc, round, tick, s)
		}

		// 4. Players (view cone, health ring, flash ring, C4 indicator)
		for _, p := range frame.Players {
			px, py := maps.WorldToRadar(r.meta, float64(p.X), float64(p.Y), s, s)
			if !validRadarPt(px, py, s) {
				continue
			}
			if !p.Alive {
				dc.SetRGBA(0.45, 0.50, 0.58, 0.65)
				dc.SetLineWidth(1.5)
				dc.DrawLine(px-4, py-4, px+4, py+4)
				dc.DrawLine(px+4, py-4, px-4, py+4)
				dc.Stroke()
				continue
			}

			// View cone (yaw in degrees, CS2 Y is inverted on radar)
			if !math.IsNaN(float64(p.Yaw)) && !math.IsInf(float64(p.Yaw), 0) {
				yawRad := -float64(p.Yaw) * math.Pi / 180.0
				coneLen := 22.0
				dc.SetRGBA(1, 1, 1, 0.16)
				dc.MoveTo(px, py)
				dc.LineTo(px+math.Cos(yawRad-0.35)*coneLen, py+math.Sin(yawRad-0.35)*coneLen)
				dc.LineTo(px+math.Cos(yawRad+0.35)*coneLen, py+math.Sin(yawRad+0.35)*coneLen)
				dc.ClosePath()
				dc.Fill()
			}

			// Team color dot
			if p.Team == 1 {
				dc.SetHexColor("#F59E0B")
			} else {
				dc.SetHexColor("#38BDF8")
			}
			dc.DrawCircle(px, py, 7)
			dc.Fill()

			// Health ring
			hpFrac := math.Max(0, math.Min(1, float64(p.Health)/100.0))
			dc.SetHexColor("#22C55E")
			dc.SetLineWidth(2)
			dc.DrawArc(px, py, 9.5, -math.Pi/2, -math.Pi/2+2*math.Pi*hpFrac)
			dc.Stroke()

			// Flashbang white ring
			if p.FlashDur > 0.1 {
				dc.SetRGBA(1, 1, 1, 0.9)
				dc.SetLineWidth(2)
				dc.DrawCircle(px, py, 12.5)
				dc.Stroke()
			}

			// C4 carrier marker
			if p.HasBomb {
				dc.SetHexColor("#EF4444")
				dc.DrawRectangle(px+6, py-10, 6, 6)
				dc.Fill()
			}

			dc.SetHexColor("#0F141C")
			dc.DrawStringAnchored(fmt.Sprintf("%d", p.Slot), px, py, 0.5, 0.5)
		}

		if r.layers.KillFeed {
			r.drawKillFeedLocked(dc, round, tick, s)
		}
	}

	// Tactical annotations stay visible even without a replay frame.
	r.drawPinsLocked(dc, s)
	r.drawStrokesLocked(dc, s)

	img := dc.Image()
	r.updateBitmap(img)
	return img
}

// routeStrideTicks samples the trail every 8 ticks (4 Hz at 64 tick) plus the
// latest frame at or before the cursor, so short pushes still render.
const routeStrideTicks int64 = 8

// drawRoutesLocked draws each player's path up to the current tick, coloured by
// team, with a dot at the current position.
func (r *Radar2DCanvas) drawRoutesLocked(dc *gg.Context, round parser.ReplayRoundData, tick int64, s float64) {
	type trail struct {
		team   uint8
		points [][2]float64
	}
	trails := map[uint8]*trail{}
	lastIdx := -1
	for i, frame := range round.Frames {
		if frame.Tick > tick {
			break
		}
		lastIdx = i
	}
	for i, frame := range round.Frames {
		if frame.Tick > tick {
			break
		}
		if frame.Tick%routeStrideTicks != 0 && i != lastIdx {
			continue
		}
		for _, p := range frame.Players {
			if p.Team != 1 && p.Team != 2 {
				continue
			}
			px, py := maps.WorldToRadar(r.meta, float64(p.X), float64(p.Y), s, s)
			if !validRadarPt(px, py, s) {
				continue
			}
			tr := trails[p.Slot]
			if tr == nil {
				tr = &trail{team: p.Team}
				trails[p.Slot] = tr
			}
			tr.points = append(tr.points, [2]float64{px, py})
		}
	}
	for _, tr := range trails {
		if len(tr.points) < 2 {
			continue
		}
		if tr.team == 1 {
			dc.SetRGBA(0.96, 0.62, 0.04, 0.5)
		} else {
			dc.SetRGBA(0.22, 0.74, 0.97, 0.5)
		}
		dc.SetLineWidth(2)
		dc.MoveTo(tr.points[0][0], tr.points[0][1])
		for _, pt := range tr.points[1:] {
			dc.LineTo(pt[0], pt[1])
		}
		dc.Stroke()
		last := tr.points[len(tr.points)-1]
		dc.DrawCircle(last[0], last[1], 2.5)
		dc.Fill()
	}
}

const (
	shotFadeTicks  int64 = 48
	deathLinkTicks int64 = 128
	blastFadeTicks int64 = 96
	c4BlastRadius        = 500.0
)

// drawStrokesLocked renders freehand annotations scaled to the canvas size.
func (r *Radar2DCanvas) drawStrokesLocked(dc *gg.Context, s float64) {
	if len(r.strokes) == 0 {
		return
	}
	dc.SetLineWidth(3)
	dc.SetLineCapRound()
	for _, stroke := range r.strokes {
		if len(stroke.Points) < 2 {
			continue
		}
		cr, cg, cb := 1.0, 0.78, 0.32
		switch stroke.Color {
		case "red":
			cr, cg, cb = 0.98, 0.34, 0.36
		case "green":
			cr, cg, cb = 0.38, 0.86, 0.58
		case "blue":
			cr, cg, cb = 0.42, 0.68, 0.98
		}
		dc.SetRGBA(cr, cg, cb, 0.92)
		dc.MoveTo(float64(stroke.Points[0].X)*s, float64(stroke.Points[0].Y)*s)
		for _, p := range stroke.Points[1:] {
			dc.LineTo(float64(p.X)*s, float64(p.Y)*s)
		}
		dc.Stroke()
	}
}

func (r *Radar2DCanvas) drawPinsLocked(dc *gg.Context, s float64) {
	for _, pin := range r.pins {
		px, py := maps.WorldToRadar(r.meta, pin.X, pin.Y, s, s)
		if !validRadarPt(px, py, s) {
			continue
		}
		cr, cg, cb := 1.0, 0.78, 0.32
		switch pin.Color {
		case "red":
			cr, cg, cb = 0.98, 0.34, 0.36
		case "green":
			cr, cg, cb = 0.38, 0.86, 0.58
		case "blue":
			cr, cg, cb = 0.42, 0.68, 0.98
		}
		dc.SetRGBA(cr, cg, cb, 0.16)
		dc.DrawCircle(px, py, 16)
		dc.Fill()
		dc.SetRGBA(cr, cg, cb, 0.95)
		dc.SetLineWidth(1.4)
		dc.DrawCircle(px, py, 10)
		dc.Stroke()
		dc.DrawCircle(px, py, 2.2)
		dc.Fill()
		if pin.Label != "" {
			dc.SetRGBA(0.95, 0.97, 1.0, 0.92)
			if err := dc.LoadFontFace("", 10); err == nil {
				dc.DrawStringAnchored(pin.Label, px+13, py-10, 0, 1)
			}
		}
	}
}

func (r *Radar2DCanvas) drawShotsLocked(dc *gg.Context, round parser.ReplayRoundData, tick int64, s float64) {
	for _, sh := range round.Shots {
		age := tick - sh.Tick
		if age < 0 || age > shotFadeTicks {
			continue
		}
		x1, y1 := maps.WorldToRadar(r.meta, float64(sh.Start.X), float64(sh.Start.Y), s, s)
		x2, y2 := maps.WorldToRadar(r.meta, float64(sh.End.X), float64(sh.End.Y), s, s)
		if !validRadarPt(x1, y1, s) || !validRadarPt(x2, y2, s) {
			continue
		}
		alpha := 0.75 * (1 - float64(age)/float64(shotFadeTicks))
		dc.SetRGBA(1.0, 0.84, 0.35, alpha)
		dc.SetLineWidth(1.2)
		dc.MoveTo(x1, y1)
		dc.LineTo(x2, y2)
		dc.Stroke()
	}
}

// drawKillFeedLocked renders the most recent kills as a compact feed panel so
// viewers can see who traded whom without scrubbing the timeline.
func (r *Radar2DCanvas) drawKillFeedLocked(dc *gg.Context, round parser.ReplayRoundData, tick int64, s float64) {
	if len(round.Deaths) == 0 {
		return
	}
	rate := r.tickRate
	if rate <= 0 {
		rate = 64
	}
	window := int64(killFeedWindowSeconds * rate)

	recent := make([]parser.ReplayDeathEvent, 0, killFeedMaxRows)
	for _, d := range round.Deaths {
		if d.Tick > tick {
			continue
		}
		if tick-d.Tick > window {
			continue
		}
		recent = append(recent, d)
	}
	if len(recent) == 0 {
		return
	}
	// Oldest first so the newest kill sits at the bottom of the feed.
	sort.Slice(recent, func(i, j int) bool { return recent[i].Tick < recent[j].Tick })
	if len(recent) > killFeedMaxRows {
		recent = recent[len(recent)-killFeedMaxRows:]
	}

	const rowH = 20.0
	pad := 12.0
	panelW := s*0.52 - pad
	if panelW < 180 {
		panelW = 180
	}
	panelH := float64(len(recent))*rowH + pad
	x := s - panelW - pad
	y := pad
	dc.SetRGBA(0.04, 0.06, 0.10, 0.62)
	dc.DrawRoundedRectangle(x, y, panelW, panelH, 8)
	dc.Fill()
	dc.SetRGBA(0.35, 0.45, 0.60, 0.55)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(x, y, panelW, panelH, 8)
	dc.Stroke()

	for i, d := range recent {
		rowY := y + pad/2 + float64(i)*rowH + rowH/2
		killer := r.slotLabel(d.KillerSlot)
		victim := r.slotLabel(d.VictimSlot)
		weapon := parser.NearestShotWeapon(round, d.KillerSlot, d.Tick)
		label := fmt.Sprintf("%s ▸ %s", killer, victim)
		if weapon != "" {
			label = fmt.Sprintf("%s ▸ %s  %s", killer, victim, weapon)
		}
		age := float64(tick-d.Tick) / float64(window)
		if age < 0 {
			age = 0
		}
		if age > 1 {
			age = 1
		}
		alpha := 0.95 - 0.45*age
		dc.SetRGBA(0.86, 0.92, 1.0, alpha)
		dc.DrawStringAnchored(label, x+10, rowY, 0, 0.5)
		if d.Headshot {
			dc.SetRGBA(0.98, 0.34, 0.34, alpha)
			dc.DrawCircle(x+panelW-12, rowY, 3)
			dc.Fill()
		}
	}
}

// slotLabel resolves a slot to its player name, falling back to the slot number.
func (r *Radar2DCanvas) slotLabel(slot uint8) string {
	if name, ok := r.slotNames[slot]; ok && name != "" {
		return name
	}
	return fmt.Sprintf("#%d", slot)
}

// killWeapon finds the killer's shot closest to the kill tick, which is the
// weapon that actually dealt the damage in practice.
func (r *Radar2DCanvas) drawDeathsLocked(dc *gg.Context, round parser.ReplayRoundData, tick int64, s float64) {
	for _, d := range round.Deaths {
		if d.Tick > tick {
			continue
		}
		vx, vy := maps.WorldToRadar(r.meta, float64(d.Victim.X), float64(d.Victim.Y), s, s)
		if !validRadarPt(vx, vy, s) {
			continue
		}
		dc.SetRGBA(0.88, 0.92, 1.0, 0.85)
		dc.SetLineWidth(1.6)
		dc.DrawLine(vx-4, vy-4, vx+4, vy+4)
		dc.DrawLine(vx+4, vy-4, vx-4, vy+4)
		dc.Stroke()
		if d.Headshot {
			dc.SetRGBA(0.98, 0.30, 0.30, 0.9)
			dc.DrawCircle(vx, vy, 3)
			dc.Fill()
		}
		if age := tick - d.Tick; age >= 0 && age <= deathLinkTicks && d.KillerSlot != 0 {
			kx, ky := maps.WorldToRadar(r.meta, float64(d.Killer.X), float64(d.Killer.Y), s, s)
			if validRadarPt(kx, ky, s) {
				alpha := 0.55 * (1 - float64(age)/float64(deathLinkTicks))
				dc.SetRGBA(0.98, 0.32, 0.32, alpha)
				dc.SetLineWidth(1.2)
				dc.MoveTo(kx, ky)
				dc.LineTo(vx, vy)
				dc.Stroke()
			}
		}
	}
}

func (r *Radar2DCanvas) drawBombsLocked(dc *gg.Context, round parser.ReplayRoundData, tick int64, s float64) {
	rad := (c4BlastRadius / math.Max(1, r.meta.Scale)) * (s / 1024.0)
	for _, b := range round.Bombs {
		if b.Tick > tick {
			continue
		}
		px, py := maps.WorldToRadar(r.meta, float64(b.Pos.X), float64(b.Pos.Y), s, s)
		if !validRadarPt(px, py, s) {
			continue
		}
		switch b.Kind {
		case parser.ReplayBombPlant:
			dc.SetRGBA(0.95, 0.28, 0.22, 0.9)
			dc.DrawRectangle(px-4, py-4, 8, 8)
			dc.Fill()
			dc.SetRGBA(1, 0.55, 0.35, 0.6)
			dc.SetLineWidth(1.4)
			dc.DrawCircle(px, py, 10)
			dc.Stroke()
		case parser.ReplayBombDefuse:
			if age := tick - b.Tick; age <= blastFadeTicks {
				dc.SetRGBA(0.30, 0.85, 0.95, 0.85)
				dc.SetLineWidth(1.6)
				dc.DrawCircle(px, py, 7)
				dc.Stroke()
			}
		case parser.ReplayBombExplode:
			age := tick - b.Tick
			if age > blastFadeTicks || !(rad > 0 && rad <= s*2) {
				continue
			}
			alpha := 0.42 * (1 - float64(age)/float64(blastFadeTicks))
			dc.SetRGBA(0.96, 0.22, 0.16, alpha)
			dc.DrawCircle(px, py, rad)
			dc.Fill()
			dc.SetRGBA(1.0, 0.55, 0.20, alpha+0.18)
			dc.SetLineWidth(2)
			dc.DrawCircle(px, py, rad)
			dc.Stroke()
		}
	}
}

func (r *Radar2DCanvas) updateBitmap(img image.Image) {
	// Never create or mutate ui.Bitmap off the main UI thread; stage the latest image
	// so RenderUI uploads it safely on the main thread.
	r.bmpMu.Lock()
	r.pendingImg = img
	r.pendingDirty = true
	r.bmpMu.Unlock()
}

// ComputeKDEGrid builds a normalized [0,1] 2D kernel density grid from world samples
// using a histogram + separable 1D Gaussian convolution so runtime is O(N + gridRes^2 * radius).
func ComputeKDEGrid(meta maps.MapMeta, points []HeatmapPoint, gridRes int, bandwidth float64) [][]float64 {
	if gridRes <= 0 {
		gridRes = 64
	}
	if bandwidth <= 0 {
		bandwidth = 2.5
	}
	grid := make([][]float64, gridRes)
	for y := range grid {
		grid[y] = make([]float64, gridRes)
	}
	if len(points) == 0 {
		return grid
	}

	radius := int(math.Ceil(bandwidth * 2.5))
	twoSigmaSq := 2 * bandwidth * bandwidth

	// 1. Accumulate weights onto a 2D histogram (O(N), zero math.Exp calls).
	hist := make([]float64, gridRes*gridRes)
	hasAny := false
	resF := float64(gridRes)
	for _, pt := range points {
		if math.IsNaN(pt.X) || math.IsInf(pt.X, 0) || math.IsNaN(pt.Y) || math.IsInf(pt.Y, 0) {
			continue
		}
		gx, gy := maps.WorldToRadar(meta, pt.X, pt.Y, resF, resF)
		if math.IsNaN(gx) || math.IsInf(gx, 0) || math.IsNaN(gy) || math.IsInf(gy, 0) {
			continue
		}
		cx, cy := int(math.Round(gx)), int(math.Round(gy))
		if cx < -radius || cx >= gridRes+radius || cy < -radius || cy >= gridRes+radius {
			continue
		}
		w := pt.Weight
		if w <= 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			w = 1
		}
		if cx >= 0 && cx < gridRes && cy >= 0 && cy < gridRes {
			hist[cy*gridRes+cx] += w
			hasAny = true
		}
	}
	if !hasAny {
		return grid
	}

	// 2. Precompute 1D Gaussian kernel (only radius+1 math.Exp calls).
	kernel := make([]float64, radius+1)
	for d := 0; d <= radius; d++ {
		kernel[d] = math.Exp(-float64(d*d) / twoSigmaSq)
	}

	// 3. Horizontal 1D pass: hist -> tmp.
	tmp := make([]float64, gridRes*gridRes)
	for y := 0; y < gridRes; y++ {
		row := y * gridRes
		for x := 0; x < gridRes; x++ {
			v := hist[row+x]
			if v == 0 {
				continue
			}
			x0 := x - radius
			if x0 < 0 {
				x0 = 0
			}
			x1 := x + radius
			if x1 >= gridRes {
				x1 = gridRes - 1
			}
			for nx := x0; nx <= x1; nx++ {
				dx := nx - x
				if dx < 0 {
					dx = -dx
				}
				tmp[row+nx] += v * kernel[dx]
			}
		}
	}

	// 4. Vertical 1D pass: tmp -> grid, tracking maxVal.
	maxVal := 0.0
	for y := 0; y < gridRes; y++ {
		y0 := y - radius
		if y0 < 0 {
			y0 = 0
		}
		y1 := y + radius
		if y1 >= gridRes {
			y1 = gridRes - 1
		}
		outRow := grid[y]
		for ny := y0; ny <= y1; ny++ {
			dy := ny - y
			if dy < 0 {
				dy = -dy
			}
			k := kernel[dy]
			srcRow := tmp[ny*gridRes : (ny+1)*gridRes]
			for x := 0; x < gridRes; x++ {
				if srcRow[x] == 0 {
					continue
				}
				outRow[x] += srcRow[x] * k
			}
		}
		for x := 0; x < gridRes; x++ {
			if outRow[x] > maxVal {
				maxVal = outRow[x]
			}
		}
	}

	if maxVal > 0 {
		invMax := 1.0 / maxVal
		for y := 0; y < gridRes; y++ {
			row := grid[y]
			for x := 0; x < gridRes; x++ {
				row[x] *= invMax
			}
		}
	}
	return grid
}

// RenderHeatmap rasterizes a KDE heatmap overlay onto the map background using direct pixel blending.
func (r *Radar2DCanvas) RenderHeatmap(points []HeatmapPoint) image.Image {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureBgCacheLocked()
	dst := image.NewRGBA(image.Rect(0, 0, r.size, r.size))
	copy(dst.Pix, r.bgCache.Pix)

	const gridRes = 96
	grid := ComputeKDEGrid(r.meta, points, gridRes, 2.2)
	for py := 0; py < r.size; py++ {
		gy := py * gridRes / r.size
		if gy >= gridRes {
			gy = gridRes - 1
		}
		rowOff := py * dst.Stride
		for px := 0; px < r.size; px++ {
			gx := px * gridRes / r.size
			if gx >= gridRes {
				gx = gridRes - 1
			}
			v := grid[gy][gx]
			if v < 0.04 {
				continue
			}
			c := thermalColor(v)
			a := uint32(c.A)
			inv := 255 - a
			idx := rowOff + px*4
			dst.Pix[idx] = uint8((uint32(dst.Pix[idx])*inv + uint32(c.R)*a) / 255)
			dst.Pix[idx+1] = uint8((uint32(dst.Pix[idx+1])*inv + uint32(c.G)*a) / 255)
			dst.Pix[idx+2] = uint8((uint32(dst.Pix[idx+2])*inv + uint32(c.B)*a) / 255)
			dst.Pix[idx+3] = 255
		}
	}

	r.updateBitmap(dst)
	return dst
}

func thermalColor(v float64) color.NRGBA {
	if v < 0.35 {
		t := v / 0.35
		return color.NRGBA{R: 56, G: 189, B: 248, A: uint8(t * 150)}
	}
	if v < 0.70 {
		t := (v - 0.35) / 0.35
		return color.NRGBA{R: uint8(56 + t*(245-56)), G: uint8(189 + t*(158-189)), B: uint8(248 * (1 - t)), A: uint8(150 + t*50)}
	}
	t := (v - 0.70) / 0.30
	return color.NRGBA{R: 239, G: uint8(158 * (1 - t)), B: 30, A: uint8(200 + t*45)}
}

// RenderUI displays the cached bitmap inside a MyGo Native UI container.
// It flushes any staged raster image into ui.Bitmap on the main UI thread.
func (r *Radar2DCanvas) RenderUI(c *ui.Context, displaySize float32) {
	r.bmpMu.Lock()
	if r.pendingDirty && r.pendingImg != nil {
		if r.bitmap == nil {
			r.bitmap = ui.NewBitmap(r.pendingImg)
		} else {
			r.bitmap.Update(r.pendingImg)
		}
		r.pendingImg = nil
		r.pendingDirty = false
	}
	bmp := r.bitmap
	r.bmpMu.Unlock()
	if bmp == nil {
		ui.Column(c).Width(displaySize).Height(displaySize).
			Background(theme.BgInput).
			Border(1, theme.RimBorder).
			Radius(18).
			Center().
			Gap(8).
			Children(func() {
				theme.RenderIcon(c, theme.IconRadar, 26, theme.TextMuted)
				theme.MutedText(c, "等待加载 2D 雷达数据...")
			})
		return
	}
	r.mu.Lock()
	brush := r.brush
	onBrush := r.onBrush
	r.mu.Unlock()

	canvas := ui.Box(c).Width(displaySize).Height(displaySize).Radius(18).Border(1, theme.RimBorder).Clip()
	canvas.Children(func() {
		ui.Image(c, bmp).Fill()
	})
	if brush && onBrush != nil {
		if x, y, over := canvas.PointerPosition(); over {
			if pressed := canvas.Pressed(); pressed || canvas.PressedOutside() {
				onBrush(clampNorm(x/displaySize), clampNorm(y/displaySize), pressed)
			}
		}
	}
}

func clampNorm(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
