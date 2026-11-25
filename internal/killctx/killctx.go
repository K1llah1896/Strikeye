// Package killctx annotates every kill with the geometry behind it: how far the
// shot travelled, whether the killer could actually see the victim, and whether
// the line crossed a live smoke.
package killctx

import (
	"math"
	"sort"

	"strikeye/internal/mapgeom"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

// Config tunes the eye/chest heights used for line-of-sight and the smoke size.
type Config struct {
	EyeHeight   float64
	ChestHeight float64
	SmokeRadius float64
	// FOVHalfAngle is half of the horizontal field of view. 53° approximates the
	// 16:9 horizontal FOV of a 90° vertical CS2 view; scoped rifles are narrower
	// but the replay only records yaw, not zoom state.
	FOVHalfAngle float64
	// BlindThreshold is the remaining flash duration (seconds) above which a
	// player is treated as blinded at the kill tick.
	BlindThreshold float64
}

// DefaultConfig uses CS2 eye height (64) and chest height (45) with the standard
// 144 unit smoke sphere.
func DefaultConfig() Config {
	return Config{EyeHeight: 64, ChestHeight: 45, SmokeRadius: 144, FOVHalfAngle: 53, BlindThreshold: 0.1}
}

// Kill is one annotated kill event.
type Kill struct {
	Round      int     `json:"round"`
	Tick       int64   `json:"tick"`
	TimeSec    float64 `json:"time_sec"`
	Killer     string  `json:"killer"`
	Victim     string  `json:"victim"`
	KillerSlot uint8   `json:"killer_slot"`
	VictimSlot uint8   `json:"victim_slot"`
	Headshot   bool    `json:"headshot"`
	// Weapon is inferred from the killer's nearest weapon_fire event, because the
	// death record itself does not carry one.
	Weapon string `json:"weapon"`
	// KillerPos / VictimPos are the raw world positions (feet), kept so stored
	// kill context rows can be plotted without re-reading the replay.
	KillerPos [3]float64 `json:"killer_pos"`
	VictimPos [3]float64 `json:"victim_pos"`
	Distance  float64    `json:"distance"`
	// Visible is false when map geometry blocks the killer's eye line to the
	// victim's chest at the kill tick (wallbang or through-floor kills).
	Visible bool `json:"visible"`
	// SmokeBlocked is true when the line crosses a smoke that is live at the kill
	// tick, independent of whether the shot was registered through smoke.
	SmokeBlocked bool `json:"smoke_blocked"`
	// Occluded reports whether geometry data was available for this kill.
	Occluded bool `json:"geometry_checked"`
	// Optics reports whether a replay frame was available to judge aim and flash.
	Optics bool `json:"optics_checked"`
	// AimOffDeg is the angle between the killer's view direction and the line to
	// the victim, in degrees (0 = crosshair exactly on the victim's chest).
	AimOffDeg float64 `json:"aim_off_deg"`
	// InFOV is true when AimOffDeg is inside the attacker's current field of
	// view (narrower while scoped).
	InFOV bool `json:"in_fov"`
	// AttackerZoom / VictimZoom are the scope levels at the kill (0 hip, 1 half,
	// 2 full).
	AttackerZoom uint8 `json:"attacker_zoom"`
	VictimZoom   uint8 `json:"victim_zoom"`
	// VictimSeesKiller is true when the victim's view direction pointed at the
	// killer at the kill tick: false marks an ambush.
	VictimSeesKiller bool `json:"victim_sees_killer"`
	// AttackerBlind / VictimBlind reflect an active flash at the kill tick.
	AttackerBlind bool `json:"attacker_blind"`
	VictimBlind   bool `json:"victim_blind"`
}

// Report is the per-demo kill context list.
type Report struct {
	Map          string  `json:"map"`
	Kills        []Kill  `json:"kills"`
	AvgDistance  float64 `json:"avg_distance"`
	BlindPct     float64 `json:"occluded_pct"`
	SmokePct     float64 `json:"smoke_pct"`
	GeometryUsed bool    `json:"geometry_used"`
	OpticsUsed   bool    `json:"optics_used"`
	// FOVPct is the share of kills where the victim was inside the killer's view.
	// Almost every kill lands with the crosshair on the victim, so this mostly
	// validates the replay yaw; AmbushPct and BlindKills carry the signal.
	FOVPct float64 `json:"fov_pct"`
	// AmbushPct is the share of kills where the victim was not looking at the killer.
	AmbushPct float64 `json:"ambush_pct"`
	// BlindKills counts kills landed while the killer was flashed.
	BlindKills int `json:"blind_kills"`
}

// Build annotates every death in the replay index. mesh may be nil: distances and
// smoke checks still work, visibility is then reported as unknown.
func Build(replays map[int]parser.ReplayRoundData, rounds []storage.MatchRound, players []storage.MatchPlayer, tickRate float64, bvh *mapgeom.BVH, cfg Config) Report {
	if cfg.SmokeRadius <= 0 {
		cfg = DefaultConfig()
	}
	rep := Report{}
	var tracer *mapgeom.Tracer
	if bvh != nil {
		tracer = mapgeom.NewTracer(bvh)
		rep.GeometryUsed = true
	}

	freeze := make(map[int]int64, len(rounds))
	for _, r := range rounds {
		freeze[r.RoundNumber] = r.FreezeEndTick
	}
	names := make(map[uint8]string, len(players))
	for _, p := range players {
		if p.SpecSlot != nil && *p.SpecSlot >= 0 && *p.SpecSlot <= 255 {
			names[uint8(*p.SpecSlot)] = p.PlayerName
		}
	}

	roundNums := make([]int, 0, len(replays))
	for rn := range replays {
		roundNums = append(roundNums, rn)
	}
	sort.Ints(roundNums)

	visible, smoke, distSum := 0, 0, 0.0
	inFOV, ambush, blind, optics := 0, 0, 0, 0
	for _, rn := range roundNums {
		rd := replays[rn]
		base := freeze[rn]
		if base <= 0 && len(rd.Frames) > 0 {
			base = rd.Frames[0].Tick
		}
		for _, d := range rd.Deaths {
			k := Kill{
				Round:      rn,
				Tick:       d.Tick,
				KillerSlot: d.KillerSlot,
				VictimSlot: d.VictimSlot,
				Killer:     names[d.KillerSlot],
				Victim:     names[d.VictimSlot],
				Headshot:   d.Headshot,
				Weapon:     parser.NearestShotWeapon(rd, d.KillerSlot, d.Tick),
				KillerPos:  [3]float64{float64(d.Killer.X), float64(d.Killer.Y), float64(d.Killer.Z)},
				VictimPos:  [3]float64{float64(d.Victim.X), float64(d.Victim.Y), float64(d.Victim.Z)},
			}
			if k.Killer == "" {
				k.Killer = "未知"
			}
			if k.Victim == "" {
				k.Victim = "未知"
			}
			if tickRate > 0 && base > 0 && d.Tick >= base {
				k.TimeSec = float64(d.Tick-base) / tickRate
			}
			eye := [3]float64{float64(d.Killer.X), float64(d.Killer.Y), float64(d.Killer.Z) + cfg.EyeHeight}
			chest := [3]float64{float64(d.Victim.X), float64(d.Victim.Y), float64(d.Victim.Z) + cfg.ChestHeight}
			k.Distance = math.Sqrt(sq(chest[0]-eye[0]) + sq(chest[1]-eye[1]) + sq(chest[2]-eye[2]))
			distSum += k.Distance
			if tracer != nil {
				k.Occluded = true
				k.Visible = !tracer.IsOccluded(eye, chest)
				if k.Visible {
					visible++
				}
			}
			if pose := poseAt(rd.Frames, d.Tick); pose != nil {
				kp, kOK := pose[d.KillerSlot]
				vp, vOK := pose[d.VictimSlot]
				if kOK && vOK {
					k.Optics = true
					optics++
					aim := [3]float64{float64(d.Victim.X) - float64(d.Killer.X), float64(d.Victim.Y) - float64(d.Killer.Y), 0}
					k.AimOffDeg = angleDeg(float64(kp.Yaw), aim)
					k.AttackerZoom = kp.Zoom
					k.VictimZoom = vp.Zoom
					k.InFOV = k.AimOffDeg <= zoomHalfAngle(cfg.FOVHalfAngle, kp.Zoom)
					if k.InFOV {
						inFOV++
					}
					back := [3]float64{float64(d.Killer.X) - float64(d.Victim.X), float64(d.Killer.Y) - float64(d.Victim.Y), 0}
					k.VictimSeesKiller = angleDeg(float64(vp.Yaw), back) <= zoomHalfAngle(cfg.FOVHalfAngle, vp.Zoom)
					if !k.VictimSeesKiller {
						ambush++
					}
					k.AttackerBlind = float64(kp.FlashDur) > cfg.BlindThreshold
					k.VictimBlind = float64(vp.FlashDur) > cfg.BlindThreshold
					if k.AttackerBlind {
						blind++
					}
				}
			}
			for _, s := range rd.Smokes {
				if d.Tick < s.StartTick || (s.EndTick > 0 && d.Tick > s.EndTick) {
					continue
				}
				center := [3]float64{float64(s.Pos.X), float64(s.Pos.Y), float64(s.Pos.Z) + 32}
				if segmentWithin(eye, chest, center, cfg.SmokeRadius) {
					k.SmokeBlocked = true
					smoke++
					break
				}
			}
			rep.Kills = append(rep.Kills, k)
		}
	}
	sort.Slice(rep.Kills, func(i, j int) bool {
		if rep.Kills[i].Round != rep.Kills[j].Round {
			return rep.Kills[i].Round < rep.Kills[j].Round
		}
		return rep.Kills[i].Tick < rep.Kills[j].Tick
	})
	if n := len(rep.Kills); n > 0 {
		rep.AvgDistance = distSum / float64(n)
		rep.SmokePct = float64(smoke) / float64(n) * 100
		if rep.GeometryUsed {
			rep.BlindPct = float64(n-visible) / float64(n) * 100
		}
		if optics > 0 {
			rep.OpticsUsed = true
			rep.FOVPct = float64(inFOV) / float64(optics) * 100
			rep.AmbushPct = float64(ambush) / float64(optics) * 100
			rep.BlindKills = blind
		}
	}
	return rep
}

// poseAt returns the newest frame at or before tick, keyed by player slot.
// zoomHalfAngle narrows the base half angle by scope level. CS2 scoped FOV is
// roughly 20° half angle on the first step and 7.5° on the second; the demo
// itself never exposes the projected FOV, so this stays an approximation.
func zoomHalfAngle(base float64, zoom uint8) float64 {
	switch zoom {
	case 1:
		return 20
	case 2:
		return 7.5
	}
	return base
}

func poseAt(frames []parser.ReplayFrame, tick int64) map[uint8]parser.ReplayPlayerPose {
	var best *parser.ReplayFrame
	for i := range frames {
		if frames[i].Tick > tick {
			break
		}
		best = &frames[i]
	}
	if best == nil {
		if len(frames) == 0 {
			return nil
		}
		best = &frames[0]
	}
	out := make(map[uint8]parser.ReplayPlayerPose, len(best.Players))
	for _, p := range best.Players {
		out[p.Slot] = p
	}
	return out
}

// angleDeg returns the angle between a yaw (degrees, 0 = +X, CCW positive) and a
// direction vector, projected on the XY plane.
func angleDeg(yaw float64, dir [3]float64) float64 {
	length := math.Hypot(dir[0], dir[1])
	if length < 1e-6 {
		return 0
	}
	rad := yaw * math.Pi / 180
	view := [2]float64{math.Cos(rad), math.Sin(rad)}
	target := [2]float64{dir[0] / length, dir[1] / length}
	dot := view[0]*target[0] + view[1]*target[1]
	if dot > 1 {
		dot = 1
	}
	if dot < -1 {
		dot = -1
	}
	return math.Acos(dot) * 180 / math.Pi
}

// segmentWithin reports whether the segment a-b passes within radius of c.
func segmentWithin(a, b, c [3]float64, radius float64) bool {
	ab := [3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
	ac := [3]float64{c[0] - a[0], c[1] - a[1], c[2] - a[2]}
	denom := ab[0]*ab[0] + ab[1]*ab[1] + ab[2]*ab[2]
	if denom == 0 {
		return math.Sqrt(ac[0]*ac[0]+ac[1]*ac[1]+ac[2]*ac[2]) <= radius
	}
	t := (ac[0]*ab[0] + ac[1]*ab[1] + ac[2]*ab[2]) / denom
	t = math.Max(0, math.Min(1, t))
	dx := a[0] + ab[0]*t - c[0]
	dy := a[1] + ab[1]*t - c[1]
	dz := a[2] + ab[2]*t - c[2]
	return math.Sqrt(dx*dx+dy*dy+dz*dz) <= radius
}

func sq(v float64) float64 { return v * v }
