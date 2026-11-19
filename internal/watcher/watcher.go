package watcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"strikeye/internal/storage"
)

const (
	DefaultSampleInterval = 1500 * time.Millisecond
	DefaultStableDuration = 3000 * time.Millisecond
	DefaultMinSamples     = 2
	DefaultMinFileSize    = int64(1024 * 1024) // 1 MB
	DemoHeaderMagic       = "PBDEMS2\x00"
)

var (
	ErrDemoTooSmall     = errors.New("demo file smaller than minimum threshold")
	ErrInvalidDemoMagic = errors.New("invalid demo header magic (expected PBDEMS2)")
)

// Options configures the demo directory watcher and write-completion stabilizer.
type Options struct {
	Targets        []WatchTarget
	SampleInterval time.Duration
	StableDuration time.Duration
	MinSamples     int
	MinFileSize    int64
	OnDiscovered   func(DiscoveredDemo)
	OnWarning      func(path string, err error)
}

// DiscoveredDemo reports a stabilized, deduplicated demo registration event.
type DiscoveredDemo struct {
	Demo    storage.Demo
	Created bool
}

type fileTrack struct {
	size       int64
	modTime    time.Time
	firstSeen  time.Time
	lastChange time.Time
	samples    int
	rejected   bool
	ingested   bool
}

// Watcher scans configured WatchTargets, stabilizes in-progress .dem writes,
// validates PBDEMS2 headers, and registers demos idempotently by content SHA-256.
type Watcher struct {
	mu     sync.Mutex
	db     *storage.DB
	opts   Options
	tracks map[string]*fileTrack
}

// New constructs a Watcher bound to db.
func New(db *storage.DB, opts Options) *Watcher {
	if opts.SampleInterval <= 0 {
		opts.SampleInterval = DefaultSampleInterval
	}
	if opts.StableDuration <= 0 {
		opts.StableDuration = DefaultStableDuration
	}
	if opts.MinSamples <= 0 {
		opts.MinSamples = DefaultMinSamples
	}
	if opts.MinFileSize <= 0 {
		opts.MinFileSize = DefaultMinFileSize
	}
	return &Watcher{
		db:     db,
		opts:   opts,
		tracks: make(map[string]*fileTrack),
	}
}

// Run polls WatchTargets at SampleInterval until ctx is canceled.
func (w *Watcher) Run(ctx context.Context) error {
	if err := w.PollOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(w.opts.SampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := w.PollOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
}

// ScanReport summarizes the outcome of a manual or reload-triggered directory scan.
type ScanReport struct {
	Targets    int      `json:"targets"`
	Candidates int      `json:"candidates"`
	Discovered int      `json:"discovered"`
	Warnings   []string `json:"warnings,omitempty"`
}

// SetTargets updates the active watch targets dynamically.
func (w *Watcher) SetTargets(targets []WatchTarget) {
	w.mu.Lock()
	w.opts.Targets = append([]WatchTarget(nil), targets...)
	w.mu.Unlock()
}

// ScanNow immediately scans all configured targets and ingests any .dem file
// whose modification time is already older than StableDuration.
func (w *Watcher) ScanNow(ctx context.Context) error {
	_, err := w.ScanNowWithReport(ctx)
	return err
}

// ScanNowWithReport immediately scans all configured targets and returns a diagnostic summary.
func (w *Watcher) ScanNowWithReport(ctx context.Context) (ScanReport, error) {
	return w.scanOnce(ctx, true)
}

// PollOnce performs one scan + stabilization step across all configured targets.
func (w *Watcher) PollOnce(ctx context.Context) error {
	_, err := w.scanOnce(ctx, false)
	return err
}

func (w *Watcher) scanOnce(ctx context.Context, immediate bool) (ScanReport, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	seenPaths := make(map[string]struct{})
	targets := append([]WatchTarget(nil), w.opts.Targets...)
	rep := ScanReport{Targets: len(targets)}

	for _, target := range targets {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		err := filepath.WalkDir(target.Dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: %v", path, walkErr))
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(d.Name()), ".dem") {
				return nil
			}
			rep.Candidates++
			clean := filepath.Clean(path)
			seenPaths[clean] = struct{}{}
			if added, warn := w.processCandidate(ctx, clean, target, now, immediate); warn != "" {
				rep.Warnings = append(rep.Warnings, warn)
			} else if added {
				rep.Discovered++
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return rep, err
		}
	}

	// Prune deleted files from tracking map.
	for path := range w.tracks {
		if _, ok := seenPaths[path]; !ok {
			delete(w.tracks, path)
		}
	}
	return rep, nil
}

func (w *Watcher) processCandidate(ctx context.Context, path string, target WatchTarget, now time.Time, immediate bool) (bool, string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false, ""
	}

	tr, exists := w.tracks[path]
	if !exists {
		tr = &fileTrack{
			size:       info.Size(),
			modTime:    info.ModTime(),
			firstSeen:  now,
			lastChange: now,
			samples:    1,
		}
		w.tracks[path] = tr
		if !immediate || now.Sub(info.ModTime()) < w.opts.StableDuration {
			return false, ""
		}
	} else {
		if tr.size != info.Size() || !tr.modTime.Equal(info.ModTime()) {
			tr.size = info.Size()
			tr.modTime = info.ModTime()
			tr.lastChange = now
			tr.samples = 1
			tr.rejected = false
			tr.ingested = false
			return false, ""
		}

		if tr.ingested || tr.rejected {
			return false, ""
		}

		tr.samples++
		if !immediate && (tr.samples < w.opts.MinSamples || now.Sub(tr.lastChange) < w.opts.StableDuration) {
			return false, ""
		}
		if immediate && now.Sub(info.ModTime()) < w.opts.StableDuration {
			return false, ""
		}
	}

	if err := ValidateDemoFile(path, tr.size, w.opts.MinFileSize); err != nil {
		tr.rejected = true
		if w.opts.OnWarning != nil {
			w.opts.OnWarning(path, err)
		}
		return false, fmt.Sprintf("%s: %v", filepath.Base(path), err)
	}

	hash, err := StreamSHA256(path)
	if err != nil {
		if w.opts.OnWarning != nil {
			w.opts.OnWarning(path, err)
		}
		return false, fmt.Sprintf("%s: %v", filepath.Base(path), err)
	}

	platform := target.Platform
	if platform == "" || platform == PlatformUnknown {
		platform = ClassifyPlatform(path)
	}
	watchRoot := target.Dir

	demo, created, err := w.db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath:       path,
		Filename:       filepath.Base(path),
		FileSize:       tr.size,
		ContentSHA256:  hash,
		SourcePlatform: platform,
		WatchRoot:      &watchRoot,
	})
	if err != nil {
		if w.opts.OnWarning != nil {
			w.opts.OnWarning(path, err)
		}
		return false, fmt.Sprintf("%s: %v", filepath.Base(path), err)
	}

	tr.ingested = true
	if w.opts.OnDiscovered != nil {
		w.opts.OnDiscovered(DiscoveredDemo{Demo: demo, Created: created})
	}
	return created, ""
}

// ValidateDemoFile verifies minimum file size, non-exclusive readability, and PBDEMS2 header magic.
func ValidateDemoFile(path string, knownSize, minSize int64) error {
	if knownSize <= minSize {
		return fmt.Errorf("%w: got %d bytes, need > %d", ErrDemoTooSmall, knownSize, minSize)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()

	var header [8]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return fmt.Errorf("read demo header: %w", err)
	}
	if !bytes.Equal(header[:], []byte(DemoHeaderMagic)) {
		return fmt.Errorf("%w: got %q", ErrInvalidDemoMagic, string(header[:]))
	}
	return nil
}

// StreamSHA256 computes the lowercase hex SHA-256 digest of file at path using streaming I/O.
func StreamSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open for sha256: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.CopyBuffer(h, f, make([]byte, 1024*1024)); err != nil {
		return "", fmt.Errorf("hash demo: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
