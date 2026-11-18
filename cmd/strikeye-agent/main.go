package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"strikeye/internal/composer"
	"strikeye/internal/config"
	"strikeye/internal/ipc"
	"strikeye/internal/parser"
	"strikeye/internal/recorder"
	"strikeye/internal/storage"
	"strikeye/internal/updater"
	"strikeye/internal/version"
	"strikeye/internal/watcher"
)

func main() {
	showVersion := flag.Bool("version", false, "Print version information and exit")
	envCheck := flag.Bool("env-check", false, "Print runtime environment summary and exit")
	dataRootFlag := flag.String("data-root", "", "Override Strikeye DataRoot directory")
	pipeFlag := flag.String("pipe", "", "Override IPC named pipe / socket address")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Summary("strikeye-agent"))
		return
	}
	if *envCheck {
		fmt.Printf("%s: ready (phase4)\n", version.Summary("strikeye-agent"))
		return
	}
	if exe, err := os.Executable(); err == nil {
		updater.CleanupOldBinaries(filepath.Dir(exe))
	}

	dataRoot := *dataRootFlag
	if dataRoot == "" {
		var err error
		dataRoot, err = config.DefaultDataRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve data-root: %v\n", err)
			os.Exit(1)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runAgent(ctx, dataRoot, *pipeFlag); err != nil {
		fmt.Fprintf(os.Stderr, "strikeye-agent error: %v\n", err)
		os.Exit(1)
	}
}

type agentService struct {
	dataRoot  string
	cfg       config.Config
	probe     config.ProbeReport
	db        *storage.DB
	rpc       *ipc.Server
	parseQ    chan int64
	scheduler *recorder.Scheduler
	watcher   *watcher.Watcher
}

func runAgent(ctx context.Context, dataRoot, pipeOverride string) error {
	cfg, _, err := config.Load(dataRoot)
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("storage open: %w", err)
	}
	defer db.Close()

	lock := ipc.NewLockInDir(filepath.Join(dataRoot, "cache"), ipc.MutexAgentSingleton)
	acquired, err := lock.TryAcquire()
	if err != nil {
		return fmt.Errorf("acquire agent singleton lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("strikeye-agent is already running")
	}
	defer func() { _ = lock.Release() }()

	pipeAddr := pipeOverride
	if pipeAddr == "" {
		if os.PathSeparator == '\\' {
			pipeAddr = ipc.DefaultPipePath
		} else {
			pipeAddr = filepath.Join(dataRoot, "strikeye-ipc.sock")
		}
	}
	ln, err := ipc.Listen(pipeAddr)
	if err != nil {
		return fmt.Errorf("listen ipc %s: %w", pipeAddr, err)
	}
	defer func() {
		_ = ln.Close()
		if os.PathSeparator != '\\' {
			_ = os.Remove(pipeAddr)
		}
	}()

	svc := newAgentService(dataRoot, cfg, db)
	if _, _, _, recErr := svc.scheduler.RecoverInterrupted(ctx); recErr != nil {
		fmt.Fprintf(os.Stderr, "startup recovery warning: %v\n", recErr)
	}
	go svc.runParseWorker(ctx)
	go func() {
		_ = svc.scheduler.Run(ctx)
	}()

	targets := watcher.DiscoverWatchTargets(watcher.DiscoverOptions{
		Config: cfg,
		Probe:  svc.probe,
	})
	w := watcher.New(db, watcher.Options{
		Targets: targets,
		OnDiscovered: func(d watcher.DiscoveredDemo) {
			svc.handleDiscovered(d)
		},
	})
	svc.watcher = w
	go func() {
		_ = w.Run(ctx)
	}()

	return svc.rpc.Serve(ctx, ln)
}

func newAgentService(dataRoot string, cfg config.Config, db *storage.DB) *agentService {
	probe := config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths})
	rpcSrv := ipc.NewServer()
	lockDir := filepath.Join(dataRoot, "cache")
	svc := &agentService{
		dataRoot: dataRoot,
		cfg:      cfg,
		probe:    probe,
		db:       db,
		rpc:      rpcSrv,
		parseQ:   make(chan int64, 128),
		scheduler: recorder.NewScheduler(recorder.SchedulerOptions{
			DataRoot:   dataRoot,
			Config:     cfg,
			Probe:      probe,
			DB:         db,
			UILock:     ipc.NewLockInDir(lockDir, ipc.MutexUISingleton),
			RenderLock: ipc.NewLockInDir(lockDir, ipc.MutexCS2RenderActive),
			Publish: func(topic string, payload map[string]any) {
				_ = rpcSrv.PublishEvent(topic, payload)
			},
		}),
	}
	svc.registerRPCHandlers()
	return svc
}

func (s *agentService) registerRPCHandlers() {
	s.rpc.Register("agent.ping", func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]any{
			"status":  "ok",
			"version": version.AppVersion,
			"parser":  version.ParserBaseline,
		}, nil
	})
	s.rpc.Register("demo.list", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return s.db.Demos().List(ctx)
	})
	s.rpc.Register("demo.parse", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req struct {
			DemoID   int64  `json:"demo_id"`
			FilePath string `json:"file_path"`
			Force    bool   `json:"force"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &req); err != nil {
				return nil, fmt.Errorf("invalid params: %w", err)
			}
		}
		return parser.ParseAndPersist(ctx, s.db, parser.ParseOptions{
			DataRoot:      s.dataRoot,
			DemoID:        req.DemoID,
			FilePath:      req.FilePath,
			PreBufferSec:  s.cfg.Render.PreBufferSec,
			PostBufferSec: s.cfg.Render.PostBufferSec,
			Force:         req.Force,
		})
	})
	s.rpc.Register("render.status", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return s.scheduler.Status(ctx), nil
	})
	s.rpc.Register("render.start_now", func(_ context.Context, _ json.RawMessage) (any, error) {
		s.scheduler.TriggerNow()
		return map[string]any{"status": "triggered"}, nil
	})
	s.rpc.Register("render.abort_and_cleanup", func(ctx context.Context, _ json.RawMessage) (any, error) {
		if err := s.scheduler.AbortAndCleanup(ctx); err != nil {
			return nil, err
		}
		return map[string]any{"status": "aborted"}, nil
	})
	s.rpc.Register("render.enqueue", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req struct {
			DemoID           int64  `json:"demo_id"`
			ClipID           string `json:"clip_id,omitempty"`
			RoundNumber      *int   `json:"round_number,omitempty"`
			TargetSteamID64  string `json:"target_steam_id64"`
			TargetPlayerName string `json:"target_player_name"`
			TargetSpecSlot   int    `json:"target_spec_slot"`
			Perspective      string `json:"perspective"`
			TaskScope        string `json:"task_scope"`
			StartTick        int64  `json:"start_tick"`
			EndTick          int64  `json:"end_tick"`
			Priority         int    `json:"priority"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		demo, err := s.db.Demos().GetByID(ctx, req.DemoID)
		if err != nil {
			return nil, err
		}
		segsJSON, _ := json.Marshal([]recorder.SegmentWindow{{StartTick: req.StartTick, EndTick: req.EndTick}})
		optsJSON, _ := json.Marshal(s.cfg.Render)
		scope := req.TaskScope
		if scope == "" {
			scope = "highlight_clip"
		}
		persp := req.Perspective
		if persp == "" {
			persp = "pov"
		}
		prio := req.Priority
		if prio <= 0 {
			prio = 100
		}
		var clipPtr *string
		if req.ClipID != "" {
			clipPtr = &req.ClipID
		}
		uid := storage.ComputeTaskUID(demo.ContentSHA256, scope, req.TargetSteamID64, persp, string(segsJSON), string(optsJSON))
		task, created, err := s.db.RenderTasks().EnqueueIdempotent(ctx, storage.RenderTask{
			TaskUID:           uid,
			DemoID:            demo.ID,
			ClipID:            clipPtr,
			RoundNumber:       req.RoundNumber,
			TargetSteamID64:   req.TargetSteamID64,
			TargetPlayerName:  req.TargetPlayerName,
			TargetSpecSlot:    req.TargetSpecSlot,
			Perspective:       persp,
			TaskScope:         scope,
			SegmentsJSON:      string(segsJSON),
			RenderOptionsJSON: string(optsJSON),
			Priority:          prio,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"task": task, "created": created}, nil
	})
	s.rpc.Register("render.cancel", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req struct {
			TaskID int64 `json:"task_id"`
		}
		if err := json.Unmarshal(raw, &req); err != nil || req.TaskID <= 0 {
			return nil, fmt.Errorf("invalid task_id")
		}
		if err := s.db.RenderTasks().Cancel(ctx, req.TaskID); err != nil {
			return nil, err
		}
		_ = s.rpc.PublishEvent("render.cancelled", map[string]any{"task_id": req.TaskID})
		return map[string]any{"status": "cancelled", "task_id": req.TaskID}, nil
	})
	s.rpc.Register("render.retry", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req struct {
			TaskID int64 `json:"task_id"`
		}
		if err := json.Unmarshal(raw, &req); err != nil || req.TaskID <= 0 {
			return nil, fmt.Errorf("invalid task_id")
		}
		if err := s.db.RenderTasks().Retry(ctx, req.TaskID); err != nil {
			return nil, err
		}
		_ = s.rpc.PublishEvent("render.retried", map[string]any{"task_id": req.TaskID})
		return map[string]any{"status": "queued", "task_id": req.TaskID}, nil
	})
	s.rpc.Register("render.priority", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req struct {
			TaskID   int64 `json:"task_id"`
			Priority int   `json:"priority"`
		}
		if err := json.Unmarshal(raw, &req); err != nil || req.TaskID <= 0 {
			return nil, fmt.Errorf("invalid params")
		}
		if err := s.db.RenderTasks().UpdatePriority(ctx, req.TaskID, req.Priority); err != nil {
			return nil, err
		}
		return map[string]any{"status": "updated", "task_id": req.TaskID, "priority": req.Priority}, nil
	})
	s.rpc.Register("config.reload", func(ctx context.Context, _ json.RawMessage) (any, error) {
		cfg, _, err := config.Load(s.dataRoot)
		if err != nil {
			return nil, err
		}
		s.cfg = cfg
		s.probe = config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths})
		if s.watcher != nil {
			targets := watcher.DiscoverWatchTargets(watcher.DiscoverOptions{
				Config: cfg,
				Probe:  s.probe,
			})
			s.watcher.SetTargets(targets)
			_ = s.watcher.ScanNow(ctx)
		}
		_ = s.rpc.PublishEvent("config.reloaded", map[string]any{
			"ffmpeg_ok": s.probe.FFmpeg.Available,
			"hlae_ok":   s.probe.HLAE.Available,
			"cs2_ok":    s.probe.CS2.Available,
		})
		return map[string]any{"status": "reloaded", "probe": s.probe}, nil
	})
	s.rpc.Register("demo.scan", func(ctx context.Context, _ json.RawMessage) (any, error) {
		if s.watcher != nil {
			cfg, _, err := config.Load(s.dataRoot)
			if err == nil {
				s.cfg = cfg
				targets := watcher.DiscoverWatchTargets(watcher.DiscoverOptions{
					Config: cfg,
					Probe:  s.probe,
				})
				s.watcher.SetTargets(targets)
			}
			if err := s.watcher.ScanNow(ctx); err != nil {
				return nil, err
			}
		}
		return map[string]any{"status": "scanned"}, nil
	})
	s.rpc.Register("montage.export", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req struct {
			ProjectID *int64               `json:"project_id,omitempty"`
			Spec      composer.MontageSpec `json:"spec"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("invalid montage params: %w", err)
		}
		if req.Spec.OutputPath == "" {
			req.Spec.OutputPath = filepath.Join(s.dataRoot, "media", "montages", fmt.Sprintf("montage_%d.mp4", time.Now().Unix()))
		}
		req.Spec.ProjectID = req.ProjectID
		go s.runMontageExport(req.Spec)
		return map[string]any{
			"status":      "started",
			"output_path": req.Spec.OutputPath,
		}, nil
	})
}

func (s *agentService) runMontageExport(spec composer.MontageSpec) {
	ctx := context.Background()
	_ = s.rpc.PublishEvent("montage.progress", map[string]any{
		"output_path": spec.OutputPath,
		"status":      "running",
		"progress":    0.1,
	})
	res, err := composer.BuildMontage(ctx, spec, composer.MontageDeps{
		FFmpegExe:  s.probe.FFmpeg.Path,
		FFprobeExe: s.probe.FFprobe.Path,
		Encoder:    s.cfg.Render.Encoder,
		DB:         s.db,
	})
	if err != nil {
		_ = s.rpc.PublishEvent("montage.failed", map[string]any{
			"output_path": spec.OutputPath,
			"error":       err.Error(),
		})
		return
	}
	_ = s.rpc.PublishEvent("montage.completed", map[string]any{
		"export_id":   res.ExportID,
		"output_path": spec.OutputPath,
		"media_id":    res.MediaVideoID,
	})
}

func (s *agentService) handleDiscovered(d watcher.DiscoveredDemo) {
	_ = s.rpc.PublishEvent("demo.discovered", map[string]any{
		"demo_id":  d.Demo.ID,
		"filename": d.Demo.Filename,
		"created":  d.Created,
		"status":   d.Demo.ParseStatus,
	})
	if d.Demo.ParseStatus == "pending" {
		select {
		case s.parseQ <- d.Demo.ID:
		default:
		}
	}
}

func (s *agentService) runParseWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case demoID := <-s.parseQ:
			_ = s.rpc.PublishEvent("parse.started", map[string]any{
				"demo_id": demoID,
				"ts":      time.Now().UTC().Format(time.RFC3339),
			})
			var lastSent time.Time
			sum, err := parser.ParseAndPersist(ctx, s.db, parser.ParseOptions{
				DataRoot:      s.dataRoot,
				DemoID:        demoID,
				PreBufferSec:  s.cfg.Render.PreBufferSec,
				PostBufferSec: s.cfg.Render.PostBufferSec,
				OnProgress: func(prog parser.ParseProgress) {
					now := time.Now()
					if !lastSent.IsZero() && now.Sub(lastSent) < 120*time.Millisecond && prog.Percent < 88 {
						return
					}
					lastSent = now
					_ = s.rpc.PublishEvent("parse.progress", map[string]any{
						"demo_id":  prog.DemoID,
						"stage":    prog.Stage,
						"progress": prog.Percent / 100.0,
						"percent":  prog.Percent,
						"tick":     prog.Tick,
					})
				},
			})
			if err != nil {
				_ = s.rpc.PublishEvent("parse.failed", map[string]any{
					"demo_id": demoID,
					"error":   err.Error(),
				})
				continue
			}
			_ = s.rpc.PublishEvent("parse.completed", map[string]any{
				"demo_id":               demoID,
				"map_name":              sum.Demo.MapName,
				"round_count":           sum.RoundCount,
				"highlight_count":       sum.HighlightCount,
				"input_data_capability": sum.InputDataCapability,
				"reused_cache":          sum.ReusedCache,
			})
			if planned, planErr := s.scheduler.PlanTasksForDemo(ctx, demoID); planErr == nil && len(planned) > 0 {
				_ = s.rpc.PublishEvent("render.planned", map[string]any{
					"demo_id":    demoID,
					"task_count": len(planned),
				})
			}
		}
	}
}
