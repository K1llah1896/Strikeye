package recorder

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"strikeye/internal/config"
	"strikeye/internal/storage"
)

type fakeProcessGuard struct {
	mu          sync.Mutex
	externalCS2 bool
	killed      bool
}

func (f *fakeProcessGuard) HasExternalCS2(_ context.Context, _ []ProcessIdentity) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.externalCS2, nil
}

func (f *fakeProcessGuard) NewJobGroup(_ string) (JobHandle, error) {
	return &fakeJobHandle{parent: f}, nil
}

type fakeJobHandle struct {
	parent *fakeProcessGuard
}

func (j *fakeJobHandle) Assign(_ int) error { return nil }
func (j *fakeJobHandle) CloseAndKill() error {
	j.parent.mu.Lock()
	j.parent.killed = true
	j.parent.mu.Unlock()
	return nil
}

type fakeLock struct {
	mu   sync.Mutex
	held bool
}

func (l *fakeLock) Name() string { return "fake" }
func (l *fakeLock) TryAcquire() (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return false, nil
	}
	l.held = true
	return true, nil
}
func (l *fakeLock) IsHeld() (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held, nil
}
func (l *fakeLock) Release() error {
	l.mu.Lock()
	l.held = false
	l.mu.Unlock()
	return nil
}

type fakeSessionRunner struct {
	mu        sync.Mutex
	calls     []BatchSessionRequest
	mp4Source string
	blockCh   chan struct{}
}

func (r *fakeSessionRunner) RunBatch(ctx context.Context, req BatchSessionRequest, _ ProcessGuard, _ func(string, map[string]any)) (BatchSessionResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, req)
	r.mu.Unlock()

	if r.blockCh != nil {
		select {
		case <-r.blockCh:
		case <-ctx.Done():
			return BatchSessionResult{}, ctx.Err()
		}
	}

	outs := make(map[int64]BatchTaskOutput, len(req.Tasks))
	for _, t := range req.Tasks {
		outs[t.ID] = BatchTaskOutput{
			TaskID:          t.ID,
			PrebuiltMP4Path: r.mp4Source,
		}
	}
	return BatchSessionResult{HLAEPID: 1001, CS2PID: 1002, TaskOutputs: outs}, nil
}

func TestSchedulerEndToEndBatchAndRecovery(t *testing.T) {
	tmp := t.TempDir()
	db, err := storage.OpenDefault(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Prepare fake steam/cs2 cfg so backup & restore are exercised end-to-end.
	steamDir := filepath.Join(tmp, "Steam")
	userCfgDir := filepath.Join(steamDir, "userdata", "1", "730", "local", "cfg")
	if err := os.MkdirAll(userCfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(userCfgDir, "config.cfg")
	if err := os.WriteFile(cfgFile, []byte("bind w +forward"), 0o644); err != nil {
		t.Fatal(err)
	}

	ffprobeScript := filepath.Join(tmp, "ffprobe-ok.sh")
	if err := os.WriteFile(ffprobeScript, []byte(`#!/bin/sh
echo '{"format":{"duration":"4.0"},"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"r_frame_rate":"60/1","avg_frame_rate":"60/1"},{"codec_type":"audio","codec_name":"aac","sample_rate":"48000"}]}'
`), 0o755); err != nil {
		t.Fatal(err)
	}
	rawMP4 := filepath.Join(tmp, "fake_capture.mp4")
	if err := os.WriteFile(rawMP4, make([]byte, 70*1024), 0o644); err != nil {
		t.Fatal(err)
	}

	mapName := "de_inferno"
	demo, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
		Filename:      "match.dem",
		FilePath:      filepath.Join(tmp, "match.dem"),
		ContentSHA256: "deadbeef",
		MapName:       &mapName,
		TickRate:      64,
		DemoEndTick:   20000,
	})
	if err != nil {
		t.Fatal(err)
	}

	slot := 2
	if err := db.Analysis().ReplaceMatchPlayers(context.Background(), demo.ID, []storage.MatchPlayer{
		{DemoID: demo.ID, SteamID64: "76561198000000001", PlayerName: "ropz", SpecSlot: &slot, StrikeyeRating: 1.45},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Analysis().ReplaceHighlightClips(context.Background(), demo.ID, []storage.HighlightClip{
		{ID: "c2", DemoID: demo.ID, RoundNumber: 8, PlayerSteamID64: "76561198000000001", PlayerName: "ropz", ClipType: "3k", StartTick: 5000, EndTick: 5400, ImportanceScore: 80},
		{ID: "c1", DemoID: demo.ID, RoundNumber: 3, PlayerSteamID64: "76561198000000001", PlayerName: "ropz", ClipType: "4k", StartTick: 1200, EndTick: 1800, ImportanceScore: 95},
	}); err != nil {
		t.Fatal(err)
	}

	uiLock := &fakeLock{}
	renderLock := &fakeLock{}
	guard := &fakeProcessGuard{}
	runner := &fakeSessionRunner{mp4Source: rawMP4}
	cfg := config.DefaultConfig(tmp)
	cfg.Render.Policy = config.PolicySmartHighlights
	cfg.Render.KeyOverlayEnabled = false

	layout, _ := config.ResolveDataLayout(tmp)
	cfgMgr := NewCFGBackupManager(layout.BackupCS2Configs, steamDir, "")

	sched := NewScheduler(SchedulerOptions{
		DataRoot: tmp,
		Config:   cfg,
		Probe: config.ProbeReport{
			FFprobe: config.ToolStatus{Available: true, Path: ffprobeScript},
			Steam:   config.ToolStatus{Available: true, Path: steamDir},
		},
		DB:         db,
		UILock:     uiLock,
		RenderLock: renderLock,
		Guard:      guard,
		Runner:     runner,
		CFGBackup:  cfgMgr,
		UICooldown: 50 * time.Millisecond,
	})

	planned, err := sched.PlanTasksForDemo(context.Background(), demo.ID)
	if err != nil {
		t.Fatalf("PlanTasksForDemo: %v", err)
	}
	if len(planned) != 2 {
		t.Fatalf("expected 2 planned tasks, got %d", len(planned))
	}

	// External CS2 must defer rendering.
	guard.externalCS2 = true
	ran, err := sched.RunNextBatch(context.Background())
	if err != nil || ran {
		t.Fatalf("expected deferred when external CS2 active; ran=%v err=%v", ran, err)
	}
	guard.externalCS2 = false

	// UI active must defer rendering.
	_, _ = uiLock.TryAcquire()
	ran, err = sched.RunNextBatch(context.Background())
	if err != nil || ran {
		t.Fatalf("expected deferred when UI active; ran=%v err=%v", ran, err)
	}
	_ = uiLock.Release()

	// TriggerNow bypasses cooldown and executes both tasks in one batch ordered by start_tick.
	sched.TriggerNow()
	ran, err = sched.RunNextBatch(context.Background())
	if err != nil || !ran {
		t.Fatalf("expected batch execution; ran=%v err=%v", ran, err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 batched RunBatch call, got %d", len(runner.calls))
	}
	if runner.calls[0].Segments[0].StartTick != 1200 || runner.calls[0].Segments[1].StartTick != 5000 {
		t.Fatalf("expected segments sorted by ascending start_tick, got %+v", runner.calls[0].Segments)
	}

	completed, err := db.RenderTasks().ListByStatus(context.Background(), "completed")
	if err != nil || len(completed) != 2 {
		t.Fatalf("expected 2 completed tasks, got %d (err=%v)", len(completed), err)
	}
	if held, _ := renderLock.IsHeld(); held {
		t.Fatalf("expected renderLock released after batch")
	}
	if _, err := os.Stat(cfgMgr.ManifestPath()); !os.IsNotExist(err) {
		t.Fatalf("expected CFG backup manifest cleared after batch")
	}

	// Crash recovery test: mark one task running and leave a pending manifest.
	_, _ = cfgMgr.BackupSession("crash-test")
	_ = os.WriteFile(cfgFile, []byte("corrupted"), 0o644)
	_ = db.RenderTasks().MarkRunning(context.Background(), completed[0].ID, nil, nil)

	requeued, failed, restored, err := sched.RecoverInterrupted(context.Background())
	if err != nil {
		t.Fatalf("RecoverInterrupted: %v", err)
	}
	if requeued != 1 || failed != 0 || restored != 1 {
		t.Fatalf("unexpected recovery counts: requeued=%d failed=%d restored=%d", requeued, failed, restored)
	}
	gotCfg, _ := os.ReadFile(cfgFile)
	if string(gotCfg) != "bind w +forward" {
		t.Fatalf("expected CFG rolled back on recovery, got %q", string(gotCfg))
	}
}
