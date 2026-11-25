package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"strikeye/internal/storage"
)

// buysPlayer aggregates one player's purchases in a round.
type buysPlayer struct {
	SteamID64  string                  `json:"steam_id64"`
	PlayerName string                  `json:"player_name"`
	Spend      int                     `json:"spend"`
	Items      []storage.MatchPurchase `json:"items"`
}

// buysRound aggregates one round's purchases.
type buysRound struct {
	Round      int          `json:"round"`
	TotalSpend int          `json:"total_spend"`
	Players    []buysPlayer `json:"players"`
}

// runBuys prints item-level purchases grouped by round, then by player.
func runBuys(ctx context.Context, dataRoot string, demoID int64, round int, out io.Writer) error {
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	demo, err := pickDemo(ctx, db, demoID)
	if err != nil {
		return err
	}
	rows, err := db.Analysis().ListMatchPurchases(ctx, demo.ID)
	if err != nil {
		return err
	}

	byRound := map[int][]storage.MatchPurchase{}
	for _, p := range rows {
		if round > 0 && p.RoundNumber != round {
			continue
		}
		byRound[p.RoundNumber] = append(byRound[p.RoundNumber], p)
	}
	roundNums := make([]int, 0, len(byRound))
	for rn := range byRound {
		roundNums = append(roundNums, rn)
	}
	sort.Ints(roundNums)

	outRounds := make([]buysRound, 0, len(roundNums))
	total := 0
	for _, rn := range roundNums {
		items := byRound[rn]
		byPlayer := map[string]*buysPlayer{}
		order := make([]string, 0, len(items))
		for _, p := range items {
			bp, ok := byPlayer[p.SteamID64]
			if !ok {
				bp = &buysPlayer{SteamID64: p.SteamID64, PlayerName: p.PlayerName}
				byPlayer[p.SteamID64] = bp
				order = append(order, p.SteamID64)
			}
			bp.Spend += p.Cost
			bp.Items = append(bp.Items, p)
		}
		players := make([]buysPlayer, 0, len(order))
		roundTotal := 0
		for _, sid := range order {
			bp := byPlayer[sid]
			roundTotal += bp.Spend
			players = append(players, *bp)
		}
		sort.SliceStable(players, func(i, j int) bool { return players[i].Spend > players[j].Spend })
		total += roundTotal
		outRounds = append(outRounds, buysRound{Round: rn, TotalSpend: roundTotal, Players: players})
	}

	payload := struct {
		DemoID      int64       `json:"demo_id"`
		Purchases   int         `json:"purchases"`
		TotalSpend  int         `json:"total_spend"`
		AvgPerRound float64     `json:"avg_per_round"`
		Rounds      []buysRound `json:"rounds"`
	}{DemoID: demo.ID, Purchases: len(rows), TotalSpend: total, Rounds: outRounds}
	if round > 0 {
		payload.AvgPerRound = float64(total)
	} else if len(outRounds) > 0 {
		payload.AvgPerRound = float64(total) / float64(len(outRounds))
	}
	if payload.Rounds == nil {
		payload.Rounds = []buysRound{}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
