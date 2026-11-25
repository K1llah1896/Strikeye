package parser

import "testing"

func TestWeaponPriceTable(t *testing.T) {
	cases := map[string]int{
		"AK-47": 2700, "AWP": 4750, "Kevlar + Helmet": 1000,
		"Smoke Grenade": 300, "Defuse Kit": 400, "Zeus x27": 200,
	}
	for weapon, want := range cases {
		got, ok := WeaponPrice(weapon)
		if !ok || got != want {
			t.Fatalf("WeaponPrice(%q) = %d,%v want %d", weapon, got, ok, want)
		}
	}
	if _, ok := WeaponPrice("Unknown Gun"); ok {
		t.Fatal("unknown weapon must not have a price")
	}
	if _, ok := WeaponPrice("  AK-47  "); !ok {
		t.Fatal("price lookup should trim spaces")
	}
}

func TestDefaultLoadoutAndKnives(t *testing.T) {
	for _, w := range []string{"Knife", "C4", "Glock-18", "USP-S", "P2000"} {
		if !IsDefaultLoadout(w) {
			t.Fatalf("%q should be default loadout", w)
		}
	}
	if IsDefaultLoadout("AK-47") {
		t.Fatal("AK-47 is not a spawn item")
	}
	for _, w := range []string{"★ Karambit", "Bayonet", "Knife"} {
		if !KnifeVariant(w) {
			t.Fatalf("%q should be a knife", w)
		}
	}
	if KnifeVariant("M4A1-S") {
		t.Fatal("rifle is not a knife")
	}
}

func TestRoundTrackerPurchaseWindow(t *testing.T) {
	rt := NewRoundTracker(64)
	rt.StartRound(1000, "CT", "T")
	rt.RecordPurchase(1100, PlayerPurchase{SteamID64: "1", PlayerName: "a", Weapon: "AK-47", Cost: 2700})
	rt.RecordPurchase(5400, PlayerPurchase{SteamID64: "2", PlayerName: "b", Weapon: "Smoke Grenade", Cost: 300})
	rt.MarkFreezeEnd(6080)
	rt.RecordPurchase(7000, PlayerPurchase{SteamID64: "3", PlayerName: "c", Weapon: "AWP", Cost: 4750})

	rw := rt.Active()
	if rw == nil {
		t.Fatal("expected active round")
	}
	if len(rw.Purchases) != 2 {
		t.Fatalf("purchases = %+v", rw.Purchases)
	}
	for _, p := range rw.Purchases {
		if p.Tick > 6080 {
			t.Fatalf("post-freeze purchase kept: %+v", p)
		}
	}

	// Purchases without an active round are dropped.
	rt2 := NewRoundTracker(64)
	rt2.RecordPurchase(10, PlayerPurchase{Weapon: "P250"})
	if rt2.Active() != nil {
		t.Fatal("no active round expected")
	}
}

func TestRoundTrackerIgnoresPreRoundPurchases(t *testing.T) {
	rt := NewRoundTracker(64)
	rt.StartRound(5000, "CT", "T")
	rt.RecordPurchase(1200, PlayerPurchase{SteamID64: "1", Weapon: "AK-47", Cost: 2700})
	rt.RecordPurchase(5100, PlayerPurchase{SteamID64: "1", Weapon: "Flashbang", Cost: 200})
	if got := rt.Active().Purchases; len(got) != 1 || got[0].Weapon != "Flashbang" {
		t.Fatalf("purchases = %+v", got)
	}
}
