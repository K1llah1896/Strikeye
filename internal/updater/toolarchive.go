package updater

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// TarZstExt is the extension of the slimmed tool archives Strikeye ships
// (`ffmpeg-windows-amd64.tar.zst`). A solid tar stream lets zstd exploit the
// redundancy between ffmpeg.exe and ffprobe.exe, which are near-identical static
// builds: the pair is 51 MB instead of 80 MB as two independent zip entries.
const TarZstExt = ".tar.zst"

// IsTarZst reports whether path names a zstd-compressed tar archive.
func IsTarZst(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), TarZstExt)
}

// ExtractToolArchive unpacks a bundled tool archive, choosing the container from
// the file name: `.zip` (upstream HLAE builds) or `.tar.zst` (slimmed FFmpeg).
func ExtractToolArchive(archivePath, destDir string) error {
	if IsTarZst(archivePath) {
		return extractTarZstSafe(archivePath, destDir)
	}
	return extractZipSafe(archivePath, destDir)
}

// extractTarZstSafe unpacks a zstd-compressed tar into destDir, rejecting entries
// that would escape it.
func extractTarZstSafe(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("打开工具压缩包失败: %w", err)
	}
	defer f.Close()
	dec, err := zstd.NewReader(f, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(1<<30))
	if err != nil {
		return fmt.Errorf("解压工具压缩包失败: %w", err)
	}
	defer dec.Close()

	cleanDest := filepath.Clean(destDir)
	prefix := cleanDest + string(os.PathSeparator)
	tr := tar.NewReader(dec)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("读取工具压缩包失败: %w", err)
		}
		target := filepath.Clean(filepath.Join(cleanDest, hdr.Name))
		if target != cleanDest && !strings.HasPrefix(target, prefix) {
			return fmt.Errorf("检测到非法压缩包路径: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
}
