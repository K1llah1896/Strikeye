package parser

const (
	EconomyPistol = "pistol"
	EconomyFull   = "full"
	EconomyForce  = "force"
	EconomySemi   = "semi"
	EconomyEco    = "eco"
)

var economyOrder = []string{
	EconomyPistol,
	EconomyFull,
	EconomyForce,
	EconomySemi,
	EconomyEco,
}

// PlayerRoundEconomy captures one player's freeze-time-end economy snapshot.
type PlayerRoundEconomy struct {
	SteamID64    string `json:"steam_id64"`
	PlayerName   string `json:"player_name"`
	TeamKey      string `json:"team_key"`
	EquipValue   int    `json:"equip_value"`
	AccountStart int    `json:"account_start"`
	CashSpent    int    `json:"cash_spent"`
	EconomyType  string `json:"economy_type"`
}

// ClassifyPlayerEconomy classifies a player's round economy into pistol, full,
// force, semi, or eco using the legacy CS2-insight-agent voting rules.
func ClassifyPlayerEconomy(roundNum int, halftimeRound *int, equipVal, accountStart, cashSpent int) string {
	if roundNum == 1 || (halftimeRound != nil && roundNum == *halftimeRound) {
		return EconomyPistol
	}
	if equipVal >= 4000 {
		return EconomyFull
	}
	if cashSpent < 1000 && equipVal < 1000 {
		return EconomyEco
	}
	if accountStart > 0 && float64(cashSpent)/float64(accountStart) >= 0.80 {
		return EconomyForce
	}
	return EconomySemi
}

// VoteTeamEconomy selects the team's dominant economy type, breaking ties
// deterministically in priority order: pistol > full > force > semi > eco.
func VoteTeamEconomy(values []string) string {
	counts := make(map[string]int, len(economyOrder))
	best := 0
	for _, v := range values {
		if !isKnownEconomy(v) {
			continue
		}
		counts[v]++
		if counts[v] > best {
			best = counts[v]
		}
	}
	if best == 0 {
		return EconomyFull
	}
	for _, k := range economyOrder {
		if counts[k] == best {
			return k
		}
	}
	return EconomyFull
}

func isKnownEconomy(v string) bool {
	for _, k := range economyOrder {
		if v == k {
			return true
		}
	}
	return false
}
