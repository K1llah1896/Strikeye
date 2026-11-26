package toolbundle

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"strikeye/internal/updater"
)

// ffmpegBinaries are the only files the app runs out of an upstream FFmpeg
// build. The full archive also carries ffplay.exe (a second 110 MB static
// binary) and HTML documentation, which would triple the sidecar for nothing.
var ffmpegBinaries = []string{"ffmpeg.exe", "ffprobe.exe"}

// PackFFmpeg rewrites an upstream FFmpeg archive into the slim tar.zst sidecar:
// just bin/ffmpeg.exe and bin/ffprobe.exe (plus the licence when upstream ships
// one), in one solid zstd frame. The result is verified by extracting it and
// re-running the same binary detection a first launch uses.
func PackFFmpeg(upstreamZip, destPath string) (int64, error) {
	zr, err := zip.OpenReader(upstreamZip)
	if err != nil {
		return 0, fmt.Errorf("打开 FFmpeg 压缩包失败: %w", err)
	}
	defer zr.Close()

	picked := map[string]*zip.File{}
	var licence *zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(f.Name))
		switch {
		case base == "ffmpeg.exe" || base == "ffprobe.exe":
			if picked[base] == nil {
				picked[base] = f
			}
		case base == "license" || base == "license.txt" || base == "copying.gplv3":
			if licence == nil {
				licence = f
			}
		}
	}
	for _, want := range ffmpegBinaries {
		if picked[want] == nil {
			return 0, fmt.Errorf("FFmpeg 压缩包缺少 %s", want)
		}
	}

	tmp := destPath + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	enc, err := zstd.NewWriter(out,
		zstd.WithEncoderLevel(zstd.SpeedBestCompression),
		zstd.WithEncoderConcurrency(1),
		zstd.WithWindowSize(128<<20),
	)
	if err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return 0, err
	}
	tw := tar.NewWriter(enc)
	write := func(name string, f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		hdr := &tar.Header{
			Name:    name,
			Mode:    0o755,
			Size:    int64(f.UncompressedSize64),
			ModTime: time.Unix(0, 0),
			Format:  tar.FormatPAX,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = io.Copy(tw, rc)
		return err
	}
	for _, want := range ffmpegBinaries {
		// bin/ mirrors the upstream layout, so the same detection walk works.
		if err := write("bin/"+want, picked[want]); err != nil {
			_ = tw.Close()
			_ = enc.Close()
			_ = out.Close()
			_ = os.Remove(tmp)
			return 0, fmt.Errorf("写入 %s: %w", want, err)
		}
	}
	if licence != nil {
		if err := write("LICENSE.txt", licence); err != nil {
			_ = tw.Close()
			_ = enc.Close()
			_ = out.Close()
			_ = os.Remove(tmp)
			return 0, err
		}
	}
	if err := tw.Close(); err != nil {
		_ = enc.Close()
		_ = out.Close()
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := enc.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	info, err := os.Stat(destPath)
	if err != nil {
		return 0, err
	}
	if err := verifyFFmpegArchive(destPath); err != nil {
		return info.Size(), err
	}
	return info.Size(), nil
}

// verifyFFmpegArchive extracts the sidecar into a scratch directory and checks
// both binaries are there, so a broken packaging run fails at build time.
func verifyFFmpegArchive(archivePath string) error {
	dir, err := os.MkdirTemp("", "strikeye-ffmpeg-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := updater.ExtractToolArchive(archivePath, dir); err != nil {
		return fmt.Errorf("校验内置 FFmpeg 失败: %w", err)
	}
	if _, err := updater.DetectToolBinaries(updater.ToolFFmpeg, dir); err != nil {
		return fmt.Errorf("校验内置 FFmpeg 失败: %w", err)
	}
	return nil
}

// TarZstMagic is the zstd frame magic the sidecar must start with.
var TarZstMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}

// LooksLikeTarZst is a cheap sanity check used by tests and the packer.
func LooksLikeTarZst(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len(TarZstMagic))
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return bytes.HasPrefix(head, TarZstMagic)
}
