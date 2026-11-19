package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strikeye/internal/agent"
	"strikeye/internal/config"
	"strikeye/internal/ipc"
	"strikeye/internal/storage"
	"strikeye/internal/watcher"
)

func TestAgentServiceRPCAndParseEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tmp := t.TempDir()
	dataRoot := filepath.Join(tmp, "data")
	cfg, _, err := config.Load(dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	svc := agent.NewService(dataRoot, cfg, db)
	go svc.RunParseWorker(ctx)

	sock := filepath.Join(tmp, "agent-test.sock")
	if os.PathSeparator == '\\' {
		sock = fmt.Sprintf(`\\.\pipe\strikeye-agent-test-%d`, time.Now().UnixNano())
	}
	ln, err := ipc.Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() { _ = svc.RPC().Serve(ctx, ln) }()

	conn, err := ipc.Dial(ctx, sock)
	if err != nil {
		t.Fatal(err)
	}
	cli := ipc.NewClient(conn)
	defer cli.Close()

	if err := cli.Subscribe(ctx); err != nil {
		t.Fatal(err)
	}
	eventsCh := cli.Events()

	var ping map[string]any
	if err := cli.Call(ctx, "agent.ping", nil, &ping); err != nil {
		t.Fatal(err)
	}
	if ping["status"] != "ok" {
		t.Fatalf("unexpected ping: %+v", ping)
	}

	// Register a corrupt demo and trigger handleDiscovered -> parse.failed event.
	badDemo := filepath.Join(tmp, "bad.dem")
	buf := make([]byte, 2048)
	copy(buf[:8], watcher.DemoHeaderMagic)
	if err := os.WriteFile(badDemo, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	d, created, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:      badDemo,
		Filename:      "bad.dem",
		FileSize:      2048,
		ContentSHA256: "badsha256",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.HandleDiscovered(watcher.DiscoveredDemo{Demo: d, Created: created})

	seenTopics := make(map[string]bool)
	for len(seenTopics) < 3 {
		select {
		case ev := <-eventsCh:
			seenTopics[ev.Topic] = true
		case <-ctx.Done():
			t.Fatalf("timed out waiting for agent events; got %v", seenTopics)
		}
	}

	var raw []json.RawMessage
	if err := cli.Call(ctx, "demo.list", nil, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 demo in demo.list, got %d", len(raw))
	}

	var enqRes map[string]any
	if err := cli.Call(ctx, "render.enqueue", map[string]any{
		"demo_id":            d.ID,
		"target_steam_id64":  "76561198000000001",
		"target_player_name": "donk",
		"target_spec_slot":   1,
		"start_tick":         100,
		"end_tick":           400,
	}, &enqRes); err != nil {
		t.Fatalf("render.enqueue: %v", err)
	}
	if enqRes["created"] != true {
		t.Fatalf("expected created=true, got %+v", enqRes)
	}

	var st map[string]any
	if err := cli.Call(ctx, "render.status", nil, &st); err != nil {
		t.Fatalf("render.status: %v", err)
	}
	if st["queued_count"] != float64(1) {
		t.Fatalf("expected queued_count=1, got %+v", st)
	}

	var startRes map[string]any
	if err := cli.Call(ctx, "render.start_now", nil, &startRes); err != nil {
		t.Fatalf("render.start_now: %v", err)
	}
	var abortRes map[string]any
	if err := cli.Call(ctx, "render.abort_and_cleanup", nil, &abortRes); err != nil {
		t.Fatalf("render.abort_and_cleanup: %v", err)
	}

	taskObj := enqRes["task"].(map[string]any)
	taskID := int64(taskObj["ID"].(float64))

	var prioRes map[string]any
	if err := cli.Call(ctx, "render.priority", map[string]any{"task_id": taskID, "priority": 250}, &prioRes); err != nil {
		t.Fatalf("render.priority: %v", err)
	}
	if prioRes["priority"] != float64(250) {
		t.Fatalf("unexpected priority response: %+v", prioRes)
	}

	var cancelRes map[string]any
	if err := cli.Call(ctx, "render.cancel", map[string]any{"task_id": taskID}, &cancelRes); err != nil {
		t.Fatalf("render.cancel: %v", err)
	}
	if cancelRes["status"] != "cancelled" {
		t.Fatalf("unexpected cancel response: %+v", cancelRes)
	}

	var retryRes map[string]any
	if err := cli.Call(ctx, "render.retry", map[string]any{"task_id": taskID}, &retryRes); err != nil {
		t.Fatalf("render.retry: %v", err)
	}
	if retryRes["status"] != "queued" {
		t.Fatalf("unexpected retry response: %+v", retryRes)
	}

	var reloadRes map[string]any
	if err := cli.Call(ctx, "config.reload", nil, &reloadRes); err != nil {
		t.Fatalf("config.reload: %v", err)
	}
	if reloadRes["status"] != "reloaded" {
		t.Fatalf("unexpected reload response: %+v", reloadRes)
	}

	var montageRes map[string]any
	if err := cli.Call(ctx, "montage.export", map[string]any{
		"spec": map[string]any{"output_path": filepath.Join(tmp, "out.mp4")},
	}, &montageRes); err != nil {
		t.Fatalf("montage.export: %v", err)
	}
	if montageRes["status"] != "started" {
		t.Fatalf("unexpected montage.export response: %+v", montageRes)
	}
}
