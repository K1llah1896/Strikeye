package playback

import (
	"strings"
	"testing"
)

func TestURLEncodesSpacesAndPlus(t *testing.T) {
	got := URL(`C:\Users\ana lyst\demos\furia+falcons m1.dem`)
	want := "steam://rungameid/730//+playdemo%20C:%5CUsers%5Cana%20lyst%5Cdemos%5Cfuria%2Bfalcons%20m1.dem"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	if strings.Count(got, "+") != 1 {
		t.Fatalf("only the playdemo flag may contain '+': %q", got)
	}
}

func TestConfigSeeksOnlyWithTick(t *testing.T) {
	if Config(0) != "" {
		t.Fatalf("zero tick should not produce a config")
	}
	cfg := Config(12345)
	if !strings.Contains(cfg, "demo_goto 12345") {
		t.Fatalf("config = %q", cfg)
	}
}

func TestDemoFileNameUsesForwardSlashes(t *testing.T) {
	if got := DemoFileName(`/data/media/demos/m1.dem`); got != "m1.dem" {
		t.Fatalf("DemoFileName = %q", got)
	}
}
