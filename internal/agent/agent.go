// Package agent implements Strikeye's headless background service: demo directory
// watching, single-pass parsing queue, render task scheduling, and JSON-RPC IPC.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"strikeye/internal/composer"
	"strikeye/internal/config"
	"strikeye/internal/ipc"
	"strikeye/internal/parser"
	"strikeye/internal/recorder"
	"strikeye/internal/storage"
	"strikeye/internal/version"
	"strikeye/internal/watcher"
)

// Service encapsulates the background agent runtime and IPC handlers.
type Service struct {
	dataRoot   string
	cfg        config.Config
	probe      config.ProbeReport
	db         *storage.DB
	rpc        *ipc.Server
	parseQ     chan int64
	scheduler  *recorder.Scheduler
	watcher    *watcher.Watcher
	cancelFunc context.CancelFunc
}

// Run starts the headless agent service, acquires the agent singleton mutex,
// listens on the IPC pipe/socket, and blocks until ctx is canceled.
func Run(ctx context.Context, dataRoot, pipeOverride string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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

	svc := NewService(dataRoot, cfg, db)
	svc.cancelFunc = cancel
	if _, _, _, recErr := svc.scheduler.RecoverInterrupted(ctx); recErr != nil {
		fmt.Fprintf(os.Stderr, "startup recovery warning: %v\n", recErr)
	}
	go svc.RunParseWorker(ctx)
	go func() {
		_ = svc.scheduler.Run(ctx)
	}()

	targets := watcher.DiscoverWatchTargets(watcher.DiscoverOptions{
		Config: cfg,
		Probe:  svc.probe,
	})
	w := watcher.New(db, watcher.Options{
		Targets:   targets,
		ImportDir: filepath.Join(dataRoot, "media", "imports"),
		OnDiscovered: func(d watcher.DiscoveredDemo) {
			svc.HandleDiscovered(d)
		},
		OnWarning: func(path string, warnErr error) {
			fmt.Fprintf(os.Stderr, "watcher warning (%s): %v\n", path, warnErr)
		},
	})
	svc.watcher = w
	go func() {
		_ = w.Run(ctx)
	}()

	return svc.rpc.Serve(ctx, ln)
}

// NewService constructs an agent Service with all JSON-RPC handlers registered.
func NewService(dataRoot string, cfg config.Config, db *storage.DB) *Service {
	probe := config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths, DataRoot: dataRoot})
	rpcSrv := ipc.NewServer()
	lockDir := filepath.Join(dataRoot, "cache")
	svc := &Service{
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

// RPC returns the underlying JSON-RPC server.
func (s *Service) RPC() *ipc.Server {
	return s.rpc
}

func (s *Service) registerRPCHandlers() {
	s.rpc.Register("agent.ping", func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]any{
			"status":  "ok",
			"version": version.AppVersion,
			"parser":  version.ParserBaseline,
		}, nil
	})
	s.rpc.Register("agent.shutdown", func(_ context.Context, _ json.RawMessage) (any, error) {
		if s.cancelFunc != nil {
			go func() {
				time.Sleep(40 * time.Millisecond)
				s.cancelFunc()
			}()
		}
		return map[string]any{"status": "shutting_down"}, nil
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
		s.probe = config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths, DataRoot: s.dataRoot})
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
		if s.watcher == nil {
			return watcher.ScanReport{}, nil
		}
		cfg, _, err := config.Load(s.dataRoot)
		if err == nil {
			s.cfg = cfg
			targets := watcher.DiscoverWatchTargets(watcher.DiscoverOptions{
				Config: cfg,
				Probe:  s.probe,
			})
			s.watcher.SetTargets(targets)
		}
		rep, err := s.watcher.ScanNowWithReport(ctx)
		if err != nil {
			return nil, err
		}
		return rep, nil
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

func (s *Service) runMontageExport(spec composer.MontageSpec) {
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

// HandleDiscovered publishes demo.discovered and enqueues pending demos for parsing.
func (s *Service) HandleDiscovered(d watcher.DiscoveredDemo) {
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

// RunParseWorker processes pending demo IDs from the discovery queue.
func (s *Service) RunParseWorker(ctx context.Context) {
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
