package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"strikeye/internal/config"
	"strikeye/internal/ipc"
	"strikeye/internal/parser"
	"strikeye/internal/recorder"
	"strikeye/internal/storage"
	"strikeye/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "Print version information and exit")
	envCheck := flag.Bool("env-check", false, "Print runtime environment summary and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Summary("strikeye-cli"))
		return
	}
	if *envCheck || (flag.NArg() > 0 && flag.Arg(0) == "env-check") {
		fmt.Printf("%s: ready (phase2)\n", version.Summary("strikeye-cli"))
		return
	}

	if flag.NArg() > 0 {
		switch flag.Arg(0) {
		case "doctor":
			fs := flag.NewFlagSet("doctor", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			_ = fs.Parse(flag.Args()[1:])

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := runDoctor(ctx, dataRoot, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "doctor failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "parse":
			fs := flag.NewFlagSet("parse", flag.ExitOnError)
			fileFlag := fs.String("file", "", "Path to CS2 .dem file")
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			forceFlag := fs.Bool("force", false, "Force re-parse even if already cached")
			_ = fs.Parse(flag.Args()[1:])

			if *fileFlag == "" {
				fmt.Fprintln(os.Stderr, "parse failed: --file is required")
				os.Exit(2)
			}
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := runParse(ctx, dataRoot, *fileFlag, *forceFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "parse failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "inspect":
			fs := flag.NewFlagSet("inspect", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Inspect persisted match by demo ID")
			rplFlag := fs.String("rpl", "", "Inspect CS2RPL01 binary replay file")
			roundFlag := fs.Uint("round", 0, "Optional round number when inspecting --rpl")
			_ = fs.Parse(flag.Args()[1:])

			if *demoIDFlag <= 0 && *rplFlag == "" {
				fmt.Fprintln(os.Stderr, "inspect failed: specify --demo-id or --rpl")
				os.Exit(2)
			}
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := runInspect(ctx, dataRoot, *demoIDFlag, *rplFlag, uint32(*roundFlag), os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "inspect failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "render":
			fs := flag.NewFlagSet("render", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Optional demo ID for dry-run script generation")
			dryRunFlag := fs.Bool("dry-run", true, "Print render self-check and generated HLAE/CFG dry-run output")
			_ = fs.Parse(flag.Args()[1:])

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := runRenderDryRun(ctx, dataRoot, *demoIDFlag, *dryRunFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "render failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	fmt.Fprintln(os.Stderr, "usage: strikeye-cli [--version | --env-check | doctor | parse --file <dem> | inspect (--demo-id <id> | --rpl <path>)]")
	os.Exit(2)
}

func resolveDataRootOrExit(override string) string {
	if override != "" {
		return override
	}
	dataRoot, err := config.DefaultDataRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve data-root failed: %v\n", err)
		os.Exit(1)
	}
	return dataRoot
}

type parseCLISummary struct {
	DemoID              int64   `json:"demo_id"`
	MapName             *string `json:"map_name,omitempty"`
	TeamAName           string  `json:"team_a_name"`
	TeamBName           string  `json:"team_b_name"`
	TeamAScore          int     `json:"team_a_score"`
	TeamBScore          int     `json:"team_b_score"`
	TickRate            float64 `json:"tick_rate"`
	HalftimeRound       *int    `json:"halftime_round,omitempty"`
	RoundCount          int     `json:"round_count"`
	PlayerCount         int     `json:"player_count"`
	EventCount          int     `json:"event_count"`
	HighlightCount      int     `json:"highlight_count"`
	InputDataCapability string  `json:"input_data_capability"`
	ReplayCachePath     string  `json:"replay_cache_path"`
	ReusedCache         bool    `json:"reused_cache"`
	ElapsedMs           int64   `json:"elapsed_ms"`
}

func runParse(ctx context.Context, dataRoot, filePath string, force bool, out io.Writer) error {
	start := time.Now()
	cfg, _, err := config.Load(dataRoot)
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("storage open: %w", err)
	}
	defer db.Close()

	sum, err := parser.ParseAndPersist(ctx, db, parser.ParseOptions{
		DataRoot:      dataRoot,
		FilePath:      filePath,
		PreBufferSec:  cfg.Render.PreBufferSec,
		PostBufferSec: cfg.Render.PostBufferSec,
		Force:         force,
	})
	if err != nil {
		return err
	}

	outObj := parseCLISummary{
		DemoID:              sum.Demo.ID,
		MapName:             sum.Demo.MapName,
		TeamAName:           sum.Demo.TeamAName,
		TeamBName:           sum.Demo.TeamBName,
		TeamAScore:          sum.Demo.TeamAScore,
		TeamBScore:          sum.Demo.TeamBScore,
		TickRate:            sum.Demo.TickRate,
		HalftimeRound:       sum.Demo.HalftimeRound,
		RoundCount:          sum.RoundCount,
		PlayerCount:         sum.PlayerCount,
		EventCount:          sum.EventCount,
		HighlightCount:      sum.HighlightCount,
		InputDataCapability: sum.InputDataCapability,
		ReplayCachePath:     sum.ReplayCachePath,
		ReusedCache:         sum.ReusedCache,
		ElapsedMs:           time.Since(start).Milliseconds(),
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(outObj)
}

func runInspect(ctx context.Context, dataRoot string, demoID int64, rplPath string, roundNum uint32, out io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")

	if rplPath != "" {
		hdr, entries, err := parser.ReadReplayIndex(rplPath)
		if err != nil {
			return err
		}
		if roundNum > 0 {
			_, rd, err := parser.ReadReplayRound(rplPath, roundNum)
			if err != nil {
				return err
			}
			return enc.Encode(map[string]any{
				"input_capability": parser.InputCapString(hdr.InputCap),
				"tick_rate":        hdr.TickRate,
				"round_number":     rd.RoundNumber,
				"start_tick":       rd.StartTick,
				"end_tick":         rd.EndTick,
				"frame_count":      len(rd.Frames),
				"grenade_count":    len(rd.Grenades),
				"smoke_count":      len(rd.Smokes),
				"inferno_count":    len(rd.Infernos),
				"input_edge_count": len(rd.InputEdges),
			})
		}
		return enc.Encode(map[string]any{
			"magic":            string(hdr.Magic[:]),
			"version":          hdr.Version,
			"input_capability": parser.InputCapString(hdr.InputCap),
			"tick_rate":        hdr.TickRate,
			"round_count":      hdr.RoundCount,
			"rounds":           entries,
		})
	}

	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("storage open: %w", err)
	}
	defer db.Close()

	demo, err := db.Demos().GetByID(ctx, demoID)
	if err != nil {
		return err
	}
	players, err := db.Analysis().ListMatchPlayers(ctx, demoID)
	if err != nil {
		return err
	}
	rounds, err := db.Analysis().ListMatchRounds(ctx, demoID)
	if err != nil {
		return err
	}
	clips, err := db.Analysis().ListHighlightClips(ctx, demoID)
	if err != nil {
		return err
	}
	return enc.Encode(map[string]any{
		"demo":       demo,
		"players":    players,
		"rounds":     rounds,
		"highlights": clips,
	})
}

type doctorSummary struct {
	Version       string             `json:"version"`
	DataRoot      string             `json:"data_root"`
	SchemaVersion int                `json:"schema_version"`
	Toolchain     config.ProbeReport `json:"toolchain"`
	IPCSelfCheck  string             `json:"ipc_self_check"`
}

func runDoctor(ctx context.Context, dataRoot string, out io.Writer) error {
	cfg, _, err := config.Load(dataRoot)
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	probe := config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths})

	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("storage open: %w", err)
	}
	defer db.Close()

	schemaVer, err := db.CurrentSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("schema version: %w", err)
	}

	if err := runIPCSelfCheck(ctx, dataRoot); err != nil {
		return fmt.Errorf("ipc self-check: %w", err)
	}

	summary := doctorSummary{
		Version:       version.Summary("strikeye-cli"),
		DataRoot:      filepath.Clean(dataRoot),
		SchemaVersion: schemaVer,
		Toolchain:     probe,
		IPCSelfCheck:  "ok",
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(summary)
}

func runIPCSelfCheck(ctx context.Context, dataRoot string) error {
	lock := ipc.NewLockInDir(filepath.Join(dataRoot, "cache"), "doctor-selfcheck-lock")
	acquired, err := lock.TryAcquire()
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("expected doctor lock to be acquired")
	}
	defer func() {
		_ = lock.Release()
	}()

	sockAddr := filepath.Join(dataRoot, "doctor-ipc.sock")
	if os.PathSeparator == '\\' {
		sockAddr = fmt.Sprintf(`\\.\pipe\strikeye-doctor-%d`, time.Now().UnixNano())
	}
	ln, err := ipc.Listen(sockAddr)
	if err != nil {
		return fmt.Errorf("listen ipc: %w", err)
	}
	defer func() {
		_ = ln.Close()
		if os.PathSeparator != '\\' {
			_ = os.Remove(sockAddr)
		}
	}()

	srv := ipc.NewServer()
	srv.Register("agent.ping", func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]string{"status": "pong"}, nil
	})

	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		_ = srv.Serve(srvCtx, ln)
	}()

	conn, err := ipc.Dial(ctx, sockAddr)
	if err != nil {
		return fmt.Errorf("dial ipc: %w", err)
	}
	cli := ipc.NewClient(conn)
	defer cli.Close()

	var pong map[string]string
	if err := cli.Call(ctx, "agent.ping", nil, &pong); err != nil {
		return fmt.Errorf("call agent.ping: %w", err)
	}
	if pong["status"] != "pong" {
		return fmt.Errorf("unexpected ping reply: %+v", pong)
	}
	return nil
}

func runRenderDryRun(ctx context.Context, dataRoot string, demoID int64, dryRun bool, out io.Writer) error {
	cfg, _, err := config.Load(dataRoot)
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	probe := config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths})
	cs2Root := ""
	if probe.CS2.Path != "" {
		cs2Root = filepath.Dir(filepath.Dir(filepath.Dir(probe.CS2.Path)))
	}
	cfgFiles, _ := recorder.DiscoverConfigFiles(probe.Steam.Path, cs2Root)

	sampleCFG, err := recorder.BuildBootCFG(recorder.BootCFGOptions{
		SessionID:     "dryrun",
		TempRenderDir: filepath.Join(dataRoot, "cache", "render_tmp", "dryrun"),
		DemoPlayArg:   "_strikeye_playback_dryrun.dem",
		DemoEndTick:   10000,
		TickRate:      64,
		Segments:      []recorder.RenderSegment{{TaskID: 1, SegIndex: 0, StartTick: 500, EndTick: 900, TargetSpecSlot: 1}},
	})
	if err != nil {
		return err
	}

	var queued []storage.RenderTask
	if demoID > 0 {
		db, err := storage.OpenDefault(dataRoot)
		if err != nil {
			return err
		}
		defer db.Close()
		all, err := db.RenderTasks().ListByStatus(ctx, "queued")
		if err != nil {
			return err
		}
		for _, t := range all {
			if t.DemoID == demoID {
				queued = append(queued, t)
			}
		}
	}

	var hlaeArgs []string
	if probe.HLAE.Path != "" && probe.AfxHookSource2.Path != "" && probe.CS2.Path != "" {
		hlaeArgs, _ = recorder.BuildHLAEArgs(recorder.LaunchOptions{
			SessionID:         "dryrun",
			HLAEExe:           probe.HLAE.Path,
			AfxHookSource2DLL: probe.AfxHookSource2.Path,
			CS2Exe:            probe.CS2.Path,
			Width:             cfg.Render.Width,
			Height:            cfg.Render.Height,
		})
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"dry_run":              dryRun,
		"policy":               cfg.Render.Policy,
		"probe":                probe,
		"discovered_cfg_files": cfgFiles,
		"queued_for_demo":      len(queued),
		"hlae_args":            hlaeArgs,
		"sample_boot_cfg":      sampleCFG,
	})
}
