package parser

import "testing"

func TestNearestShotWeapon(t *testing.T) {
	round := ReplayRoundData{Shots: []ReplayShotEvent{
		{Tick: 100, PlayerSlot: 2, Weapon: "weapon_awp"},
		{Tick: 160, PlayerSlot: 2, Weapon: "weapon_ak47"},
		{Tick: 160, PlayerSlot: 5, Weapon: "weapon_m4a1"},
		{Tick: 400, PlayerSlot: 2, Weapon: "weapon_glock"},
	}}
	cases := []struct {
		name string
		slot uint8
		tick int64
		want string
	}{
		{"closest wins", 2, 158, "ak47"},
		{"prefix stripped", 5, 160, "m4a1"},
		{"outside window", 2, 300, ""},
		{"unknown slot", 9, 160, ""},
	}
	for _, tc := range cases {
		if got := NearestShotWeapon(round, tc.slot, tc.tick); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
	if got := WeaponDisplayName("weapon_smokegrenade"); got != "smokegrenade" {
		t.Fatalf("display name = %q", got)
	}
}
