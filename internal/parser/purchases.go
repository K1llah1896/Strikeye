package parser

import "strings"

// PlayerPurchase is one item a player bought during a round's buy phase.
type PlayerPurchase struct {
	SteamID64  string  `json:"steam_id64"`
	PlayerName string  `json:"player_name"`
	Weapon     string  `json:"weapon"`
	Cost       int     `json:"cost"`
	Tick       int64   `json:"tick"`
	TimeSec    float64 `json:"time_sec"` // negative before freeze time ends
}

// weaponPrices holds CS2 buy-menu prices in dollars. Names match the labels
// demoinfocs prints for an Equipment ("AK-47", "Kevlar + Helmet", ...).
var weaponPrices = map[string]int{
	// Pistols
	"Glock-18": 200, "USP-S": 200, "P2000": 200, "Dual Berettas": 300,
	"P250": 300, "Five-SeveN": 500, "Tec-9": 500, "CZ75-Auto": 500,
	"Desert Eagle": 700, "R8 Revolver": 600,
	// SMGs
	"MP9": 1250, "MAC-10": 1050, "MP7": 1500, "MP5-SD": 1500,
	"UMP-45": 1200, "P90": 2350, "PP-Bizon": 1400,
	// Rifles
	"Galil AR": 1800, "FAMAS": 2050, "AK-47": 2700, "M4A4": 3100,
	"M4A1-S": 2900, "SG 553": 3000, "AUG": 3300, "SSG 08": 1700,
	"AWP": 4750, "G3SG1": 5000, "SCAR-20": 5000,
	// Heavy
	"Nova": 1050, "XM1014": 2000, "Sawed-Off": 1100, "MAG-7": 1300,
	"M249": 5200, "Negev": 1700,
	// Gear
	"Kevlar Vest": 650, "Kevlar + Helmet": 1000, "Zeus x27": 200,
	"Defuse Kit": 400,
	// Utility
	"Flashbang": 200, "Smoke Grenade": 300, "HE Grenade": 300,
	"Incendiary Grenade": 600, "Molotov": 400, "Decoy Grenade": 50,
}

// defaultLoadout lists what every player spawns with for free; picking those up
// is not a purchase.
var defaultLoadout = map[string]bool{
	"Knife": true, "C4": true, "Glock-18": true, "USP-S": true, "P2000": true,
}

// WeaponPrice returns the buy-menu price for a weapon or gear label.
func WeaponPrice(weapon string) (int, bool) {
	price, ok := weaponPrices[strings.TrimSpace(weapon)]
	return price, ok
}

// IsDefaultLoadout reports whether the item is part of the free spawn loadout.
func IsDefaultLoadout(weapon string) bool {
	return defaultLoadout[strings.TrimSpace(weapon)]
}

// KnifeVariant reports whether a label is a knife skin name; those never count
// as purchases even though they are not in defaultLoadout by exact string.
func KnifeVariant(weapon string) bool {
	w := strings.ToLower(strings.TrimSpace(weapon))
	return strings.Contains(w, "knife") || strings.Contains(w, "bayonet") ||
		strings.Contains(w, "karambit") || strings.HasPrefix(w, "★")
}
