package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"

	"strikeye/internal/mapgeom"
	"strikeye/internal/storage"
	"strikeye/internal/version"
	"strikeye/internal/watcher"
)

// ParseProgress reports incremental single-pass progress.
type ParseProgress struct {
	DemoID  int64
	Percent float64
	Tick    int64
	Stage   string
}

// ParseOptions configures ParseAndPersist.
type ParseOptions struct {
	DataRoot      string
	DemoID        int64
	FilePath      string
	PreBufferSec  float64
	PostBufferSec float64
	Force         bool
	// GeometryBundle optionally points at a map_geometry.zip sidecar; when set,
	// visibility geometry is pulled from it on demand instead of only from the
	// DataRoot cache.
	GeometryBundle string
	OnProgress     func(ParseProgress)
}

// ParseSummary summarizes the persisted output of a single-pass parse.
type ParseSummary struct {
	Demo                storage.Demo
	ReusedCache         bool
	PlayerCount         int
	RoundCount          int
	EventCount          int
	HighlightCount      int
	InputDataCapability string
	ReplayCachePath     string
}

// ParseAndPersist executes a single-pass parse (or reuses cached results if already parsed)
// and persists match_players, match_rounds, match_events, highlight_clips, and CS2RPL01.
func ParseAndPersist(ctx context.Context, db *storage.DB, opts ParseOptions) (ParseSummary, error) {
	if db == nil {
		return ParseSummary{}, errors.New("database is required")
	}
	if opts.DataRoot == "" {
		return ParseSummary{}, errors.New("data_root is required")
	}

	demo, err := resolveDemoRecord(ctx, db, opts)
	if err != nil {
		return ParseSummary{}, err
	}

	// Reuse existing parsed records and .rpl cache unless forced.
	if !opts.Force && demo.ParseStatus == "parsed" && demo.ReplayCachePath != nil && *demo.ReplayCachePath != "" {
		if _, statErr := os.Stat(*demo.ReplayCachePath); statErr == nil {
			players, _ := db.Analysis().ListMatchPlayers(ctx, demo.ID)
			rounds, _ := db.Analysis().ListMatchRounds(ctx, demo.ID)
			evts, _ := db.Analysis().ListMatchEvents(ctx, demo.ID, nil)
			clips, _ := db.Analysis().ListHighlightClips(ctx, demo.ID)
			return ParseSummary{
				Demo:                demo,
				ReusedCache:         true,
				PlayerCount:         len(players),
				RoundCount:          len(rounds),
				EventCount:          len(evts),
				HighlightCount:      len(clips),
				InputDataCapability: demo.InputDataCapability,
				ReplayCachePath:     *demo.ReplayCachePath,
			}, nil
		}
	}

	// Mark parsing in progress.
	demo.ParseStatus = "parsing"
	demo.ParseError = nil
	if err := db.Demos().UpdateParseResult(ctx, demo); err != nil {
		return ParseSummary{}, fmt.Errorf("mark parsing: %w", err)
	}

	res, parseErr := executeSinglePass(ctx, demo, opts)
	if parseErr != nil {
		errMsg := parseErr.Error()
		demo.ParseStatus = "failed"
		demo.ParseError = &errMsg
		_ = db.Demos().UpdateParseResult(ctx, demo)
		return ParseSummary{}, parseErr
	}

	rplPath := filepath.Join(opts.DataRoot, "cache", "replay_32hz", demo.ContentSHA256+".rpl")
	if err := WriteReplayFileAtomic(rplPath, float32(res.tickRate), res.inputCap, res.replayRounds); err != nil {
		errMsg := fmt.Sprintf("write CS2RPL01: %v", err)
		demo.ParseStatus = "failed"
		demo.ParseError = &errMsg
		_ = db.Demos().UpdateParseResult(ctx, demo)
		return ParseSummary{}, fmt.Errorf("%s", errMsg)
	}

	// Persist parsed tables.
	if err := db.Analysis().ReplaceMatchPlayers(ctx, demo.ID, res.players); err != nil {
		return ParseSummary{}, fmt.Errorf("persist match_players: %w", err)
	}
	if err := db.Analysis().ReplaceMatchPlayerSides(ctx, demo.ID, res.playerSides); err != nil {
		return ParseSummary{}, fmt.Errorf("persist match_player_sides: %w", err)
	}
	if err := db.Analysis().ReplaceMatchRounds(ctx, demo.ID, res.rounds); err != nil {
		return ParseSummary{}, fmt.Errorf("persist match_rounds: %w", err)
	}
	if err := db.Analysis().ReplaceMatchPurchases(ctx, demo.ID, res.purchases); err != nil {
		return ParseSummary{}, fmt.Errorf("persist match_purchases: %w", err)
	}
	if err := db.Analysis().ReplaceMatchEvents(ctx, demo.ID, res.events); err != nil {
		return ParseSummary{}, fmt.Errorf("persist match_events: %w", err)
	}
	if err := db.Analysis().ReplaceHighlightClips(ctx, demo.ID, res.highlights); err != nil {
		return ParseSummary{}, fmt.Errorf("persist highlight_clips: %w", err)
	}

	nowISO := time.Now().UTC().Format(time.RFC3339Nano)
	pVer := version.ParserBaseline + "+" + version.ParserBaselineVersion
	demo.MapName = &res.mapName
	demo.TickRate = res.tickRate
	demo.MatchStartTick = res.matchStartTick
	demo.DemoEndTick = res.demoEndTick
	demo.TotalRounds = len(res.rounds)
	demo.HalftimeRound = res.halftimeRound
	demo.TeamAName = res.teamAName
	demo.TeamBName = res.teamBName
	demo.TeamAScore = res.teamAScore
	demo.TeamBScore = res.teamBScore
	demo.DurationSec = res.durationSec
	demo.ParseStatus = "parsed"
	demo.ParseError = nil
	demo.ParserVersion = &pVer
	demo.InputDataCapability = res.inputCap
	demo.ReplayCachePath = &rplPath
	demo.ParsedAt = &nowISO

	if err := db.Demos().UpdateParseResult(ctx, demo); err != nil {
		return ParseSummary{}, fmt.Errorf("update demo parse result: %w", err)
	}

	return ParseSummary{
		Demo:                demo,
		ReusedCache:         false,
		PlayerCount:         len(res.players),
		RoundCount:          len(res.rounds),
		EventCount:          len(res.events),
		HighlightCount:      len(res.highlights),
		InputDataCapability: res.inputCap,
		ReplayCachePath:     rplPath,
	}, nil
}

func resolveDemoRecord(ctx context.Context, db *storage.DB, opts ParseOptions) (storage.Demo, error) {
	if opts.DemoID > 0 {
		return db.Demos().GetByID(ctx, opts.DemoID)
	}
	if opts.FilePath == "" {
		return storage.Demo{}, errors.New("either DemoID or FilePath is required")
	}
	clean := filepath.Clean(opts.FilePath)
	info, err := os.Stat(clean)
	if err != nil {
		return storage.Demo{}, fmt.Errorf("stat demo: %w", err)
	}
	if err := watcher.ValidateDemoFile(clean, info.Size(), 1024); err != nil {
		return storage.Demo{}, err
	}
	sha, err := watcher.StreamSHA256(clean)
	if err != nil {
		return storage.Demo{}, err
	}
	demo, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:       clean,
		Filename:       filepath.Base(clean),
		FileSize:       info.Size(),
		ContentSHA256:  sha,
		SourcePlatform: watcher.ClassifyPlatform(clean),
	})
	return demo, err
}

type singlePassOutput struct {
	mapName        string
	tickRate       float64
	matchStartTick int64
	demoEndTick    int64
	halftimeRound  *int
	teamAName      string
	teamBName      string
	teamAScore     int
	teamBScore     int
	durationSec    float64
	inputCap       string
	players        []storage.MatchPlayer
	playerSides    []storage.MatchPlayerSide
	rounds         []storage.MatchRound
	purchases      []storage.MatchPurchase
	events         []storage.MatchEvent
	highlights     []storage.HighlightClip
	replayRounds   []ReplayRoundData
}

func executeSinglePass(ctx context.Context, demo storage.Demo, opts ParseOptions) (singlePassOutput, error) {
	f, err := os.Open(demo.FilePath)
	if err != nil {
		return singlePassOutput{}, fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()

	p := demoinfocs.NewParser(f)
	defer p.Close()

	var (
		tickRate       = 64.0
		mapName        = ""
		matchStartTick int64
		teamAName      = "Team A"
		teamBName      = "Team B"

		rt           = NewRoundTracker(tickRate)
		gc           = NewGrenadeCollector()
		ic           = NewInputCollector()
		teamABySteam = make(map[string]string) // steamID64 -> "team_a" / "team_b"
		playerMeta   = make(map[string]PlayerMeta)
		playerSlots  = make(map[string]uint8)
		equipValues  = make(map[string][]int)

		kills           []KillEvent
		damages         []DamageEvent
		flashes         []FlashEvent
		defuses         []DefuseEvent
		bombPlantTicks  = make(map[int]int64)
		matchEvents     []storage.MatchEvent
		framesByRound   = make(map[int][]ReplayFrame)
		shotsByRound    = make(map[int][]ReplayShotEvent)
		deathsByRound   = make(map[int][]ReplayDeathEvent)
		bombsByRound    = make(map[int][]ReplayBombEvent)
		plantPosByRound = make(map[int]Point3D)
		activeBombSite  string
		visSamples      []visSample
		ttdSamples      []TTDSample
		ttdSeen         = make(map[string]bool)
		visTracer       *mapgeom.Tracer
		visGeoLoaded    bool
		lastSampleTick  int64 = -1
		lastProgTick    int64 = -1
	)

	resetState := func() {
		rt.ResetMatch()
		gc = NewGrenadeCollector()
		ic = NewInputCollector()
		kills = nil
		damages = nil
		defuses = nil
		matchEvents = nil
		framesByRound = make(map[int][]ReplayFrame)
		shotsByRound = make(map[int][]ReplayShotEvent)
		deathsByRound = make(map[int][]ReplayDeathEvent)
		bombsByRound = make(map[int][]ReplayBombEvent)
		plantPosByRound = make(map[int]Point3D)
		activeBombSite = ""
		visSamples = nil
		ttdSeen = make(map[string]bool)
	}

	resolveTeamKey := func(pl *common.Player) string {
		if pl == nil {
			return ""
		}
		sid := steamIDStr(pl)
		if sid != "" {
			if tk, ok := teamABySteam[sid]; ok {
				return tk
			}
		}
		if rw := rt.Active(); rw != nil {
			switch pl.Team {
			case common.TeamCounterTerrorists:
				if rw.TeamASide == "CT" {
					return "team_a"
				}
				return "team_b"
			case common.TeamTerrorists:
				if rw.TeamASide == "T" {
					return "team_a"
				}
				return "team_b"
			}
		}
		return ""
	}

	inferSidesFromPlayers := func(gs demoinfocs.GameState) (string, string) {
		if len(teamABySteam) == 0 {
			// First official round: assign CT -> team_a, T -> team_b.
			for _, pl := range gs.Participants().Playing() {
				sid := steamIDStr(pl)
				if sid == "" {
					continue
				}
				switch pl.Team {
				case common.TeamCounterTerrorists:
					teamABySteam[sid] = "team_a"
				case common.TeamTerrorists:
					teamABySteam[sid] = "team_b"
				}
			}
			if ct := gs.TeamCounterTerrorists(); ct != nil && strings.TrimSpace(ct.ClanName()) != "" {
				teamAName = strings.TrimSpace(ct.ClanName())
			}
			if tSide := gs.TeamTerrorists(); tSide != nil && strings.TrimSpace(tSide.ClanName()) != "" {
				teamBName = strings.TrimSpace(tSide.ClanName())
			}
			return "CT", "T"
		}

		// Vote current side of team_a players.
		ctVotes, tVotes := 0, 0
		for _, pl := range gs.Participants().Playing() {
			sid := steamIDStr(pl)
			if teamABySteam[sid] == "team_a" {
				if pl.Team == common.TeamCounterTerrorists {
					ctVotes++
				} else if pl.Team == common.TeamTerrorists {
					tVotes++
				}
			}
		}
		if tVotes > ctVotes {
			return "T", "CT"
		}
		return "CT", "T"
	}

	recordParticipants := func(gs demoinfocs.GameState) {
		for idx, pl := range gs.Participants().Playing() {
			sid := steamIDStr(pl)
			if sid == "" {
				continue
			}
			tk := resolveTeamKey(pl)
			slot := uint8(idx + 1)
			if pl.Entity != nil {
				slot = uint8(pl.Entity.ID())
			}
			playerSlots[sid] = slot
			slotInt := int(slot)
			playerMeta[sid] = PlayerMeta{
				SteamID64:  sid,
				PlayerName: pl.Name,
				TeamKey:    tk,
				SpecSlot:   &slotInt,
			}
		}
	}

	p.RegisterNetMessageHandler(func(m *msg.CDemoFileHeader) {
		if m != nil && m.GetMapName() != "" {
			mapName = m.GetMapName()
		}
	})
	p.RegisterNetMessageHandler(func(m *msg.CSVCMsg_UserCommands) {
		if m == nil || len(m.GetCommands()) == 0 {
			return
		}
		ic.ObserveUserCmd()
		rw := rt.Active()
		if rw == nil {
			return
		}
		tick := int64(p.GameState().IngameTick())
		for _, cmd := range m.GetCommands() {
			btn, ok := DecodeUserCmdButtonState(cmd.GetData())
			if !ok {
				continue
			}
			slot := uint8(cmd.GetPlayerSlot() + 1)
			ic.ObserveButtons(rw.RoundNumber, tick, slot, ButtonsFromBitmask(btn))
		}
	})
	p.RegisterNetMessageHandler(func(m *msg.CDemoUserCmd) {
		if m == nil || len(m.GetData()) == 0 {
			return
		}
		ic.ObserveUserCmd()
	})

	p.RegisterEventHandler(func(e events.TickRateInfoAvailable) {
		if e.TickRate > 0 {
			tickRate = e.TickRate
			rt.tickRate = tickRate
		}
	})

	p.RegisterEventHandler(func(e events.MatchStartedChanged) {
		if e.NewIsStarted && !e.OldIsStarted {
			resetState()
			matchStartTick = int64(p.GameState().IngameTick())
		}
	})

	p.RegisterEventHandler(func(e events.RoundStart) {
		gs := p.GameState()
		if gs.IsWarmupPeriod() || !gs.IsMatchStarted() {
			return
		}
		if tr := p.TickRate(); tr > 0 {
			tickRate = tr
			rt.tickRate = tr
		}
		if matchStartTick == 0 {
			matchStartTick = int64(gs.IngameTick())
		}
		activeBombSite = ""
		teamASide, teamBSide := inferSidesFromPlayers(gs)
		rt.StartRound(int64(gs.IngameTick()), teamASide, teamBSide)
		recordParticipants(gs)
	})

	p.RegisterEventHandler(func(e events.ItemPickup) {
		gs := p.GameState()
		if gs.IsWarmupPeriod() || !gs.IsMatchStarted() {
			return
		}
		if e.Player == nil || e.Weapon == nil || !e.Player.IsAlive() {
			return
		}
		name := e.Weapon.String()
		if IsDefaultLoadout(name) || KnifeVariant(name) {
			return
		}
		cost, ok := WeaponPrice(name)
		if !ok {
			return
		}
		rt.RecordPurchase(int64(gs.IngameTick()), PlayerPurchase{
			SteamID64:  steamIDStr(e.Player),
			PlayerName: e.Player.Name,
			Weapon:     name,
			Cost:       cost,
		})
	})

	p.RegisterEventHandler(func(e events.RoundFreezetimeEnd) {
		gs := p.GameState()
		if gs.IsWarmupPeriod() || !gs.IsMatchStarted() {
			return
		}
		rw := rt.Active()
		if rw == nil {
			return
		}
		rt.MarkFreezeEnd(int64(gs.IngameTick()))
		recordParticipants(gs)

		var econA, econB []string
		var equipA, equipB int
		var peList []PlayerRoundEconomy

		for _, pl := range gs.Participants().Playing() {
			sid := steamIDStr(pl)
			if sid == "" {
				continue
			}
			tk := resolveTeamKey(pl)
			if tk != "team_a" && tk != "team_b" {
				continue
			}
			ev := pl.EquipmentValueCurrent()
			spent := pl.MoneySpentThisRound()
			moneyRem := pl.Money()
			acctStart := moneyRem + spent
			etype := ClassifyPlayerEconomy(rw.RoundNumber, rt.HalftimeRound(), ev, acctStart, spent)
			side := ""
			if pl.Team == common.TeamCounterTerrorists {
				side = "CT"
			} else if pl.Team == common.TeamTerrorists {
				side = "T"
			}
			equipValues[sid] = append(equipValues[sid], ev)
			peList = append(peList, PlayerRoundEconomy{
				SteamID64:      sid,
				PlayerName:     pl.Name,
				TeamKey:        tk,
				Side:           side,
				EquipValue:     ev,
				AccountStart:   acctStart,
				CashSpent:      spent,
				MoneyRemaining: moneyRem,
				EconomyType:    etype,
				Weapons:        extractPlayerEquipment(pl),
				HasArmor:       pl.Armor() > 0,
				HasHelmet:      pl.HasHelmet(),
				HasDefuseKit:   pl.HasDefuseKit(),
			})
			if tk == "team_a" {
				equipA += ev
				econA = append(econA, etype)
			} else {
				equipB += ev
				econB = append(econB, etype)
			}
		}
		rw.TeamAEquipValue = equipA
		rw.TeamBEquipValue = equipB
		rw.TeamAEconomyType = VoteTeamEconomy(econA)
		rw.TeamBEconomyType = VoteTeamEconomy(econB)
		rw.PlayerEconomy = peList
	})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		gs := p.GameState()
		if gs.IsWarmupPeriod() || !gs.IsMatchStarted() || rt.Active() == nil {
			return
		}
		winSide := ""
		switch e.Winner {
		case common.TeamCounterTerrorists:
			winSide = "CT"
		case common.TeamTerrorists:
			winSide = "T"
		}
		rt.EndRound(int64(gs.IngameTick()), winSide, fmt.Sprintf("%v", e.Reason), activeBombSite)
	})

	p.RegisterEventHandler(func(e events.BombPlanted) {
		rw := rt.Active()
		if rw == nil {
			return
		}
		site := string(e.Site)
		activeBombSite = site
		tick := int64(p.GameState().IngameTick())
		if _, exists := bombPlantTicks[rw.RoundNumber]; !exists {
			bombPlantTicks[rw.RoundNumber] = tick
		}
		var actorSID, actorName, actorTK *string
		if e.Player != nil {
			s, n, tk := steamIDStr(e.Player), e.Player.Name, resolveTeamKey(e.Player)
			actorSID, actorName, actorTK = &s, &n, &tk
			pp := e.Player.Position()
			pos := Point3D{X: float32(pp.X), Y: float32(pp.Y), Z: float32(pp.Z)}
			plantPosByRound[rw.RoundNumber] = pos
			bombsByRound[rw.RoundNumber] = append(bombsByRound[rw.RoundNumber], ReplayBombEvent{
				Tick: tick, Kind: ReplayBombPlant, Pos: pos,
			})
		}
		matchEvents = append(matchEvents, storage.MatchEvent{
			RoundNumber:    rw.RoundNumber,
			Tick:           tick,
			RoundTimeSec:   roundTimeSec(tick, rw.FreezeEndTick, tickRate),
			EventType:      "bomb_planted",
			ActorSteamID64: actorSID,
			ActorName:      actorName,
			ActorTeamKey:   actorTK,
			Site:           &site,
		})
	})

	p.RegisterEventHandler(func(e events.BombDefused) {
		rw := rt.Active()
		if rw == nil {
			return
		}
		gs := p.GameState()
		tick := int64(gs.IngameTick())
		site := string(e.Site)
		var actorSID, actorName, actorTK *string
		enemiesAlive := 0
		if e.Player != nil {
			s, n, tk := steamIDStr(e.Player), e.Player.Name, resolveTeamKey(e.Player)
			actorSID, actorName, actorTK = &s, &n, &tk
			pp := e.Player.Position()
			bombsByRound[rw.RoundNumber] = append(bombsByRound[rw.RoundNumber], ReplayBombEvent{
				Tick: tick, Kind: ReplayBombDefuse,
				Pos: Point3D{X: float32(pp.X), Y: float32(pp.Y), Z: float32(pp.Z)},
			})
			for _, pl := range gs.Participants().Playing() {
				if pl.IsAlive() && pl.Team != e.Player.Team {
					enemiesAlive++
				}
			}
			defuses = append(defuses, DefuseEvent{
				RoundNumber:   rw.RoundNumber,
				Tick:          tick,
				PlayerSteamID: s,
				PlayerName:    n,
				TeamKey:       tk,
				EnemiesAlive:  enemiesAlive,
			})
		}
		matchEvents = append(matchEvents, storage.MatchEvent{
			RoundNumber:    rw.RoundNumber,
			Tick:           tick,
			RoundTimeSec:   roundTimeSec(tick, rw.FreezeEndTick, tickRate),
			EventType:      "bomb_defused",
			ActorSteamID64: actorSID,
			ActorName:      actorName,
			ActorTeamKey:   actorTK,
			Site:           &site,
		})
	})

	p.RegisterEventHandler(func(e events.BombExplode) {
		rw := rt.Active()
		if rw == nil {
			return
		}
		tick := int64(p.GameState().IngameTick())
		site := string(e.Site)
		if pos, ok := plantPosByRound[rw.RoundNumber]; ok {
			bombsByRound[rw.RoundNumber] = append(bombsByRound[rw.RoundNumber], ReplayBombEvent{
				Tick: tick, Kind: ReplayBombExplode, Pos: pos,
			})
		}
		matchEvents = append(matchEvents, storage.MatchEvent{
			RoundNumber:  rw.RoundNumber,
			Tick:         tick,
			RoundTimeSec: roundTimeSec(tick, rw.FreezeEndTick, tickRate),
			EventType:    "bomb_exploded",
			Site:         &site,
		})
	})

	p.RegisterEventHandler(func(e events.WeaponFire) {
		rw := rt.Active()
		if rw == nil || e.Shooter == nil || e.Weapon == nil {
			return
		}
		sid := steamIDStr(e.Shooter)
		slot, ok := playerSlots[sid]
		if !ok {
			return
		}
		pos := e.Shooter.Position()
		yaw := float64(e.Shooter.ViewDirectionX()) * math.Pi / 180.0
		pitch := float64(e.Shooter.ViewDirectionY()) * math.Pi / 180.0
		dirX := math.Cos(yaw) * math.Cos(pitch)
		dirY := math.Sin(yaw) * math.Cos(pitch)
		dirZ := -math.Sin(pitch)
		const traceLen = 1024.0
		shotsByRound[rw.RoundNumber] = append(shotsByRound[rw.RoundNumber], ReplayShotEvent{
			Tick:       int64(p.GameState().IngameTick()),
			PlayerSlot: slot,
			Weapon:     strings.ToLower(e.Weapon.String()),
			Start:      Point3D{X: float32(pos.X), Y: float32(pos.Y), Z: float32(pos.Z + 64)},
			End: Point3D{
				X: float32(pos.X + dirX*traceLen),
				Y: float32(pos.Y + dirY*traceLen),
				Z: float32(pos.Z + 64 + dirZ*traceLen),
			},
		})
	})

	p.RegisterEventHandler(func(e events.PlayerHurt) {
		rw := rt.Active()
		if rw == nil || e.Player == nil {
			return
		}
		attSID, attTK := "", ""
		if e.Attacker != nil {
			attSID = steamIDStr(e.Attacker)
			attTK = resolveTeamKey(e.Attacker)
		}
		wep := ""
		if e.Weapon != nil {
			wep = strings.ToLower(e.Weapon.String())
		}
		vicSID := steamIDStr(e.Player)
		vicTK := resolveTeamKey(e.Player)
		damages = append(damages, DamageEvent{
			RoundNumber:     rw.RoundNumber,
			Tick:            int64(p.GameState().IngameTick()),
			AttackerSteamID: attSID,
			AttackerTeamKey: attTK,
			VictimSteamID:   vicSID,
			VictimTeamKey:   vicTK,
			Weapon:          wep,
			RawHealthDamage: e.HealthDamage,
		})

		if visTracer == nil || attSID == "" || vicSID == "" || attSID == vicSID || e.HealthDamage <= 0 {
			return
		}
		if attTK != "" && vicTK != "" && attTK == vicTK {
			return
		}
		tick := int64(p.GameState().IngameTick())
		key := fmt.Sprintf("%d|%s|%s", rw.RoundNumber, attSID, vicSID)
		if ttdSeen[key] {
			return
		}
		if ms, ok := visibleRunMS(visTracer, visSamples, attSID, vicSID, tick, int64(math.Round(5*tickRate)), tickRate); ok {
			ttdSeen[key] = true
			ttdSamples = append(ttdSamples, TTDSample{
				RoundNumber:     rw.RoundNumber,
				AttackerSteamID: attSID,
				DurationMS:      ms,
			})
		}
	})

	p.RegisterEventHandler(func(e events.PlayerFlashed) {
		rw := rt.Active()
		if rw == nil || e.Player == nil || e.Attacker == nil {
			return
		}
		attSID := steamIDStr(e.Attacker)
		vicSID := steamIDStr(e.Player)
		if attSID == "" || vicSID == "" || attSID == vicSID {
			return
		}
		attTK := resolveTeamKey(e.Attacker)
		vicTK := resolveTeamKey(e.Player)
		if (attTK != "" && vicTK != "" && attTK == vicTK) || e.Attacker.Team == e.Player.Team {
			return
		}
		durSec := e.FlashDuration().Seconds()
		if durSec <= 0 {
			return
		}
		flashes = append(flashes, FlashEvent{
			RoundNumber:     rw.RoundNumber,
			Tick:            int64(p.GameState().IngameTick()),
			AttackerSteamID: attSID,
			AttackerTeamKey: attTK,
			VictimSteamID:   vicSID,
			VictimTeamKey:   vicTK,
			DurationSec:     durSec,
		})
	})

	p.RegisterEventHandler(func(e events.Kill) {
		rw := rt.Active()
		if rw == nil || e.Victim == nil {
			return
		}
		tick := int64(p.GameState().IngameTick())
		var attSID, attName, attTK string
		var attHP int
		var airborne bool
		if e.Killer != nil {
			attSID = steamIDStr(e.Killer)
			attName = e.Killer.Name
			attTK = resolveTeamKey(e.Killer)
			attHP = e.Killer.Health()
			airborne = e.Killer.IsAirborne()
		}
		vicSID := steamIDStr(e.Victim)
		vicName := e.Victim.Name
		vicTK := resolveTeamKey(e.Victim)

		var assSID, assName string
		if e.Assister != nil {
			assSID = steamIDStr(e.Assister)
			assName = e.Assister.Name
		}

		wep := ""
		if e.Weapon != nil {
			wep = strings.ToLower(e.Weapon.String())
		}
		dist := 0.0
		if e.Killer != nil {
			kp, vp := e.Killer.Position(), e.Victim.Position()
			dist = kp.Distance(vp)
		}
		teamKill := attTK != "" && vicTK != "" && attTK == vicTK

		ke := KillEvent{
			RoundNumber:     rw.RoundNumber,
			Tick:            tick,
			RoundTimeSec:    roundTimeSec(tick, rw.FreezeEndTick, tickRate),
			AttackerSteamID: attSID,
			AttackerName:    attName,
			AttackerTeamKey: attTK,
			VictimSteamID:   vicSID,
			VictimName:      vicName,
			VictimTeamKey:   vicTK,
			AssisterSteamID: assSID,
			AssisterName:    assName,
			Weapon:          wep,
			IsHeadshot:      e.IsHeadshot,
			IsWallbang:      e.PenetratedObjects > 0,
			IsThroughSmoke:  e.ThroughSmoke,
			IsNoscope:       e.NoScope,
			IsAttackerBlind: e.AttackerBlind,
			IsAssistedFlash: e.AssistedFlash,
			IsAirborne:      airborne,
			Distance:        dist,
			AttackerHP:      attHP,
			TeamKill:        teamKill,
		}
		kills = append(kills, ke)

		if vicSID != "" {
			vp := e.Victim.Position()
			kp := vp
			killerSlot := uint8(0)
			if e.Killer != nil {
				kp = e.Killer.Position()
				killerSlot = playerSlots[attSID]
			}
			deathsByRound[rw.RoundNumber] = append(deathsByRound[rw.RoundNumber], ReplayDeathEvent{
				Tick:       tick,
				KillerSlot: killerSlot,
				VictimSlot: playerSlots[vicSID],
				Headshot:   e.IsHeadshot,
				Victim:     Point3D{X: float32(vp.X), Y: float32(vp.Y), Z: float32(vp.Z)},
				Killer:     Point3D{X: float32(kp.X), Y: float32(kp.Y), Z: float32(kp.Z)},
			})
		}

		var extraJSONPtr *string
		{
			coords := map[string]float64{}
			if e.Killer != nil {
				kp := e.Killer.Position()
				coords["pos_x"] = kp.X
				coords["pos_y"] = kp.Y
			}
			if e.Victim != nil {
				vp := e.Victim.Position()
				coords["victim_x"] = vp.X
				coords["victim_y"] = vp.Y
			}
			if len(coords) > 0 {
				if raw, mErr := json.Marshal(coords); mErr == nil {
					s := string(raw)
					extraJSONPtr = &s
				}
			}
		}

		matchEvents = append(matchEvents, storage.MatchEvent{
			RoundNumber:     rw.RoundNumber,
			Tick:            tick,
			RoundTimeSec:    ke.RoundTimeSec,
			EventType:       "kill",
			ActorSteamID64:  strPtrIfNonEmpty(attSID),
			ActorName:       strPtrIfNonEmpty(attName),
			ActorTeamKey:    strPtrIfNonEmpty(attTK),
			TargetSteamID64: strPtrIfNonEmpty(vicSID),
			TargetName:      strPtrIfNonEmpty(vicName),
			TargetTeamKey:   strPtrIfNonEmpty(vicTK),
			AssisterSteamID: strPtrIfNonEmpty(assSID),
			AssisterName:    strPtrIfNonEmpty(assName),
			Weapon:          strPtrIfNonEmpty(wep),
			IsHeadshot:      e.IsHeadshot,
			IsWallbang:      e.PenetratedObjects > 0,
			IsThroughSmoke:  e.ThroughSmoke,
			IsNoscope:       e.NoScope,
			IsAttackerBlind: e.AttackerBlind,
			IsAssistedFlash: e.AssistedFlash,
			ExtraJSON:       extraJSONPtr,
		})
	})

	p.RegisterEventHandler(func(e events.GrenadeProjectileThrow) {
		rw := rt.Active()
		if rw == nil || e.Projectile == nil {
			return
		}
		thrower := ""
		if e.Projectile.Thrower != nil {
			thrower = steamIDStr(e.Projectile.Thrower)
		}
		gType := ""
		if e.Projectile.WeaponInstance != nil {
			gType = strings.ToLower(e.Projectile.WeaponInstance.String())
		}
		pos := e.Projectile.Position()
		gc.StartProjectile(e.Projectile.UniqueID(), rw.RoundNumber, thrower, gType, int64(p.GameState().IngameTick()), Point3D{float32(pos.X), float32(pos.Y), float32(pos.Z)})
	})

	p.RegisterEventHandler(func(e events.GrenadeProjectileDestroy) {
		if e.Projectile == nil {
			return
		}
		gc.EndProjectile(e.Projectile.UniqueID(), int64(p.GameState().IngameTick()))
	})

	p.RegisterEventHandler(func(e events.SmokeStart) {
		rw := rt.Active()
		if rw == nil {
			return
		}
		thrower := ""
		if e.Thrower != nil {
			thrower = steamIDStr(e.Thrower)
		}
		gc.StartSmoke(int64(e.GrenadeEntityID), rw.RoundNumber, thrower, int64(p.GameState().IngameTick()), Point3D{float32(e.Position.X), float32(e.Position.Y), float32(e.Position.Z)})
	})

	p.RegisterEventHandler(func(e events.SmokeExpired) {
		gc.EndSmoke(int64(e.GrenadeEntityID), int64(p.GameState().IngameTick()))
	})

	p.RegisterEventHandler(func(e events.FireGrenadeStart) {
		rw := rt.Active()
		if rw == nil {
			return
		}
		thrower := ""
		if e.Thrower != nil {
			thrower = steamIDStr(e.Thrower)
		}
		gc.StartInferno(int64(e.GrenadeEntityID), rw.RoundNumber, thrower, int64(p.GameState().IngameTick()), nil)
	})

	p.RegisterEventHandler(func(e events.FireGrenadeExpired) {
		gc.EndInferno(int64(e.GrenadeEntityID), int64(p.GameState().IngameTick()))
	})

	p.RegisterEventHandler(func(_ events.FrameDone) {
		if ctx.Err() != nil {
			p.Cancel()
			return
		}
		gs := p.GameState()
		tick := int64(gs.IngameTick())

		if !visGeoLoaded && mapName != "" && gs.IsMatchStarted() {
			visGeoLoaded = true
			if bvh, ok := mapgeom.LoadForMapWithBundle(opts.DataRoot, opts.GeometryBundle, mapName); ok {
				visTracer = mapgeom.NewTracer(bvh)
			}
		}

		if opts.OnProgress != nil && (lastProgTick < 0 || tick-lastProgTick >= 4096) {
			lastProgTick = tick
			opts.OnProgress(ParseProgress{
				DemoID:  demo.ID,
				Percent: float64(p.Progress()) * 85.0,
				Tick:    tick,
				Stage:   "扫描 Demo 帧数据",
			})
		}

		rw := rt.Active()
		if rw == nil || gs.IsWarmupPeriod() || !gs.IsMatchStarted() {
			return
		}

		// Record raw-tick button transitions on every tick.
		for _, pl := range gs.Participants().Playing() {
			if !pl.IsAlive() {
				continue
			}
			sid := steamIDStr(pl)
			if sid == "" {
				continue
			}
			slot := playerSlots[sid]
			ic.ObserveButtons(rw.RoundNumber, tick, slot, ButtonsFromPlayer(pl))
		}

		// Sample 32Hz replay frame & active projectiles/infernos.
		stride := int64(math.Max(1, math.Round(tickRate/32.0)))
		if lastSampleTick >= 0 && tick-lastSampleTick < stride {
			return
		}
		lastSampleTick = tick

		if visGeoLoaded && visTracer != nil {
			sample := sampleObservation(gs)
			sample.Tick = tick
			visSamples = append(visSamples, sample)
			if cutoff := tick - int64(math.Round(5*tickRate)); cutoff > 0 {
				for len(visSamples) > 0 && visSamples[0].Tick < cutoff {
					visSamples = visSamples[1:]
				}
			}
		}

		poses := make([]ReplayPlayerPose, 0, 10)
		for _, pl := range gs.Participants().Playing() {
			sid := steamIDStr(pl)
			if sid == "" {
				continue
			}
			var teamCode uint8
			switch pl.Team {
			case common.TeamTerrorists:
				teamCode = 1
			case common.TeamCounterTerrorists:
				teamCode = 2
			}
			pos := pl.Position()
			wep := ""
			zoom := uint8(0)
			if aw := pl.ActiveWeapon(); aw != nil {
				wep = strings.ToLower(aw.String())
				if z := aw.ZoomLevel(); z > 0 {
					zoom = uint8(z)
				}
			}
			hasBomb := false
			for _, w := range pl.Weapons() {
				if w != nil && w.Type == common.EqBomb {
					hasBomb = true
					break
				}
			}
			poses = append(poses, ReplayPlayerPose{
				Slot:         playerSlots[sid],
				Team:         teamCode,
				Alive:        pl.IsAlive(),
				HasBomb:      hasBomb,
				IsDefusing:   pl.IsDefusing,
				Health:       uint8(clampInt(pl.Health(), 0, 100)),
				Armor:        uint8(clampInt(pl.Armor(), 0, 100)),
				Money:        uint16(clampInt(pl.Money(), 0, 65535)),
				EquipValue:   uint16(clampInt(pl.EquipmentValueCurrent(), 0, 65535)),
				FlashDur:     pl.GetFlashDuration(),
				X:            float32(pos.X),
				Y:            float32(pos.Y),
				Z:            float32(pos.Z),
				Yaw:          pl.ViewDirectionX(),
				Pitch:        pl.ViewDirectionY(),
				Zoom:         zoom,
				ActiveWeapon: wep,
			})
		}
		framesByRound[rw.RoundNumber] = append(framesByRound[rw.RoundNumber], ReplayFrame{
			Tick:    tick,
			Players: poses,
		})

		for _, proj := range gs.GrenadeProjectiles() {
			if proj == nil {
				continue
			}
			pos := proj.Position()
			gc.SampleProjectile(proj.UniqueID(), tick, Point3D{float32(pos.X), float32(pos.Y), float32(pos.Z)})
		}
		for _, inf := range gs.Infernos() {
			if inf == nil {
				continue
			}
			pts := inf.Fires().Active().ConvexHull2D()
			if len(pts) > 0 {
				hull := make([]Point2D, len(pts))
				for i, pt := range pts {
					hull[i] = Point2D{float32(pt.X), float32(pt.Y)}
				}
				gc.UpdateInfernoHull(inf.UniqueID(), tick, hull)
			}
		}
	})

	if err := p.ParseToEnd(); err != nil {
		return singlePassOutput{}, fmt.Errorf("parse demo: %w", err)
	}
	if mapName == "" && len(rt.Rounds()) == 0 && rt.Active() == nil {
		return singlePassOutput{}, errors.New("corrupt or empty demo stream: no header or rounds found")
	}

	if tr := p.TickRate(); tr > 0 {
		tickRate = tr
		rt.tickRate = tr
	}

	endTick := int64(p.GameState().IngameTick())
	rt.FinalizeEOF(endTick)
	if opts.OnProgress != nil {
		opts.OnProgress(ParseProgress{
			DemoID:  demo.ID,
			Percent: 90.0,
			Tick:    endTick,
			Stage:   "计算高光与战力指标",
		})
	}

	metaSlice := make([]PlayerMeta, 0, len(playerMeta))
	for _, m := range playerMeta {
		metaSlice = append(metaSlice, m)
	}

	rounds := rt.Rounds()
	metricsRes := ComputeMetrics(MetricsInput{
		TickRate:             tickRate,
		TotalRounds:          len(rounds),
		Players:              metaSlice,
		Rounds:               rounds,
		Kills:                kills,
		Damages:              damages,
		Flashes:              flashes,
		TTDSamples:           ttdSamples,
		BombPlantTickByRound: bombPlantTicks,
		EquipValues:          equipValues,
	})

	nowISO := time.Now().UTC().Format(time.RFC3339Nano)
	highlights := DetectCompetitiveHighlights(HighlightInput{
		ContentSHA256: demo.ContentSHA256,
		TickRate:      tickRate,
		HalftimeRound: rt.HalftimeRound(),
		PreBufferSec:  opts.PreBufferSec,
		PostBufferSec: opts.PostBufferSec,
		Rounds:        rounds,
		Kills:         kills,
		Clutches:      metricsRes.Clutches,
		Defuses:       defuses,
		CreatedAt:     nowISO,
	})

	storageRounds := make([]storage.MatchRound, 0, len(rounds))
	var purchases []storage.MatchPurchase
	replayRounds := make([]ReplayRoundData, 0, len(rounds))

	for _, r := range rounds {
		tagsJSON, _ := json.Marshal(r.Tags)
		peJSON, _ := json.Marshal(r.PlayerEconomy)
		storageRounds = append(storageRounds, storage.MatchRound{
			RoundNumber:       r.RoundNumber,
			StartTick:         r.StartTick,
			FreezeEndTick:     r.FreezeEndTick,
			EndTick:           r.EndTick,
			DurationSec:       r.DurationSec,
			WinnerTeamKey:     r.WinnerTeamKey,
			WinnerSide:        r.WinnerSide,
			WinReason:         r.WinReason,
			BombSite:          r.BombSite,
			TeamASide:         r.TeamASide,
			TeamBSide:         r.TeamBSide,
			TeamAScoreBefore:  r.TeamAScoreBefore,
			TeamBScoreBefore:  r.TeamBScoreBefore,
			TeamAScoreAfter:   r.TeamAScoreAfter,
			TeamBScoreAfter:   r.TeamBScoreAfter,
			TeamAEquipValue:   r.TeamAEquipValue,
			TeamBEquipValue:   r.TeamBEquipValue,
			TeamAEconomyType:  r.TeamAEconomyType,
			TeamBEconomyType:  r.TeamBEconomyType,
			Headline:          r.Headline,
			TagsJSON:          string(tagsJSON),
			PlayerEconomyJSON: string(peJSON),
		})

		if len(r.Purchases) > 0 {
			sorted := append([]PlayerPurchase(nil), r.Purchases...)
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].Tick < sorted[j].Tick })
			for i := range sorted {
				if r.FreezeEndTick > 0 {
					sorted[i].TimeSec = roundTo(float64(sorted[i].Tick-r.FreezeEndTick)/tickRate, 2)
				}
				purchases = append(purchases, storage.MatchPurchase{
					RoundNumber: r.RoundNumber,
					Tick:        sorted[i].Tick,
					TimeSec:     sorted[i].TimeSec,
					SteamID64:   sorted[i].SteamID64,
					PlayerName:  sorted[i].PlayerName,
					Weapon:      sorted[i].Weapon,
					Cost:        sorted[i].Cost,
				})
			}
		}

		gTracks, smokes, infernos := gc.RoundEffects(r.RoundNumber)
		replayRounds = append(replayRounds, ReplayRoundData{
			RoundNumber: uint32(r.RoundNumber),
			StartTick:   r.StartTick,
			EndTick:     r.EndTick,
			Frames:      framesByRound[r.RoundNumber],
			Grenades:    gTracks,
			Smokes:      smokes,
			Infernos:    infernos,
			InputEdges:  ic.RoundEdges(r.RoundNumber),
			Shots:       shotsByRound[r.RoundNumber],
			Deaths:      deathsByRound[r.RoundNumber],
			Bombs:       bombsByRound[r.RoundNumber],
		})
	}

	scoreA, scoreB := rt.Scores()
	durSec := p.CurrentTime().Seconds()
	if durSec <= 0 && tickRate > 0 && endTick > matchStartTick {
		durSec = float64(endTick-matchStartTick) / tickRate
	}

	return singlePassOutput{
		mapName:        mapName,
		tickRate:       tickRate,
		matchStartTick: matchStartTick,
		demoEndTick:    endTick,
		halftimeRound:  rt.HalftimeRound(),
		teamAName:      teamAName,
		teamBName:      teamBName,
		teamAScore:     scoreA,
		teamBScore:     scoreB,
		durationSec:    durSec,
		inputCap:       ic.Capability(),
		players:        metricsRes.Players,
		playerSides:    metricsRes.Sides,
		rounds:         storageRounds,
		events:         matchEvents,
		highlights:     highlights,
		replayRounds:   replayRounds,
		purchases:      purchases,
	}, nil
}

func steamIDStr(pl *common.Player) string {
	if pl == nil || pl.SteamID64 == 0 {
		return ""
	}
	return strconv.FormatUint(pl.SteamID64, 10)
}

func roundTimeSec(tick, freezeEnd int64, tickRate float64) float64 {
	if tickRate <= 0 || tick <= freezeEnd {
		return 0
	}
	return roundTo(float64(tick-freezeEnd)/tickRate, 2)
}

func strPtrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func extractPlayerEquipment(pl *common.Player) []string {
	if pl == nil {
		return nil
	}
	var primary, pistols, nades []string
	for _, eq := range pl.Weapons() {
		if eq == nil {
			continue
		}
		switch eq.Type {
		case common.EqKnife, common.EqKevlar, common.EqHelmet, common.EqDefuseKit, common.EqUnknown:
			continue
		case common.EqSmoke:
			nades = append(nades, "Smoke")
			continue
		case common.EqFlash:
			nades = append(nades, "Flash")
			continue
		case common.EqHE:
			nades = append(nades, "HE")
			continue
		case common.EqMolotov:
			nades = append(nades, "Molotov")
			continue
		case common.EqIncendiary:
			nades = append(nades, "Inc")
			continue
		case common.EqDecoy:
			nades = append(nades, "Decoy")
			continue
		case common.EqBomb:
			nades = append(nades, "C4")
			continue
		}
		name := eq.Type.String()
		if name == "" {
			continue
		}
		switch eq.Class() {
		case common.EqClassRifle, common.EqClassSMG, common.EqClassHeavy:
			primary = append(primary, name)
		case common.EqClassPistols:
			pistols = append(pistols, name)
		default:
			nades = append(nades, name)
		}
	}
	out := make([]string, 0, len(primary)+len(pistols)+len(nades))
	out = append(out, primary...)
	out = append(out, pistols...)
	out = append(out, nades...)
	return out
}
