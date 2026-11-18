package guard

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"strikeye/internal/ipc"
)

type stubIPC struct {
	calls  []string
	onCall func(method string, result any) error
}

func (s *stubIPC) Call(_ context.Context, method string, _ any, result any) error {
	s.calls = append(s.calls, method)
	if s.onCall != nil {
		return s.onCall(method, result)
	}
	return nil
}

func TestGuardPollAndAbortLifecycle(t *testing.T) {
	lock := ipc.NewLockInDir(filepath.Join(t.TempDir(), "locks"), ipc.MutexCS2RenderActive)
	held, err := lock.TryAcquire()
	if err != nil || !held {
		t.Fatalf("expected lock acquire, got held=%v err=%v", held, err)
	}

	stub := &stubIPC{
		onCall: func(method string, result any) error {
			switch method {
			case "render.status":
				m := result.(*map[string]any)
				*m = map[string]any{
					"rendering":          true,
					"queued_count":       float64(3),
					"current_task_label": "de_mirage R12 - donk (4K)",
				}
			case "render.abort_and_cleanup":
				_ = lock.Release()
			}
			return nil
		},
	}

	ctrl := NewController(stub, lock)
	if err := ctrl.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
	snap, aborting, errText := ctrl.Snapshot()
	if !snap.Rendering || snap.QueuedCount != 3 || snap.CurrentTaskLabel != "de_mirage R12 - donk (4K)" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if aborting || errText != "" {
		t.Fatalf("unexpected aborting=%v err=%q", aborting, errText)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ctrl.AbortAndWait(ctx, 20*time.Millisecond); err != nil {
		t.Fatalf("AbortAndWait: %v", err)
	}
	if ctrl.Decision() != DecisionOpenUI {
		t.Fatalf("expected DecisionOpenUI, got %v", ctrl.Decision())
	}
}
