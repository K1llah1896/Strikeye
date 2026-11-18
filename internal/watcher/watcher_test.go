package watcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strikeye/internal/config"
	"strikeye/internal/storage"
)

func TestDiscoverWatchTargetsAndPlatformClassification(t *testing.T) {
	tmp := t.TempDir()
	customDir := filepath.Join(tmp, "custom_demos")
	cs2Root := filepath.Join(tmp, "SteamLibrary", "steamapps", "common", "Counter-Strike Global Offensive")
	cs2Exe := filepath.Join(cs2Root, "game", "bin", "win64", "cs2.exe")
	replaysDir := filepath.Join(cs2Root, "game", "csgo", "replays")
	pwDir := filepath.Join(tmp, "PerfectWorldArena", "demos")
	fiveEDir := filepath.Join(tmp, "5EPlay", "demos")

	for _, dir := range []string{customDir, filepath.Dir(cs2Exe), replaysDir, pwDir, fiveEDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(cs2Exe, []byte("stub"), 0o644); err != nil {
		t.Fatalf("write cs2.exe stub: %v", err)
	}

	cfg := config.DefaultConfig(tmp)
	cfg.DemoWatchDirs = []string{customDir, filepath.Join(tmp, "nonexistent")}
	cfg.Paths.CS2Exe = cs2Exe

	targets := DiscoverWatchTargets(DiscoverOptions{
		Config: cfg,
		EnvLookup: func(key string) string {
			switch key {
			case "STRIKEYE_PW_DEMO_DIR":
				return pwDir
			case "STRIKEYE_5E_DEMO_DIR":
				return fiveEDir
			default:
				return ""
			}
		},
	})

	got := make(map[string]WatchTarget)
	for _, tgt := range targets {
		got[tgt.Dir] = tgt
	}

	if tgt, ok := got[customDir]; !ok || tgt.Platform != PlatformUnknown || tgt.AutoProbed {
		t.Fatalf("unexpected custom target: %+v (ok=%v)", tgt, ok)
	}
	if tgt, ok := got[replaysDir]; !ok || tgt.Platform != PlatformValve || !tgt.AutoProbed {
		t.Fatalf("unexpected valve replays target: %+v (ok=%v)", tgt, ok)
	}
	if tgt, ok := got[pwDir]; !ok || tgt.Platform != PlatformPerfectWorld || !tgt.AutoProbed {
		t.Fatalf("unexpected pw target: %+v (ok=%v)", tgt, ok)
	}
	if tgt, ok := got[fiveEDir]; !ok || tgt.Platform != Platform5E || !tgt.AutoProbed {
		t.Fatalf("unexpected 5e target: %+v (ok=%v)", tgt, ok)
	}
}

func makeFakeDemoBytes(size int, validMagic bool) []byte {
	buf := make([]byte, size)
	if validMagic {
		copy(buf[:8], DemoHeaderMagic)
	} else {
		copy(buf[:8], "BADMAGIC")
	}
	for i := 8; i < len(buf); i++ {
		buf[i] = byte(i % 251)
	}
	return buf
}

func TestWatcherStabilizationValidationAndDeduplication(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	dataRoot := filepath.Join(tmp, "data")
	watchDir := filepath.Join(tmp, "watch", "5e")
	if err := os.MkdirAll(watchDir, 0o755); err != nil {
		t.Fatalf("mkdir watchDir: %v", err)
	}

	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var discovered []DiscoveredDemo
	var warnings []error

	w := New(db, Options{
		Targets:        []WatchTarget{{Dir: watchDir, Platform: Platform5E}},
		SampleInterval: 10 * time.Millisecond,
		StableDuration: 25 * time.Millisecond,
		MinSamples:     2,
		MinFileSize:    1024, // 1 KB threshold for fast unit tests
		OnDiscovered: func(d DiscoveredDemo) {
			discovered = append(discovered, d)
		},
		OnWarning: func(_ string, err error) {
			warnings = append(warnings, err)
		},
	})

	// 1. Non-.dem files are ignored.
	if err := os.WriteFile(filepath.Join(watchDir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Small / invalid header files trigger warnings without crashing.
	smallDemo := filepath.Join(watchDir, "too_small.dem")
	if err := os.WriteFile(smallDemo, makeFakeDemoBytes(512, true), 0o644); err != nil {
		t.Fatal(err)
	}
	badMagicDemo := filepath.Join(watchDir, "bad_magic.dem")
	if err := os.WriteFile(badMagicDemo, makeFakeDemoBytes(2048, false), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Valid demo still being written: first sample at 1500 bytes, then grows to 2048 bytes.
	liveDemo := filepath.Join(watchDir, "match_01.dem")
	if err := os.WriteFile(liveDemo, makeFakeDemoBytes(1500, true), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := w.PollOnce(ctx); err != nil {
		t.Fatalf("poll 1: %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("expected 0 discovered on first sample, got %d", len(discovered))
	}

	// Grow liveDemo before stability window completes.
	payload := makeFakeDemoBytes(2048, true)
	if err := os.WriteFile(liveDemo, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if err := w.PollOnce(ctx); err != nil {
		t.Fatalf("poll 2: %v", err)
	}
	// smallDemo & badMagicDemo should have stabilized and been rejected; liveDemo reset on size change.
	if len(discovered) != 0 {
		t.Fatalf("expected liveDemo not yet ingested after resize, got %d", len(discovered))
	}
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings (small + bad magic), got %d (%v)", len(warnings), warnings)
	}
	if !errors.Is(warnings[0], ErrDemoTooSmall) && !errors.Is(warnings[1], ErrDemoTooSmall) {
		t.Fatalf("expected ErrDemoTooSmall in warnings: %v", warnings)
	}
	if !errors.Is(warnings[0], ErrInvalidDemoMagic) && !errors.Is(warnings[1], ErrInvalidDemoMagic) {
		t.Fatalf("expected ErrInvalidDemoMagic in warnings: %v", warnings)
	}

	// Wait for liveDemo to stabilize.
	time.Sleep(35 * time.Millisecond)
	if err := w.PollOnce(ctx); err != nil {
		t.Fatalf("poll 3: %v", err)
	}
	if len(discovered) != 1 || !discovered[0].Created {
		t.Fatalf("expected 1 newly created demo, got %+v", discovered)
	}
	if discovered[0].Demo.SourcePlatform != Platform5E {
		t.Fatalf("expected platform 5e, got %s", discovered[0].Demo.SourcePlatform)
	}

	// Mark demo as parsed, then place an identical copy at a new path -> must deduplicate without re-parsing.
	parsedDemo := discovered[0].Demo
	parsedDemo.ParseStatus = "parsed"
	if err := db.Demos().UpdateParseResult(ctx, parsedDemo); err != nil {
		t.Fatalf("mark parsed: %v", err)
	}

	dupDemo := filepath.Join(watchDir, "match_01_copy.dem")
	if err := os.WriteFile(dupDemo, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.PollOnce(ctx); err != nil {
		t.Fatalf("poll 4: %v", err)
	}
	time.Sleep(35 * time.Millisecond)
	if err := w.PollOnce(ctx); err != nil {
		t.Fatalf("poll 5: %v", err)
	}
	if len(discovered) != 2 {
		t.Fatalf("expected 2nd discovery callback for duplicate, got %d", len(discovered))
	}
	if discovered[1].Created {
		t.Fatalf("expected duplicate demo Created=false")
	}
	if discovered[1].Demo.ID != discovered[0].Demo.ID {
		t.Fatalf("expected same demo ID %d, got %d", discovered[0].Demo.ID, discovered[1].Demo.ID)
	}
	if discovered[1].Demo.ParseStatus != "parsed" {
		t.Fatalf("expected parse_status 'parsed' preserved, got %s", discovered[1].Demo.ParseStatus)
	}
}
