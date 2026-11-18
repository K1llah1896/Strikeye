package parser

// Point2D represents a 2D map coordinate.
type Point2D struct {
	X float32
	Y float32
}

// Point3D represents a 3D map coordinate.
type Point3D struct {
	X float32
	Y float32
	Z float32
}

// GrenadeSample captures a projectile's position at a sampled tick.
type GrenadeSample struct {
	Tick int64
	Pos  Point3D
}

// GrenadeTrack captures one thrown grenade's trajectory during a round.
type GrenadeTrack struct {
	EntityID       int64
	RoundNumber    int
	ThrowerSteamID string
	GrenadeType    string
	ThrowTick      int64
	EndTick        int64
	Points         []GrenadeSample
}

// SmokeEffect captures a smoke detonation and dissipation window.
type SmokeEffect struct {
	EntityID       int64
	RoundNumber    int
	ThrowerSteamID string
	StartTick      int64
	EndTick        int64
	Pos            Point3D
}

// InfernoEffect captures an incendiary/molotov fire polygon and active window.
type InfernoEffect struct {
	UniqueID       int64
	RoundNumber    int
	ThrowerSteamID string
	StartTick      int64
	EndTick        int64
	Hull           []Point2D
}

// GrenadeCollector accumulates per-round grenade trajectories, smokes, and infernos.
type GrenadeCollector struct {
	activeProj map[int64]*GrenadeTrack
	finished   map[int][]*GrenadeTrack
	smokes     map[int][]*SmokeEffect
	openSmoke  map[int64]*SmokeEffect
	infernos   map[int][]*InfernoEffect
	openInf    map[int64]*InfernoEffect
}

// NewGrenadeCollector creates an initialized GrenadeCollector.
func NewGrenadeCollector() *GrenadeCollector {
	return &GrenadeCollector{
		activeProj: make(map[int64]*GrenadeTrack),
		finished:   make(map[int][]*GrenadeTrack),
		smokes:     make(map[int][]*SmokeEffect),
		openSmoke:  make(map[int64]*SmokeEffect),
		infernos:   make(map[int][]*InfernoEffect),
		openInf:    make(map[int64]*InfernoEffect),
	}
}

func (gc *GrenadeCollector) StartProjectile(entityID int64, roundNum int, thrower, grenadeType string, tick int64, pos Point3D) {
	tr := &GrenadeTrack{
		EntityID:       entityID,
		RoundNumber:    roundNum,
		ThrowerSteamID: thrower,
		GrenadeType:    grenadeType,
		ThrowTick:      tick,
		EndTick:        tick,
		Points:         []GrenadeSample{{Tick: tick, Pos: pos}},
	}
	gc.activeProj[entityID] = tr
	gc.finished[roundNum] = append(gc.finished[roundNum], tr)
}

func (gc *GrenadeCollector) SampleProjectile(entityID int64, tick int64, pos Point3D) {
	if tr, ok := gc.activeProj[entityID]; ok {
		tr.Points = append(tr.Points, GrenadeSample{Tick: tick, Pos: pos})
		tr.EndTick = tick
	}
}

func (gc *GrenadeCollector) EndProjectile(entityID int64, tick int64) {
	if tr, ok := gc.activeProj[entityID]; ok {
		tr.EndTick = tick
		delete(gc.activeProj, entityID)
	}
}

func (gc *GrenadeCollector) StartSmoke(entityID int64, roundNum int, thrower string, tick int64, pos Point3D) {
	s := &SmokeEffect{
		EntityID:       entityID,
		RoundNumber:    roundNum,
		ThrowerSteamID: thrower,
		StartTick:      tick,
		EndTick:        tick,
		Pos:            pos,
	}
	gc.openSmoke[entityID] = s
	gc.smokes[roundNum] = append(gc.smokes[roundNum], s)
}

func (gc *GrenadeCollector) EndSmoke(entityID int64, tick int64) {
	if s, ok := gc.openSmoke[entityID]; ok {
		s.EndTick = tick
		delete(gc.openSmoke, entityID)
	}
}

func (gc *GrenadeCollector) StartInferno(uniqueID int64, roundNum int, thrower string, tick int64, hull []Point2D) {
	inf := &InfernoEffect{
		UniqueID:       uniqueID,
		RoundNumber:    roundNum,
		ThrowerSteamID: thrower,
		StartTick:      tick,
		EndTick:        tick,
		Hull:           append([]Point2D(nil), hull...),
	}
	gc.openInf[uniqueID] = inf
	gc.infernos[roundNum] = append(gc.infernos[roundNum], inf)
}

func (gc *GrenadeCollector) UpdateInfernoHull(uniqueID int64, tick int64, hull []Point2D) {
	if inf, ok := gc.openInf[uniqueID]; ok {
		if len(hull) > 0 {
			inf.Hull = append([]Point2D(nil), hull...)
		}
		inf.EndTick = tick
	}
}

func (gc *GrenadeCollector) EndInferno(uniqueID int64, tick int64) {
	if inf, ok := gc.openInf[uniqueID]; ok {
		inf.EndTick = tick
		delete(gc.openInf, uniqueID)
	}
}

// RoundEffects returns all collected grenade tracks, smokes, and infernos for roundNum.
func (gc *GrenadeCollector) RoundEffects(roundNum int) ([]GrenadeTrack, []SmokeEffect, []InfernoEffect) {
	var gOut []GrenadeTrack
	for _, g := range gc.finished[roundNum] {
		gOut = append(gOut, *g)
	}
	var sOut []SmokeEffect
	for _, s := range gc.smokes[roundNum] {
		sOut = append(sOut, *s)
	}
	var iOut []InfernoEffect
	for _, inf := range gc.infernos[roundNum] {
		iOut = append(iOut, *inf)
	}
	return gOut, sOut, iOut
}
