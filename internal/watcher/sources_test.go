package watcher

import "testing"

func TestClassifyPlatformCanonicalSources(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/home/u/.steam/steam/steamapps/common/Counter-Strike Global Offensive/game/csgo/replays/match.dem", PlatformValve},
		{"/home/u/PerfectWorldArena/demos/123.dem", PlatformPerfectWorld},
		{`D:\PerfectWorld\replays\m.dem`, PlatformPerfectWorld},
		{"/home/u/5EPlay/demos/m.dem", Platform5E},
		{"/home/u/Downloads/5e_match_01.dem", Platform5E},
		{"/home/u/Downloads/FACEIT/m1.dem", PlatformFACEIT},
		{"/home/u/Downloads/faceit_mirage_2026.dem", PlatformFACEIT},
		{"/home/u/demos/esl_pro_league.dem", PlatformESL},
		{"/home/u/demos/ESL/case.dem", PlatformESL},
		{"/home/u/demos/esea_season.dem", PlatformESEA},
		{"/home/u/demos/matchzy_pug.dem", PlatformMatchZy},
		{"/home/u/demos/pracc_scrim.dem", PlatformPracc},
		{"/home/u/demos/renown_cup.dem", PlatformRenown},
		{"/home/u/demos/challengermode_bo3.dem", PlatformChallengermode},
		{"/home/u/demos/esportal_faceit.dem", PlatformEsportal},
		{"/home/u/demos/fastcup_final.dem", PlatformFastCup},
		{"/home/u/demos/esplay_cup.dem", PlatformEsplay},
		{"/home/u/demos/esportligaen.dem", PlatformEsportligaen},
		{"/home/u/demos/popflash_retake.dem", PlatformPopflash},
		{"/home/u/demos/ebot_server.dem", PlatformEbot},
		{"/home/u/demos/random_match.dem", PlatformUnknown},
		{"/home/u/Movies/recording.mp4", PlatformUnknown},
	}
	for _, tc := range cases {
		if got := ClassifyPlatform(tc.path); got != tc.want {
			t.Errorf("ClassifyPlatform(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestNormalizePlatformLegacyLabels(t *testing.T) {
	cases := map[string]string{
		"perfectworld":  PlatformPerfectWorld,
		"Perfect World": PlatformPerfectWorld,
		"pw":            PlatformPerfectWorld,
		"5E":            Platform5E,
		"5eplay":        Platform5E,
		"FACEIT":        PlatformFACEIT,
		"":              PlatformUnknown,
		"community-x":   "community-x",
	}
	for in, want := range cases {
		if got := NormalizePlatform(in); got != want {
			t.Errorf("NormalizePlatform(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlatformRegistryCoversAllConstants(t *testing.T) {
	ids := make(map[string]bool)
	for _, id := range PlatformIDs() {
		ids[id] = true
	}
	for _, id := range []string{
		PlatformValve, PlatformPerfectWorld, Platform5E, PlatformFACEIT, PlatformESL,
		PlatformESEA, PlatformMatchZy, PlatformPracc, PlatformRenown,
		PlatformChallengermode, PlatformEsportal, PlatformFastCup, PlatformEsplay,
		PlatformEsportligaen, PlatformPopflash, PlatformEbot, PlatformCustom, PlatformUnknown,
	} {
		if !ids[id] {
			t.Errorf("platform %q missing from registry", id)
		}
	}
	if len(AllPlatforms()) != len(PlatformIDs()) {
		t.Fatalf("registry descriptors (%d) and ids (%d) diverged", len(AllPlatforms()), len(PlatformIDs()))
	}
	if PlatformLabel(PlatformPerfectWorld) == "" || PlatformLabel("mystery") != "mystery" {
		t.Fatalf("unexpected labels: %q / %q", PlatformLabel(PlatformPerfectWorld), PlatformLabel("mystery"))
	}
}
