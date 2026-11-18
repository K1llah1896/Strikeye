package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCrossProcessMutexContentionAndRelease(t *testing.T) {
	dir := t.TempDir()
	lockA := NewLockInDir(dir, MutexCS2RenderActive)
	lockB := NewLockInDir(dir, MutexCS2RenderActive)

	acquiredA, err := lockA.TryAcquire()
	if err != nil || !acquiredA {
		t.Fatalf("lockA TryAcquire failed: acquired=%v err=%v", acquiredA, err)
	}

	held, err := lockB.IsHeld()
	if err != nil || !held {
		t.Fatalf("expected lockB IsHeld=true while lockA holds mutex, got held=%v err=%v", held, err)
	}

	acquiredB, err := lockB.TryAcquire()
	if err != nil {
		t.Fatalf("lockB TryAcquire returned unexpected error: %v", err)
	}
	if acquiredB {
		t.Fatalf("expected lockB TryAcquire=false while lockA holds mutex")
	}

	if err := lockA.Release(); err != nil {
		t.Fatalf("lockA Release failed: %v", err)
	}

	heldAfter, err := lockB.IsHeld()
	if err != nil || heldAfter {
		t.Fatalf("expected IsHeld=false after release, got held=%v err=%v", heldAfter, err)
	}

	acquiredB2, err := lockB.TryAcquire()
	if err != nil || !acquiredB2 {
		t.Fatalf("expected lockB TryAcquire=true after lockA release, got acquired=%v err=%v", acquiredB2, err)
	}
	_ = lockB.Release()
}

func TestRPCConcurrentRequestsMalformedFramesAndEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sockPath := filepath.Join(t.TempDir(), "strikeye-test.sock")
	ln, err := Listen(sockPath)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	srv := NewServer()
	srv.Register("agent.ping", func(_ context.Context, params json.RawMessage) (any, error) {
		var in map[string]int
		if len(params) > 0 {
			_ = json.Unmarshal(params, &in)
		}
		return map[string]any{"pong": true, "seq": in["seq"]}, nil
	})

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ctx, ln)
	}()

	client, err := DialClient(ctx, sockPath)
	if err != nil {
		t.Fatalf("DialClient failed: %v", err)
	}
	defer client.Close()

	if err := client.Subscribe(ctx); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish ordered events concurrently with RPC calls.
	const eventCount = 5
	for i := 0; i < eventCount; i++ {
		if err := srv.PublishEvent("parse.progress", map[string]int{"step": i}); err != nil {
			t.Fatalf("PublishEvent failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			var res struct {
				Pong bool `json:"pong"`
				Seq  int  `json:"seq"`
			}
			if err := client.Call(ctx, "agent.ping", map[string]int{"seq": seq}, &res); err != nil {
				t.Errorf("concurrent ping %d failed: %v", seq, err)
				return
			}
			if !res.Pong || res.Seq != seq {
				t.Errorf("unexpected ping response for %d: %+v", seq, res)
			}
		}(i)
	}
	wg.Wait()

	// Unknown method returns RPCError without crashing server.
	err = client.Call(ctx, "unknown.method", nil, nil)
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != ErrCodeMethodNotFound {
		t.Fatalf("expected ErrCodeMethodNotFound, got %v", err)
	}

	// Raw malformed JSON frame returns ParseError without closing connection.
	rawConn, err := Dial(ctx, sockPath)
	if err != nil {
		t.Fatalf("raw Dial failed: %v", err)
	}
	if _, err := rawConn.Write([]byte("{not-json\n")); err != nil {
		t.Fatalf("write malformed frame: %v", err)
	}
	buf := make([]byte, 512)
	_ = rawConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := rawConn.Read(buf)
	_ = rawConn.Close()
	if err != nil || n == 0 {
		t.Fatalf("expected parse error frame response, got n=%d err=%v", n, err)
	}
	var errEnv rpcEnvelope
	if uErr := json.Unmarshal(buf[:n], &errEnv); uErr != nil || errEnv.Error == nil || errEnv.Error.Code != ErrCodeParseError {
		t.Fatalf("expected JSON-RPC parse error frame, got %q (uErr=%v)", string(buf[:n]), uErr)
	}

	// Verify event ordering on subscribed client.
	for i := 0; i < eventCount; i++ {
		select {
		case ev := <-client.Events():
			if ev.Topic != "parse.progress" {
				t.Fatalf("unexpected event topic: %q", ev.Topic)
			}
			var body map[string]int
			if err := json.Unmarshal(ev.Params, &body); err != nil || body["step"] != i {
				t.Fatalf("out-of-order event at %d: %s", i, string(ev.Params))
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}

	// Server still healthy after malformed frame.
	var check map[string]any
	if err := client.Call(ctx, "agent.ping", map[string]int{"seq": 99}, &check); err != nil {
		t.Fatalf("post-malformed ping failed: %v", err)
	}

	cancel()
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve returned unexpected error on shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Serve did not exit on context cancel")
	}
	_ = fmt.Sprint("ok")
}
