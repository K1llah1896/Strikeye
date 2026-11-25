package parser

import (
	"math"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"strikeye/internal/mapgeom"
)

// eyeHeight is the offset from a player's origin to their view position.
const eyeHeight = 64.0

// ttdFOVHalfAngle is the horizontal half angle the attacker must have the
// victim inside for the frame to count as "able to see". 53° matches the 16:9
// CS2 field of view used by the kill-context optics analysis.
const ttdFOVHalfAngle = 53.0

// ttdBlindThreshold is the remaining flash duration (seconds) above which an
// attacker is considered unable to see. Mirrors killctx.BlindThreshold.
const ttdBlindThreshold = 0.1

// Scoped field of view. CS2 zooms by narrowing the same projection, so a scoped
// eye keeps the victim inside a much smaller half angle: ~20° at the first zoom
// step (AWP/SSG08 half zoom) and ~7.5° at the second. These are approximations
// of the engine's scoped FOV, which the demo does not expose directly.
const (
	ttdScopedHalfAngle     = 20.0
	ttdScopedFullHalfAngle = 7.5
)

// visSample is one 32 Hz snapshot of alive player positions, eye yaw and
// remaining blindness.
type visSample struct {
	Tick  int64
	Pos   map[string][3]float64
	Yaw   map[string]float64
	Blind map[string]float64
	// Zoom is the scope level per player (0 hip, 1 half, 2 full).
	Zoom map[string]uint8
}

// TTDSample records how long an attacker could see a victim before the first
// damage of a duel.
type TTDSample struct {
	RoundNumber     int
	AttackerSteamID string
	DurationMS      float64
}

// sampleObservation snapshots eye positions, eye yaw (degrees, 0 = +X, CCW
// positive) and remaining blind seconds for every alive player.
func sampleObservation(gs demoinfocs.GameState) visSample {
	playing := gs.Participants().Playing()
	pos := make(map[string][3]float64, len(playing))
	yaw := make(map[string]float64, len(playing))
	blind := make(map[string]float64, len(playing))
	zoom := make(map[string]uint8, len(playing))
	for _, pl := range playing {
		if pl == nil || !pl.IsAlive() {
			continue
		}
		sid := steamIDStr(pl)
		if sid == "" {
			continue
		}
		p := pl.Position()
		pos[sid] = [3]float64{p.X, p.Y, p.Z + eyeHeight}
		yaw[sid] = float64(pl.ViewDirectionX())
		blind[sid] = pl.FlashDurationTimeRemaining().Seconds()
		if aw := pl.ActiveWeapon(); aw != nil {
			if z := aw.ZoomLevel(); z > 0 {
				zoom[sid] = uint8(z)
			}
		}
	}
	return visSample{Pos: pos, Yaw: yaw, Blind: blind, Zoom: zoom}
}

// visibleRunMS walks back from tick while the attacker kept line of sight to the
// victim and returns the continuous visible duration. ok is false when no
// visible frame was found within the window.
func visibleRunMS(tracer *mapgeom.Tracer, samples []visSample, attSID, vicSID string, tick, windowTicks int64, tickRate float64) (float64, bool) {
	return visibleRunMSOptics(tracer, samples, attSID, vicSID, tick, windowTicks, tickRate, ttdFOVHalfAngle)
}

// visibleRunMSOptics is visibleRunMS with a configurable FOV half angle. A frame
// counts as visible only when the attacker has line of sight, is not blinded and
// holds the victim inside their horizontal FOV.
func visibleRunMSOptics(tracer *mapgeom.Tracer, samples []visSample, attSID, vicSID string, tick, windowTicks int64, tickRate, fovHalfDeg float64) (float64, bool) {
	if tracer == nil || tickRate <= 0 {
		return 0, false
	}
	start := -1
	for i := len(samples) - 1; i >= 0; i-- {
		if samples[i].Tick <= tick {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, false
	}
	visibleTicks := int64(0)
	seen := false
	for i := start; i >= 0; i-- {
		s := samples[i]
		if tick-s.Tick > windowTicks {
			break
		}
		a, okA := s.Pos[attSID]
		v, okV := s.Pos[vicSID]
		if !okA || !okV {
			break
		}
		if tracer.IsOccluded(a, v) {
			break
		}
		if b, ok := s.Blind[attSID]; ok && b > ttdBlindThreshold {
			break
		}
		half := scopedFOVHalfAngle(fovHalfDeg, s.Zoom[attSID])
		if y, ok := s.Yaw[attSID]; ok && half > 0 {
			dir := [3]float64{v[0] - a[0], v[1] - a[1], 0}
			if math.Hypot(dir[0], dir[1]) >= 8 && angleBetweenDeg(y, dir) > half {
				break
			}
		}
		seen = true
		visibleTicks = tick - s.Tick
	}
	if !seen {
		return 0, false
	}
	return float64(visibleTicks) / tickRate * 1000.0, true
}

// scopedFOVHalfAngle narrows the base half angle when the sample was taken
// while scoped in.
func scopedFOVHalfAngle(base float64, zoom uint8) float64 {
	switch zoom {
	case 1:
		return ttdScopedHalfAngle
	case 2:
		return ttdScopedFullHalfAngle
	}
	return base
}

// angleBetweenDeg returns the absolute angle between an eye yaw (degrees, 0 = +X,
// CCW positive) and a direction vector, projected on the XY plane.
func angleBetweenDeg(yaw float64, dir [3]float64) float64 {
	length := math.Hypot(dir[0], dir[1])
	if length < 1e-6 {
		return 0
	}
	rad := yaw * math.Pi / 180
	view := [2]float64{math.Cos(rad), math.Sin(rad)}
	dot := (view[0]*dir[0] + view[1]*dir[1]) / length
	if dot > 1 {
		dot = 1
	}
	if dot < -1 {
		dot = -1
	}
	return math.Acos(dot) * 180 / math.Pi
}
