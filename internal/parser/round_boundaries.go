package parser

import (
	"math"
)

// RoundWindow captures one official competitive round's tick boundaries,
// team sides, scores, and economy snapshot.
type RoundWindow struct {
	RoundNumber      int
	StartTick        int64
	FreezeEndTick    int64
	EndTick          int64
	DurationSec      int
	WinnerTeamKey    *string
	WinnerSide       *string
	WinReason        *string
	BombSite         *string
	TeamASide        string
	TeamBSide        string
	TeamAScoreBefore int
	TeamBScoreBefore int
	TeamAScoreAfter  int
	TeamBScoreAfter  int
	TeamAEquipValue  int
	TeamBEquipValue  int
	TeamAEconomyType string
	TeamBEconomyType string
	PlayerEconomy    []PlayerRoundEconomy
	Headline         *string
	Tags             []string
}

// RoundTracker accumulates official round windows across a single-pass demo scan.
type RoundTracker struct {
	tickRate      float64
	halftimeRound *int
	rounds        []*RoundWindow
	active        *RoundWindow

	teamAScore int
	teamBScore int
	lastSideA  string
}

// NewRoundTracker creates a RoundTracker for the given demo tick rate.
func NewRoundTracker(tickRate float64) *RoundTracker {
	if tickRate <= 0 {
		tickRate = 64.0
	}
	return &RoundTracker{tickRate: tickRate}
}

// ResetMatch clears any pre-match / warmup / knife rounds when live match starts.
func (rt *RoundTracker) ResetMatch() {
	rt.rounds = nil
	rt.active = nil
	rt.halftimeRound = nil
	rt.teamAScore = 0
	rt.teamBScore = 0
	rt.lastSideA = ""
}

// StartRound opens a new official round at startTick. If a previous round was still
// open without an explicit end tick, it is closed at startTick.
func (rt *RoundTracker) StartRound(startTick int64, teamASide, teamBSide string) *RoundWindow {
	if rt.active != nil {
		if rt.active.EndTick <= rt.active.StartTick {
			rt.CloseActive(startTick)
		} else {
			rt.finalizeActive()
		}
	}

	if teamASide == "" {
		teamASide = "CT"
	}
	if teamBSide == "" {
		if teamASide == "CT" {
			teamBSide = "T"
		} else {
			teamBSide = "CT"
		}
	}

	roundNum := len(rt.rounds) + 1
	if rt.lastSideA != "" && rt.lastSideA != teamASide && rt.halftimeRound == nil {
		hr := roundNum
		rt.halftimeRound = &hr
	}
	rt.lastSideA = teamASide

	w := &RoundWindow{
		RoundNumber:      roundNum,
		StartTick:        startTick,
		FreezeEndTick:    startTick,
		EndTick:          startTick,
		TeamASide:        teamASide,
		TeamBSide:        teamBSide,
		TeamAScoreBefore: rt.teamAScore,
		TeamBScoreBefore: rt.teamBScore,
		TeamAScoreAfter:  rt.teamAScore,
		TeamBScoreAfter:  rt.teamBScore,
		TeamAEconomyType: EconomyFull,
		TeamBEconomyType: EconomyFull,
	}
	rt.active = w
	return w
}

// MarkFreezeEnd records freeze_end_tick on the currently active round.
func (rt *RoundTracker) MarkFreezeEnd(tick int64) {
	if rt.active == nil {
		return
	}
	if tick >= rt.active.StartTick {
		rt.active.FreezeEndTick = tick
	}
}

// EndRound records the winner and win reason for the currently active round.
func (rt *RoundTracker) EndRound(endTick int64, winnerSide, winReason, bombSite string) {
	if rt.active == nil {
		return
	}
	if endTick > rt.active.EndTick {
		rt.active.EndTick = endTick
	}
	if winnerSide != "" {
		ws := winnerSide
		rt.active.WinnerSide = &ws
		var winnerKey string
		if winnerSide == rt.active.TeamASide {
			winnerKey = "team_a"
			rt.teamAScore++
		} else if winnerSide == rt.active.TeamBSide {
			winnerKey = "team_b"
			rt.teamBScore++
		}
		if winnerKey != "" {
			rt.active.WinnerTeamKey = &winnerKey
		}
	}
	if winReason != "" {
		wr := winReason
		rt.active.WinReason = &wr
	}
	if bombSite != "" {
		bs := bombSite
		rt.active.BombSite = &bs
	}
	rt.active.TeamAScoreAfter = rt.teamAScore
	rt.active.TeamBScoreAfter = rt.teamBScore
	rt.active.DurationSec = calcDurationSec(rt.active.StartTick, rt.active.EndTick, rt.tickRate)
}

// CloseActive closes the active round at endTick (used on next RoundStart or EOF).
func (rt *RoundTracker) CloseActive(endTick int64) {
	if rt.active == nil {
		return
	}
	if endTick > rt.active.EndTick {
		rt.active.EndTick = endTick
	}
	rt.active.DurationSec = calcDurationSec(rt.active.StartTick, rt.active.EndTick, rt.tickRate)
	rt.finalizeActive()
}

// FinalizeEOF closes any open round at the demo's final tick.
func (rt *RoundTracker) FinalizeEOF(eofTick int64) {
	if rt.active != nil {
		rt.CloseActive(eofTick)
	}
}

func (rt *RoundTracker) finalizeActive() {
	if rt.active == nil {
		return
	}
	rt.rounds = append(rt.rounds, rt.active)
	rt.active = nil
}

// Active returns the currently open round, or nil.
func (rt *RoundTracker) Active() *RoundWindow {
	return rt.active
}

// Rounds returns all finalized official rounds in order.
func (rt *RoundTracker) Rounds() []*RoundWindow {
	return rt.rounds
}

// HalftimeRound returns the 1-based round number where team sides first swapped, if any.
func (rt *RoundTracker) HalftimeRound() *int {
	return rt.halftimeRound
}

// Scores returns the final Team A and Team B scores.
func (rt *RoundTracker) Scores() (int, int) {
	return rt.teamAScore, rt.teamBScore
}

func calcDurationSec(startTick, endTick int64, tickRate float64) int {
	if endTick <= startTick || tickRate <= 0 {
		return 0
	}
	return int(math.Round(float64(endTick-startTick) / tickRate))
}
