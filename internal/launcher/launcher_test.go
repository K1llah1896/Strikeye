package launcher

import "testing"

func TestCommandPerPlatform(t *testing.T) {
	cases := map[string]string{
		"windows": "rundll32",
		"darwin":  "open",
		"linux":   "xdg-open",
	}
	for goos, want := range cases {
		argv, err := Command(goos, "steam://rungameid/730")
		if err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
		if argv[0] != want {
			t.Fatalf("%s: argv[0] = %q, want %q", goos, argv[0], want)
		}
		if argv[len(argv)-1] != "steam://rungameid/730" {
			t.Fatalf("%s: url lost: %v", goos, argv)
		}
	}
}

func TestCommandEmptyURL(t *testing.T) {
	if _, err := Command("linux", ""); err == nil {
		t.Fatal("expected an error for an empty url")
	}
}
