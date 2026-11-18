package recorder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTailLogFileEmitsEvents(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "_strikeye_session_t1.log")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	events := make(chan LogEvent, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- TailLogFile(ctx, logPath, 10*time.Millisecond, events)
	}()

	time.Sleep(30 * time.Millisecond)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("some cs2 console noise\n")
	_, _ = f.WriteString("STRIKEYE_DEMO_LOADED:t1\n")
	_, _ = f.WriteString("STRIKEYE_EVT:START:10:0\n")
	_, _ = f.WriteString("STRIKEYE_EVT:END:10:0\n")
	_ = f.Close()

	var got []LogEvent
	for len(got) < 3 {
		select {
		case ev := <-events:
			got = append(got, ev)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for log events; got %d", len(got))
		}
	}
	cancel()
	<-errCh

	if got[0].Kind != LogEventDemoLoaded || got[0].SessionID != "t1" {
		t.Fatalf("unexpected event 0: %+v", got[0])
	}
	if got[1].Kind != LogEventSegStart || got[1].TaskID != 10 || got[1].SegIndex != 0 {
		t.Fatalf("unexpected event 1: %+v", got[1])
	}
	if got[2].Kind != LogEventSegEnd || got[2].TaskID != 10 || got[2].SegIndex != 0 {
		t.Fatalf("unexpected event 2: %+v", got[2])
	}
}
