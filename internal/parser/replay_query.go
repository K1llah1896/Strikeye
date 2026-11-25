package parser

import "strings"

// KillWeaponWindowTicks is how far from a death tick a shot may be attributed to
// that kill: one second at 64 tick. Both the 2D kill feed and the kill-context
// exporter use this window, so the two never disagree about the weapon.
const KillWeaponWindowTicks int64 = 64

// NearestShotWeapon returns the weapon of the shot closest to tick fired by slot,
// stripped of the `weapon_` prefix. It returns "" when no shot falls inside the
// window, which happens for knife kills and grenade deaths.
func NearestShotWeapon(round ReplayRoundData, slot uint8, tick int64) string {
	best := ""
	bestDelta := KillWeaponWindowTicks + 1
	for _, shot := range round.Shots {
		if shot.PlayerSlot != slot || shot.Weapon == "" {
			continue
		}
		delta := shot.Tick - tick
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			best, bestDelta = shot.Weapon, delta
		}
	}
	return strings.ReplaceAll(best, "weapon_", "")
}

// WeaponDisplayName normalises a raw weapon name for UI and export use.
func WeaponDisplayName(raw string) string { return strings.ReplaceAll(raw, "weapon_", "") }
