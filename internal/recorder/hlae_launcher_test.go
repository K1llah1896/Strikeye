package recorder

import (
	"strings"
	"testing"
)

func TestClampSegmentEndTick(t *testing.T) {
	// 64 tick -> guard is ceil(1.5 * 64) = 96 ticks.
	got := ClampSegmentEndTick(9990, 10000, 64)
	if got != 9904 {
		t.Fatalf("expected 9904, got %d", got)
	}
	// Within safe boundary -> unchanged.
	got2 := ClampSegmentEndTick(5000, 10000, 64)
	if got2 != 5000 {
		t.Fatalf("expected 5000, got %d", got2)
	}
}

func TestBuildBootCFGAndHLAEArgs(t *testing.T) {
	cfg, err := BuildBootCFG(BootCFGOptions{
		SessionID:     "sess-42",
		TempRenderDir: "/tmp/render_out",
		DemoPlayArg:   "_strikeye_playback_sess-42.dem",
		DemoEndTick:   5000,
		TickRate:      64,
		Segments: []RenderSegment{
			{TaskID: 2, SegIndex: 0, StartTick: 2000, EndTick: 4990, TargetSpecSlot: 3},
			{TaskID: 1, SegIndex: 0, StartTick: 500, EndTick: 900, TargetSpecSlot: 2},
		},
	})
	if err != nil {
		t.Fatalf("BuildBootCFG: %v", err)
	}

	// Segment 1 (tick 500) must appear before Segment 2 (tick 2000).
	idx1 := strings.Index(cfg, "STRIKEYE_EVT:START:1:0")
	idx2 := strings.Index(cfg, "STRIKEYE_EVT:START:2:0")
	if idx1 < 0 || idx2 < 0 || idx1 >= idx2 {
		t.Fatalf("expected ascending start_tick order in boot cfg:\n%s", cfg)
	}
	// Pre-seek warmup for 500 - 128 = 372.
	if !strings.Contains(cfg, "mirv_cmd addAtTick 372 \"spec_mode 5; spec_player 2; demo_timescale 1\"") {
		t.Fatalf("missing pre-seek warmup command:\n%s", cfg)
	}
	// End tick 4990 clamped to 5000 - 96 = 4904.
	if !strings.Contains(cfg, "mirv_cmd addAtTick 4904 \"mirv_streams record end; demo_pause; echo STRIKEYE_EVT:END:2:0\"") {
		t.Fatalf("expected clamped end tick 4904 in cfg:\n%s", cfg)
	}

	args, err := BuildHLAEArgs(LaunchOptions{
		SessionID:         "sess-42",
		HLAEExe:           `C:\HLAE\HLAE.exe`,
		AfxHookSource2DLL: `C:\HLAE\x64\AfxHookSource2.dll`,
		CS2Exe:            `C:\CS2\game\bin\win64\cs2.exe`,
		Width:             1920,
		Height:            1080,
	})
	if err != nil {
		t.Fatalf("BuildHLAEArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, req := range []string{"-customLoader", "-noGui", "-autoStart", "-insecure", "-con_logfile _strikeye_session_sess-42.log", "+exec _strikeye_boot_sess-42.cfg"} {
		if !strings.Contains(joined, req) {
			t.Fatalf("missing required arg %q in %q", req, joined)
		}
	}
	for _, forbidden := range []string{"-secure", "+connect"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("forbidden arg %q found in %q", forbidden, joined)
		}
	}
}
