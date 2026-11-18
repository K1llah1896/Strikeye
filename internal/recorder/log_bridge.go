package recorder

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// LogEventKind identifies a parsed Strikeye marker from CS2 -con_logfile.
type LogEventKind string

const (
	LogEventDemoLoaded LogEventKind = "demo_loaded"
	LogEventSegStart   LogEventKind = "seg_start"
	LogEventSegEnd     LogEventKind = "seg_end"
)

// LogEvent represents a structured marker parsed from CS2's console log.
type LogEvent struct {
	Kind      LogEventKind
	SessionID string
	TaskID    int64
	SegIndex  int
	RawLine   string
}

// ParseLogLine parses a single line from -con_logfile into a LogEvent if it
// contains a Strikeye protocol marker.
func ParseLogLine(line string) (LogEvent, bool) {
	trimmed := strings.TrimSpace(line)
	if idx := strings.Index(trimmed, "STRIKEYE_DEMO_LOADED:"); idx >= 0 {
		sid := strings.TrimSpace(trimmed[idx+len("STRIKEYE_DEMO_LOADED:"):])
		return LogEvent{
			Kind:      LogEventDemoLoaded,
			SessionID: sid,
			RawLine:   trimmed,
		}, true
	}
	if idx := strings.Index(trimmed, "STRIKEYE_EVT:"); idx >= 0 {
		rest := trimmed[idx+len("STRIKEYE_EVT:"):]
		parts := strings.Split(rest, ":")
		if len(parts) < 3 {
			return LogEvent{}, false
		}
		taskID, err1 := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		segIdx, err2 := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err1 != nil || err2 != nil {
			return LogEvent{}, false
		}
		switch parts[0] {
		case "START":
			return LogEvent{Kind: LogEventSegStart, TaskID: taskID, SegIndex: segIdx, RawLine: trimmed}, true
		case "END":
			return LogEvent{Kind: LogEventSegEnd, TaskID: taskID, SegIndex: segIdx, RawLine: trimmed}, true
		}
	}
	return LogEvent{}, false
}

// TailLogFile incrementally reads logPath from byte 0 until ctx is cancelled,
// emitting parsed LogEvents to eventsCh.
func TailLogFile(ctx context.Context, logPath string, pollInterval time.Duration, eventsCh chan<- LogEvent) error {
	if pollInterval <= 0 {
		pollInterval = 25 * time.Millisecond
	}

	var offset int64
	var partial string
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	readAvailable := func() error {
		f, err := os.Open(logPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("open con_logfile %q: %w", logPath, err)
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			return err
		}
		if info.Size() < offset {
			// File was truncated/recreated.
			offset = 0
			partial = ""
		}
		if info.Size() == offset {
			return nil
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return err
		}
		reader := bufio.NewReader(f)
		for {
			chunk, err := reader.ReadString('\n')
			if len(chunk) > 0 {
				offset += int64(len(chunk))
				if strings.HasSuffix(chunk, "\n") {
					full := partial + chunk
					partial = ""
					if ev, ok := ParseLogLine(full); ok {
						select {
						case eventsCh <- ev:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				} else {
					partial += chunk
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
		}
		return nil
	}

	for {
		if err := readAvailable(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
