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
	// ImportDir receives demos extracted from archives. It defaults to
	// <user cache>/strikeye/imports and is excluded from directory walks.
	ImportDir    string
	OnDiscovered func(DiscoveredDemo)
	OnWarning    func(path string, err error)
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
	if strings.TrimSpace(opts.ImportDir) == "" {
		opts.ImportDir = defaultImportDir()
	}
	opts.ImportDir = filepath.Clean(opts.ImportDir)
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
				if path != target.Dir && isUnderDir(path, w.opts.ImportDir) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(d.Name()), ".dem") && !IsDemoArchive(d.Name()) {
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

	if IsDemoArchive(path) {
		return w.processArchive(ctx, path, target, tr, now)
	}
	return w.ingestDemoFile(ctx, path, target.Platform, tr.size, target.Dir, tr)
}

// processArchive extracts every .dem entry from a stabilized archive and
// registers each extracted demo through the normal validation/dedupe pipeline.
func (w *Watcher) processArchive(ctx context.Context, archivePath string, target WatchTarget, tr *fileTrack, now time.Time) (bool, string) {
	platform := target.Platform
	if platform == "" || platform == PlatformUnknown {
		platform = ClassifyPlatform(archivePath)
	}

	extracted, err := ExtractDemoArchives(ctx, archivePath, w.opts.ImportDir)
	if err != nil {
		tr.rejected = true
		w.warn(archivePath, err)
		return false, fmt.Sprintf("%s: %v", filepath.Base(archivePath), err)
	}

	createdAny := false
	var warns []string
	for _, demoPath := range extracted {
		info, statErr := os.Stat(demoPath)
		if statErr != nil || info.IsDir() {
			continue
		}
		created, warn := w.ingestDemoFile(ctx, demoPath, platform, info.Size(), w.opts.ImportDir, nil)
		createdAny = createdAny || created
		if warn != "" {
			warns = append(warns, warn)
		}
	}
	tr.ingested = true
	if len(warns) > 0 {
		return createdAny, strings.Join(warns, "; ")
	}
	return createdAny, ""
}

// ingestDemoFile validates, hashes, and registers one demo file, applying any
// sidecar .info metadata without overwriting parsed data or user annotations.
func (w *Watcher) ingestDemoFile(ctx context.Context, path, platform string, size int64, watchRoot string, tr *fileTrack) (bool, string) {
	if err := ValidateDemoFile(path, size, w.opts.MinFileSize); err != nil {
		if tr != nil {
			tr.rejected = true
		}
		w.warn(path, err)
		return false, fmt.Sprintf("%s: %v", filepath.Base(path), err)
	}

	hash, err := StreamSHA256(path)
	if err != nil {
		if tr != nil {
			tr.rejected = true
		}
		w.warn(path, err)
		return false, fmt.Sprintf("%s: %v", filepath.Base(path), err)
	}

	meta, hasSidecar, sidecarErr := ReadSidecar(path)
	if sidecarErr != nil {
		w.warn(path, sidecarErr)
	}
	if platform == "" || platform == PlatformUnknown {
		if meta.SourcePlatform != "" {
			platform = meta.SourcePlatform
		} else {
			platform = ClassifyPlatform(path)
		}
	}
	if platform == "" {
		platform = PlatformUnknown
	}

	root := watchRoot
	demo := storage.Demo{
		FilePath:       path,
		Filename:       filepath.Base(path),
		FileSize:       size,
		ContentSHA256:  hash,
		SourcePlatform: platform,
		WatchRoot:      &root,
	}
	if hasSidecar {
		if meta.MatchDate != "" {
			demo.MatchDate = strPtr(meta.MatchDate)
		}
		if meta.TeamAName != "" {
			demo.TeamAName = meta.TeamAName
		}
		if meta.TeamBName != "" {
			demo.TeamBName = meta.TeamBName
		}
		if meta.HasScore {
			demo.TeamAScore = meta.TeamAScore
			demo.TeamBScore = meta.TeamBScore
		}
		if meta.MatchID != "" {
			demo.Remark = strPtr(meta.MatchID)
		}
	}

	registered, created, err := w.db.Demos().UpsertByContentHash(ctx, demo)
	if err != nil {
		if tr != nil {
			tr.rejected = true
		}
		w.warn(path, err)
		return false, fmt.Sprintf("%s: %v", filepath.Base(path), err)
	}

	if hasSidecar && !meta.IsZero() {
		if _, applyErr := w.db.Demos().ApplyIngestMetadata(ctx, registered.ID, storage.IngestMetadata{
			SourcePlatform: meta.SourcePlatform,
			MatchDate:      meta.MatchDate,
			TeamAName:      meta.TeamAName,
			TeamBName:      meta.TeamBName,
			TeamAScore:     meta.TeamAScore,
			TeamBScore:     meta.TeamBScore,
			HasScore:       meta.HasScore,
			MatchID:        meta.MatchID,
		}); applyErr != nil {
			w.warn(path, applyErr)
		} else if updated, getErr := w.db.Demos().GetByID(ctx, registered.ID); getErr == nil {
			registered = updated
		}
	}

	if tr != nil {
		tr.ingested = true
	}
	if w.opts.OnDiscovered != nil {
		w.opts.OnDiscovered(DiscoveredDemo{Demo: registered, Created: created})
	}
	return created, ""
}

func (w *Watcher) warn(path string, err error) {
	if w.opts.OnWarning != nil {
		w.opts.OnWarning(path, err)
	}
}

func strPtr(s string) *string { return &s }

// defaultImportDir returns a stable per-user directory for extracted demos.
func defaultImportDir() string {
	if cache, err := os.UserCacheDir(); err == nil && cache != "" {
		return filepath.Join(cache, "strikeye", "imports")
	}
	return filepath.Join(os.TempDir(), "strikeye-imports")
}

func isUnderDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..")
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
