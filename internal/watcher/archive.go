package watcher

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"compress/bzip2"

	"github.com/klauspost/compress/zstd"
)

// Archive limits guard against zip bombs and runaway extractions.
const (
	MaxArchiveEntries    = 32
	MaxArchiveEntryBytes = int64(4) << 30 // 4 GiB
	MaxArchiveTotalBytes = int64(8) << 30 // 8 GiB
)

var (
	// ErrArchiveLimit reports that an archive exceeded an extraction limit.
	ErrArchiveLimit = errors.New("demo archive exceeds extraction limits")
	// ErrArchiveEmpty reports that an archive contained no .dem entry.
	ErrArchiveEmpty = errors.New("demo archive contains no .dem file")
)

// ArchiveKind returns a stable kind name for a supported demo archive path.
// Supported: .zip, .gz, .bz2, .zst (including ".dem.gz" style names).
func ArchiveKind(path string) (string, bool) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return "zip", true
	case strings.HasSuffix(lower, ".gz"):
		return "gzip", true
	case strings.HasSuffix(lower, ".bz2"):
		return "bzip2", true
	case strings.HasSuffix(lower, ".zst"):
		return "zstd", true
	default:
		return "", false
	}
}

// IsDemoArchive reports whether path is a supported demo archive.
func IsDemoArchive(path string) bool {
	_, ok := ArchiveKind(path)
	return ok
}

// ExtractDemoArchives extracts every .dem file from archivePath into destRoot.
// Entry names are flattened to their base name and written atomically, so an
// archive can never write outside destRoot. Existing identical files are reused,
// making repeated scans idempotent.
func ExtractDemoArchives(ctx context.Context, archivePath, destRoot string) ([]string, error) {
	kind, ok := ArchiveKind(archivePath)
	if !ok {
		return nil, fmt.Errorf("unsupported demo archive: %s", filepath.Base(archivePath))
	}
	outDir := filepath.Join(destRoot, archiveExtractDirName(archivePath))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create import dir: %w", err)
	}

	switch kind {
	case "zip":
		return extractZip(ctx, archivePath, outDir)
	default:
		return extractSingleStream(ctx, kind, archivePath, outDir)
	}
}

func archiveExtractDirName(archivePath string) string {
	base := strings.TrimSuffix(filepath.Base(archivePath), filepath.Ext(archivePath))
	base = sanitizeEntryBase(base)
	if base == "" {
		base = "archive"
	}
	sum := sha256.Sum256([]byte(filepath.Clean(archivePath)))
	return base + "-" + hex.EncodeToString(sum[:4])
}

func extractZip(ctx context.Context, archivePath, outDir string) ([]string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	if len(zr.File) > MaxArchiveEntries {
		return nil, fmt.Errorf("%w: %d entries (max %d)", ErrArchiveLimit, len(zr.File), MaxArchiveEntries)
	}

	var (
		paths []string
		total int64
		used  = make(map[string]int)
	)
	for _, f := range zr.File {
		if ctx.Err() != nil {
			return paths, ctx.Err()
		}
		if f.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(f.Name), ".dem") {
			continue
		}
		if f.UncompressedSize64 > uint64(MaxArchiveEntryBytes) {
			return paths, fmt.Errorf("%w: entry %q is %d bytes", ErrArchiveLimit, f.Name, f.UncompressedSize64)
		}
		name := uniqueEntryName(sanitizeEntryBase(filepath.Base(f.Name)), used)
		if name == "" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return paths, fmt.Errorf("open zip entry %q: %w", f.Name, err)
		}
		written, writeErr := writeLimited(ctx, rc, filepath.Join(outDir, name), total)
		_ = rc.Close()
		if writeErr != nil {
			return paths, fmt.Errorf("extract zip entry %q: %w", f.Name, writeErr)
		}
		total += written
		paths = append(paths, filepath.Join(outDir, name))
	}
	if len(paths) == 0 {
		return nil, ErrArchiveEmpty
	}
	return paths, nil
}

func extractSingleStream(ctx context.Context, kind, archivePath, outDir string) ([]string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	var reader io.Reader
	switch kind {
	case "gzip":
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("open gzip: %w", err)
		}
		defer gr.Close()
		reader = gr
	case "bzip2":
		reader = bzip2.NewReader(f)
	case "zstd":
		zr, err := zstd.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("open zstd: %w", err)
		}
		defer zr.Close()
		reader = zr
	default:
		return nil, fmt.Errorf("unsupported archive kind %q", kind)
	}

	name := sanitizeEntryBase(strings.TrimSuffix(filepath.Base(archivePath), filepath.Ext(archivePath)))
	if !strings.EqualFold(filepath.Ext(name), ".dem") {
		name += ".dem"
	}
	if name == "" || name == ".dem" {
		name = "demo.dem"
	}
	dest := filepath.Join(outDir, name)
	if info, statErr := os.Stat(dest); statErr == nil && info.Size() > 0 {
		return []string{dest}, nil
	}
	if _, err := writeLimited(ctx, reader, dest, 0); err != nil {
		return nil, fmt.Errorf("extract %s: %w", kind, err)
	}
	return []string{dest}, nil
}

// writeLimited copies src into dest atomically, enforcing the per-entry cap and
// the global remaining budget (carried by alreadyWritten).
func writeLimited(ctx context.Context, src io.Reader, dest string, alreadyWritten int64) (int64, error) {
	remaining := MaxArchiveTotalBytes - alreadyWritten
	if remaining <= 0 {
		return 0, fmt.Errorf("%w: total size limit reached", ErrArchiveLimit)
	}
	limit := MaxArchiveEntryBytes
	if remaining < limit {
		limit = remaining
	}

	tmp := dest + ".part"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}

	written, copyErr := io.Copy(out, io.LimitReader(&ctxReader{ctx: ctx, r: src}, limit+1))
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return written, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return written, closeErr
	}
	if written > limit {
		_ = os.Remove(tmp)
		return written, fmt.Errorf("%w: entry exceeds %d bytes", ErrArchiveLimit, limit)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return written, err
	}
	return written, nil
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// sanitizeEntryBase reduces an archive entry to a safe flat file name.
func sanitizeEntryBase(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	switch name {
	case "", ".", "..", "/":
		return ""
	}
	// Strip characters that are illegal on Windows and can confuse shells.
	name = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '|', '?', '*', 0:
			return '_'
		}
		return r
	}, name)
	return name
}

func uniqueEntryName(base string, used map[string]int) string {
	if base == "" {
		return ""
	}
	key := strings.ToLower(base)
	if n, ok := used[key]; ok {
		used[key] = n + 1
		ext := filepath.Ext(base)
		return strings.TrimSuffix(base, ext) + fmt.Sprintf("-%d", n+1) + ext
	}
	used[key] = 1
	return base
}
