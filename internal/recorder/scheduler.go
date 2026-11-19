package recorder

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"strikeye/internal/composer"
	"strikeye/internal/config"
	"strikeye/internal/ipc"
	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

const (
	DefaultUICooldown = 15 * time.Second
)

// SegmentWindow is the JSON payload stored in render_tasks.segments_json.
type SegmentWindow struct {
	StartTick int64 `json:"start_tick"`
	EndTick   int64 `json:"end_tick"`
}

// BatchSessionRequest describes a single-demo HLAE/CS2 capture batch.
type BatchSessionRequest struct {
	SessionID     string
	Demo          storage.Demo
	Tasks         []storage.RenderTask
	Segments      []RenderSegment
	TempRenderDir string
	BootCFGPath   string
	LogFilePath   string
	PlaybackDemo  string
}

// BatchTaskOutput holds the captured artifacts for a single task in a batch.
type BatchTaskOutput struct {
	TaskID          int64
	TakeDir         string
	PrebuiltMP4Path string
	Err             error
}

// BatchSessionResult summarizes the outcome of a SessionRunner.RunBatch call.
type BatchSessionResult struct {
	HLAEPID     int
	CS2PID      int
	TaskOutputs map[int64]BatchTaskOutput
}

// SessionRunner abstracts the HLAE/CS2 process lifecycle so production uses
// real HLAE.exe + -con_logfile while tests can plug in a Fake Runner.
type SessionRunner interface {
	RunBatch(ctx context.Context, req BatchSessionRequest, guard ProcessGuard, onProgress func(event string, payload map[string]any)) (BatchSessionResult, error)
}

// EventPublisher receives asynchronous scheduler status updates.
type EventPublisher func(topic string, payload map[string]any)

// SchedulerStatus reports live render scheduler state for IPC/UI queries.
type SchedulerStatus struct {
	Active         bool   `json:"active"`
	ActiveDemoID   int64  `json:"active_demo_id,omitempty"`
	ActiveSession  string `json:"active_session,omitempty"`
	DeferredReason string `json:"deferred_reason,omitempty"`
	QueuedCount    int    `json:"queued_count"`
}

// SchedulerOptions configures the Phase 4 render scheduler.
type SchedulerOptions struct {
	DataRoot    string
	Config      config.Config
	Probe       config.ProbeReport
	DB          *storage.DB
	UILock      ipc.Lock
	RenderLock  ipc.Lock
	Guard       ProcessGuard
	Runner      SessionRunner
	CFGBackup   *CFGBackupManager
	UICooldown  time.Duration
	PollPeriod  time.Duration
	Publish     EventPublisher
}

// Scheduler orchestrates policy-based render planning, UI/CS2 mutual exclusion,
// batch capture execution, FFprobe verification, and crash recovery.
type Scheduler struct {
	opts          SchedulerOptions
	mu            sync.Mutex
	active        bool
	activeDemoID  int64
	activeSession string
	activeCancel  context.CancelFunc
	activeDone    chan struct{}
	deferred      string
	forceNow      bool
	lastUIHeldAt  time.Time
	wakeCh        chan struct{}
}

// NewScheduler constructs a Scheduler with sensible defaults.
func NewScheduler(opts SchedulerOptions) *Scheduler {
	if opts.UICooldown <= 0 {
		opts.UICooldown = DefaultUICooldown
	}
	if opts.PollPeriod <= 0 {
		opts.PollPeriod = 2 * time.Second
	}
	if opts.Guard == nil {
		opts.Guard = NewProcessGuard()
	}
	if opts.CFGBackup == nil {
		layout, _ := config.ResolveDataLayout(opts.DataRoot)
		opts.CFGBackup = NewCFGBackupManager(layout.BackupCS2Configs, opts.Probe.Steam.Path, filepath.Dir(filepath.Dir(filepath.Dir(opts.Probe.CS2.Path))))
	}
	if opts.Runner == nil {
		opts.Runner = &HLAESessionRunner{
			Config: opts.Config,
			Probe:  opts.Probe,
		}
	}
	return &Scheduler{
		opts:   opts,
		wakeCh: make(chan struct{}, 4),
	}
}

func (s *Scheduler) emit(topic string, payload map[string]any) {
	if s.opts.Publish != nil {
		s.opts.Publish(topic, payload)
	}
}

// TriggerNow bypasses the 15-second UI cooldown (used by render.start_now after UI exits).
func (s *Scheduler) TriggerNow() {
	s.mu.Lock()
	s.forceNow = true
	s.mu.Unlock()
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

// Status returns a snapshot of the scheduler's current state.
func (s *Scheduler) Status(ctx context.Context) SchedulerStatus {
	queued, _ := s.opts.DB.RenderTasks().ListByStatus(ctx, "queued")
	s.mu.Lock()
	defer s.mu.Unlock()
	return SchedulerStatus{
		Active:         s.active,
		ActiveDemoID:   s.activeDemoID,
		ActiveSession:  s.activeSession,
		DeferredReason: s.deferred,
		QueuedCount:    len(queued),
	}
}

// RecoverInterrupted performs startup crash recovery: rolls back any pending
// CS2 config snapshot, cleans stale temporary render artifacts, and reconciles
// orphaned 'running' render_tasks rows.
func (s *Scheduler) RecoverInterrupted(ctx context.Context) (requeued int64, failed int64, restoredCFG int, err error) {
	if s.opts.CFGBackup != nil {
		restoredCFG, err = s.opts.CFGBackup.RestoreIfPending()
		if err != nil {
			return 0, 0, restoredCFG, fmt.Errorf("restore pending cfg: %w", err)
		}
	}
	s.cleanupTempArtifacts()
	requeued, failed, err = s.opts.DB.RenderTasks().RecoverInterrupted(ctx)
	return requeued, failed, restoredCFG, err
}

// PlanTasksForDemo evaluates RenderConfig.Policy after a demo is parsed and
// idempotently enqueues matching render_tasks.
func (s *Scheduler) PlanTasksForDemo(ctx context.Context, demoID int64) ([]storage.RenderTask, error) {
	policy := s.opts.Config.Render.Policy
	if policy == "" {
		policy = config.PolicyManualOnly
	}
	if policy == config.PolicyManualOnly {
		return nil, nil
	}

	demo, err := s.opts.DB.Demos().GetByID(ctx, demoID)
	if err != nil {
		return nil, fmt.Errorf("load demo %d: %w", demoID, err)
	}
	players, err := s.opts.DB.Analysis().ListMatchPlayers(ctx, demoID)
	if err != nil {
		return nil, fmt.Errorf("load players for demo %d: %w", demoID, err)
	}
	if len(players) == 0 {
		return nil, nil
	}

	target := selectTargetPlayer(players, s.opts.Config.PrimarySteamID64)
	if target.SteamID64 == "" {
		return nil, nil
	}

	tickRate := demo.TickRate
	if tickRate <= 0 {
		tickRate = 64
	}
	optsBytes, _ := json.Marshal(s.opts.Config.Render)
	optsJSON := string(optsBytes)

	specSlot := 1
	if target.SpecSlot != nil && *target.SpecSlot > 0 {
		specSlot = *target.SpecSlot
	}
	var created []storage.RenderTask
	switch policy {
	case config.PolicySmartHighlights:
		clips, err := s.opts.DB.Analysis().ListHighlightClips(ctx, demoID)
		if err != nil {
			return nil, fmt.Errorf("load highlights for demo %d: %w", demoID, err)
		}
		minImp := s.opts.Config.Render.HighlightImportanceMin
		for _, c := range clips {
			if c.PlayerSteamID64 != target.SteamID64 || c.ImportanceScore < minImp {
				continue
			}
			segsJSON, _ := json.Marshal([]SegmentWindow{{StartTick: c.StartTick, EndTick: c.EndTick}})
			clipID := c.ID
			rn := c.RoundNumber
			uid := storage.ComputeTaskUID(demo.ContentSHA256, "highlight_clip", target.SteamID64, "pov", string(segsJSON), optsJSON)
			task, wasCreated, err := s.opts.DB.RenderTasks().EnqueueIdempotent(ctx, storage.RenderTask{
				TaskUID:           uid,
				DemoID:            demo.ID,
				ClipID:            &clipID,
				RoundNumber:       &rn,
				TargetSteamID64:   target.SteamID64,
				TargetPlayerName:  target.PlayerName,
				TargetSpecSlot:    specSlot,
				Perspective:       "pov",
				TaskScope:         "highlight_clip",
				SegmentsJSON:      string(segsJSON),
				RenderOptionsJSON: optsJSON,
				Priority:          50,
			})
			if err != nil {
				return created, err
			}
			if wasCreated {
				created = append(created, task)
			}
		}

	case config.PolicyFullPOV:
		rounds, err := s.opts.DB.Analysis().ListMatchRounds(ctx, demoID)
		if err != nil {
			return nil, fmt.Errorf("load rounds for demo %d: %w", demoID, err)
		}
		preTicks := int64(2.0 * tickRate)
		for _, r := range rounds {
			startTick := r.FreezeEndTick - preTicks
			if startTick < r.StartTick {
				startTick = r.StartTick
			}
			endTick := ClampSegmentEndTick(r.EndTick, demo.DemoEndTick, tickRate)
			if endTick <= startTick {
				continue
			}
			segsJSON, _ := json.Marshal([]SegmentWindow{{StartTick: startTick, EndTick: endTick}})
			rn := r.RoundNumber
			uid := storage.ComputeTaskUID(demo.ContentSHA256, "round_full", target.SteamID64, "pov", string(segsJSON), optsJSON)
			task, wasCreated, err := s.opts.DB.RenderTasks().EnqueueIdempotent(ctx, storage.RenderTask{
				TaskUID:           uid,
				DemoID:            demo.ID,
				RoundNumber:       &rn,
				TargetSteamID64:   target.SteamID64,
				TargetPlayerName:  target.PlayerName,
				TargetSpecSlot:    specSlot,
				Perspective:       "pov",
				TaskScope:         "round_full",
				SegmentsJSON:      string(segsJSON),
				RenderOptionsJSON: optsJSON,
				Priority:          30,
			})
			if err != nil {
				return created, err
			}
			if wasCreated {
				created = append(created, task)
			}
		}
	}

	if len(created) > 0 {
		select {
		case s.wakeCh <- struct{}{}:
		default:
		}
	}
	return created, nil
}

// Run starts the background render loop until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.opts.PollPeriod)
	defer ticker.Stop()

	for {
		if _, err := s.RunNextBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			s.emit("render.error", map[string]any{"error": err.Error()})
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.wakeCh:
		case <-ticker.C:
		}
	}
}

// RunNextBatch checks preconditions (queued tasks, UI mutex/cooldown, external CS2),
// acquires MutexCS2RenderActive, backs up player CFG, executes one demo batch,
// verifies outputs, and guarantees cleanup + CFG restore.
func (s *Scheduler) RunNextBatch(ctx context.Context) (bool, error) {
	first, err := s.opts.DB.RenderTasks().NextQueued(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.setDeferred("")
			return false, nil
		}
		return false, err
	}

	if ok, reason := s.canStartRender(ctx); !ok {
		s.setDeferred(reason)
		return false, nil
	}
	s.setDeferred("")

	allQueued, err := s.opts.DB.RenderTasks().ListByStatus(ctx, "queued")
	if err != nil {
		return false, err
	}
	var batchTasks []storage.RenderTask
	for _, t := range allQueued {
		if t.DemoID == first.DemoID {
			batchTasks = append(batchTasks, t)
		}
	}
	if len(batchTasks) == 0 {
		return false, nil
	}

	demo, err := s.opts.DB.Demos().GetByID(ctx, first.DemoID)
	if err != nil {
		return false, fmt.Errorf("load demo %d: %w", first.DemoID, err)
	}

	if s.opts.RenderLock != nil {
		acquired, err := s.opts.RenderLock.TryAcquire()
		if err != nil {
			return false, fmt.Errorf("acquire render mutex: %w", err)
		}
		if !acquired {
			s.setDeferred("render_mutex_held")
			return false, nil
		}
		defer func() { _ = s.opts.RenderLock.Release() }()
	}

	sessionID := fmt.Sprintf("d%d_%d", demo.ID, time.Now().UnixNano())
	batchCtx, cancel := context.WithCancel(ctx)
	doneCh := make(chan struct{})

	s.mu.Lock()
	s.active = true
	s.activeDemoID = demo.ID
	s.activeSession = sessionID
	s.activeCancel = cancel
	s.activeDone = doneCh
	s.forceNow = false
	s.mu.Unlock()

	defer func() {
		cancel()
		s.mu.Lock()
		s.active = false
		s.activeDemoID = 0
		s.activeSession = ""
		s.activeCancel = nil
		close(doneCh)
		s.activeDone = nil
		s.mu.Unlock()
	}()

	if s.opts.CFGBackup != nil {
		if _, err := s.opts.CFGBackup.BackupSession(sessionID); err != nil {
			return true, fmt.Errorf("backup cs2 cfg: %w", err)
		}
		defer func() {
			_, _ = s.opts.CFGBackup.RestoreIfPending()
		}()
	}

	segments, err := buildBatchSegments(batchTasks, demo.DemoEndTick, demo.TickRate)
	if err != nil {
		return true, err
	}

	tempRoot := filepath.Join(s.opts.DataRoot, "cache", "render_tmp", sessionID)
	defer func() { _ = os.RemoveAll(tempRoot) }()

	for _, t := range batchTasks {
		_ = s.opts.DB.RenderTasks().MarkRunning(ctx, t.ID, nil, nil)
	}
	s.emit("render.session_started", map[string]any{
		"session_id": sessionID,
		"demo_id":    demo.ID,
		"task_count": len(batchTasks),
	})

	req := BatchSessionRequest{
		SessionID:     sessionID,
		Demo:          demo,
		Tasks:         batchTasks,
		Segments:      segments,
		TempRenderDir: filepath.Join(tempRoot, "captures"),
		BootCFGPath:   filepath.Join(tempRoot, fmt.Sprintf("_strikeye_boot_%s.cfg", sanitizeToken(sessionID))),
		LogFilePath:   filepath.Join(tempRoot, fmt.Sprintf("_strikeye_session_%s.log", sanitizeToken(sessionID))),
		PlaybackDemo:  filepath.Join(tempRoot, fmt.Sprintf("_strikeye_playback_%s.dem", sanitizeToken(sessionID))),
	}

	res, runErr := s.opts.Runner.RunBatch(batchCtx, req, s.opts.Guard, s.emit)
	if runErr != nil {
		for _, t := range batchTasks {
			if errors.Is(batchCtx.Err(), context.Canceled) {
				_ = s.opts.DB.RenderTasks().Cancel(context.Background(), t.ID)
			} else {
				_ = s.opts.DB.RenderTasks().MarkFailed(context.Background(), t.ID, "RUNNER_FAILED", runErr.Error())
			}
		}
		s.emit("render.session_finished", map[string]any{"session_id": sessionID, "status": "failed", "error": runErr.Error()})
		return true, runErr
	}

	layout, _ := config.ResolveDataLayout(s.opts.DataRoot)

	for _, t := range batchTasks {
		out, ok := res.TaskOutputs[t.ID]
		if !ok || out.Err != nil {
			msg := "missing capture output"
			if ok && out.Err != nil {
				msg = out.Err.Error()
			}
			_ = s.opts.DB.RenderTasks().MarkFailed(ctx, t.ID, "CAPTURE_FAILED", msg)
			continue
		}
		var inputEdges []parser.InputEdge
		if s.opts.Config.Render.KeyOverlayEnabled && demo.ReplayCachePath != nil && *demo.ReplayCachePath != "" && t.RoundNumber != nil {
			if _, rplRound, err := parser.ReadReplayRound(*demo.ReplayCachePath, uint32(*t.RoundNumber)); err == nil {
				inputEdges = rplRound.InputEdges
			}
		}
		segStart, segEnd := taskTickSpan(t)
		outDir := layout.MediaHighlights
		if t.TaskScope == "round_full" {
			outDir = layout.MediaPOVRounds
		}
		outMP4 := filepath.Join(outDir, fmt.Sprintf("demo_%d_task_%d.mp4", demo.ID, t.ID))
		media, err := MuxAndVerifyTask(ctx, s.opts.DB, t, MuxVerifyOptions{
			TakeDir:           out.TakeDir,
			PrebuiltMP4Path:   out.PrebuiltMP4Path,
			OutputMP4Path:     outMP4,
			FFmpegExe:         s.opts.Probe.FFmpeg.Path,
			FFprobeExe:        s.opts.Probe.FFprobe.Path,
			PreferredEncoder:  s.opts.Config.Render.Encoder,
			FPS:               s.opts.Config.Render.FPS,
			StartTick:         segStart,
			EndTick:           segEnd,
			TickRate:          demo.TickRate,
			KeyOverlayEnabled: s.opts.Config.Render.KeyOverlayEnabled,
			InputEdges:        inputEdges,
			PlayerSlot:        t.TargetSpecSlot,
			FrameMeld: composer.FrameMeldConfig{
				Enabled: s.opts.Config.Render.FrameMeldEnabled,
				ExePath: s.opts.Config.Paths.FrameMeldExe,
			},
			RequireAudio: true,
		})
		if err != nil {
			_ = s.opts.DB.RenderTasks().MarkFailed(ctx, t.ID, "VERIFY_FAILED", err.Error())
			s.emit("render.task_failed", map[string]any{"task_id": t.ID, "error": err.Error()})
			continue
		}
		_ = s.opts.DB.RenderTasks().MarkCompleted(ctx, t.ID, media.ID)
		s.emit("render.task_completed", map[string]any{"task_id": t.ID, "media_id": media.ID, "file_path": media.FilePath})
	}

	s.emit("render.session_finished", map[string]any{"session_id": sessionID, "status": "completed"})
	return true, nil
}

// AbortAndCleanup cancels any active render batch and waits for process cleanup,
// CFG rollback, and RenderMutex release.
func (s *Scheduler) AbortAndCleanup(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.activeCancel
	done := s.activeDone
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.opts.CFGBackup != nil {
		_, _ = s.opts.CFGBackup.RestoreIfPending()
	}
	return nil
}

func (s *Scheduler) canStartRender(ctx context.Context) (bool, string) {
	s.mu.Lock()
	force := s.forceNow
	lastUI := s.lastUIHeldAt
	s.mu.Unlock()

	if s.opts.UILock != nil && !force {
		held, err := s.opts.UILock.IsHeld()
		if err == nil && held {
			s.mu.Lock()
			s.lastUIHeldAt = time.Now()
			s.mu.Unlock()
			return false, "ui_active"
		}
		if !lastUI.IsZero() && time.Since(lastUI) < s.opts.UICooldown {
			return false, "ui_cooldown"
		}
	}
	if s.opts.Guard != nil {
		ext, err := s.opts.Guard.HasExternalCS2(ctx, nil)
		if err == nil && ext {
			return false, "external_cs2_running"
		}
	}
	return true, ""
}

func (s *Scheduler) setDeferred(reason string) {
	s.mu.Lock()
	prev := s.deferred
	s.deferred = reason
	s.mu.Unlock()
	if reason != "" && reason != prev {
		s.emit("render.deferred", map[string]any{"reason": reason})
	}
}

func (s *Scheduler) cleanupTempArtifacts() {
	tmpDir := filepath.Join(s.opts.DataRoot, "cache", "render_tmp")
	_ = os.RemoveAll(tmpDir)
	if s.opts.Probe.CS2.Path != "" {
		csgoDir := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(s.opts.Probe.CS2.Path))), "csgo")
		for _, pat := range []string{"_strikeye_playback_*.dem", "_strikeye_session_*.log", "cfg/_strikeye_boot_*.cfg"} {
			matches, _ := filepath.Glob(filepath.Join(csgoDir, pat))
			for _, m := range matches {
				_ = os.Remove(m)
			}
		}
	}
}

func selectTargetPlayer(players []storage.MatchPlayer, primarySteamID string) storage.MatchPlayer {
	if primarySteamID != "" {
		for _, p := range players {
			if p.SteamID64 == primarySteamID {
				return p
			}
		}
	}
	best := players[0]
	for _, p := range players[1:] {
		if p.StrikeyeRating > best.StrikeyeRating {
			best = p
		}
	}
	return best
}

func buildBatchSegments(tasks []storage.RenderTask, demoEndTick int64, tickRate float64) ([]RenderSegment, error) {
	var segs []RenderSegment
	for _, t := range tasks {
		var windows []SegmentWindow
		if err := json.Unmarshal([]byte(t.SegmentsJSON), &windows); err != nil {
			return nil, fmt.Errorf("parse segments_json for task %d: %w", t.ID, err)
		}
		for i, w := range windows {
			segs = append(segs, RenderSegment{
				TaskID:         t.ID,
				SegIndex:       i,
				StartTick:      w.StartTick,
				EndTick:        ClampSegmentEndTick(w.EndTick, demoEndTick, tickRate),
				TargetSpecSlot: t.TargetSpecSlot,
			})
		}
	}
	sort.Slice(segs, func(i, j int) bool {
		if segs[i].StartTick != segs[j].StartTick {
			return segs[i].StartTick < segs[j].StartTick
		}
		return segs[i].TaskID < segs[j].TaskID
	})
	return segs, nil
}

func taskTickSpan(t storage.RenderTask) (int64, int64) {
	var windows []SegmentWindow
	if err := json.Unmarshal([]byte(t.SegmentsJSON), &windows); err != nil || len(windows) == 0 {
		return 0, 0
	}
	return windows[0].StartTick, windows[len(windows)-1].EndTick
}

// HLAESessionRunner is the production SessionRunner that launches HLAE.exe
// inside a ProcessGuard Job Object and tails -con_logfile.
type HLAESessionRunner struct {
	Config config.Config
	Probe  config.ProbeReport
}

func (r *HLAESessionRunner) RunBatch(ctx context.Context, req BatchSessionRequest, guard ProcessGuard, onProgress func(string, map[string]any)) (BatchSessionResult, error) {
	if r.Probe.HLAE.Path == "" || r.Probe.AfxHookSource2.Path == "" || r.Probe.CS2.Path == "" {
		return BatchSessionResult{}, fmt.Errorf("HLAE/AfxHookSource2/CS2 toolchain paths are incomplete")
	}
	csgoDir := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(r.Probe.CS2.Path))), "csgo")
	cfgDir := filepath.Join(csgoDir, "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return BatchSessionResult{}, err
	}
	if err := os.MkdirAll(req.TempRenderDir, 0o755); err != nil {
		return BatchSessionResult{}, err
	}

	playbackPath, err := PreparePlaybackDemo(req.Demo.FilePath, csgoDir, req.SessionID)
	if err != nil {
		return BatchSessionResult{}, err
	}
	defer func() { _ = os.Remove(playbackPath) }()

	bootName := fmt.Sprintf("_strikeye_boot_%s.cfg", sanitizeToken(req.SessionID))
	bootPath := filepath.Join(cfgDir, bootName)
	logName := fmt.Sprintf("_strikeye_session_%s.log", sanitizeToken(req.SessionID))
	logPath := filepath.Join(csgoDir, logName)
	defer func() {
		_ = os.Remove(bootPath)
		_ = os.Remove(logPath)
	}()

	cfgText, err := BuildBootCFG(BootCFGOptions{
		SessionID:     req.SessionID,
		TempRenderDir: req.TempRenderDir,
		DemoPlayArg:   filepath.Base(playbackPath),
		DemoEndTick:   req.Demo.DemoEndTick,
		TickRate:      req.Demo.TickRate,
		Segments:      req.Segments,
	})
	if err != nil {
		return BatchSessionResult{}, err
	}
	if err := os.WriteFile(bootPath, []byte(cfgText), 0o644); err != nil {
		return BatchSessionResult{}, err
	}

	args, err := BuildHLAEArgs(LaunchOptions{
		SessionID:         req.SessionID,
		HLAEExe:           r.Probe.HLAE.Path,
		AfxHookSource2DLL: r.Probe.AfxHookSource2.Path,
		CS2Exe:            r.Probe.CS2.Path,
		BootCFGName:       bootName,
		LogFileName:       logName,
		Width:             r.Config.Render.Width,
		Height:            r.Config.Render.Height,
	})
	if err != nil {
		return BatchSessionResult{}, err
	}

	job, err := guard.NewJobGroup(req.SessionID)
	if err != nil {
		return BatchSessionResult{}, err
	}
	defer func() { _ = job.CloseAndKill() }()

	cmd, err := startDetachedCommand(ctx, r.Probe.HLAE.Path, args...)
	if err != nil {
		return BatchSessionResult{}, fmt.Errorf("launch HLAE: %w", err)
	}
	_ = job.Assign(cmd.Process.Pid)

	eventsCh := make(chan LogEvent, 64)
	tailCtx, tailCancel := context.WithCancel(ctx)
	defer tailCancel()
	go func() {
		_ = TailLogFile(tailCtx, logPath, 25*time.Millisecond, eventsCh)
	}()

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	outputs := make(map[int64]BatchTaskOutput, len(req.Tasks))
	for {
		select {
		case ev := <-eventsCh:
			if onProgress != nil {
				onProgress("render.log_event", map[string]any{
					"kind":    ev.Kind,
					"task_id": ev.TaskID,
					"seg_idx": ev.SegIndex,
				})
			}
			if ev.Kind == LogEventSegEnd {
				outputs[ev.TaskID] = BatchTaskOutput{
					TaskID:  ev.TaskID,
					TakeDir: req.TempRenderDir,
				}
			}
		case err := <-waitCh:
			tailCancel()
			if err != nil && !strings.Contains(err.Error(), "signal: killed") {
				return BatchSessionResult{HLAEPID: cmd.Process.Pid, TaskOutputs: outputs}, err
			}
			return BatchSessionResult{HLAEPID: cmd.Process.Pid, TaskOutputs: outputs}, nil
		case <-ctx.Done():
			tailCancel()
			_ = job.CloseAndKill()
			return BatchSessionResult{HLAEPID: cmd.Process.Pid, TaskOutputs: outputs}, ctx.Err()
		}
	}
}
