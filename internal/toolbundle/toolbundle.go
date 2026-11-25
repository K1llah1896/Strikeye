// Package toolbundle provisions Strikeye's external tools (FFmpeg, HLAE) from
// archives that ship inside the release package, so a fresh install never has
// to click "安装" or reach the network.
//
// The archives are data sidecars placed next to the executables
// (ffmpeg-windows-amd64.zip, hlae-windows-amd64.zip). They are produced by
// `strikeye-cli assets-pack --tools-out <dir>` at packaging time and extracted
// into <DataRoot>/tools/<tool> on first run, reusing the exact extraction and
// binary detection the network installer uses.
package toolbundle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"strikeye/internal/updater"
)

const (
	// TargetOS/TargetArch describe the platform the bundled archives are built
	// for. Strikeye ships Windows builds only, so the tool sidecars are fixed.
	TargetOS   = "windows"
	TargetArch = "amd64"

	HLAAArchiveName   = "hlae-" + TargetOS + "-" + TargetArch + ".zip"
	FFmpegArchiveName = "ffmpeg-" + TargetOS + "-" + TargetArch + ".zip"

	// Pinned upstream archives. Packaging uses these direct URLs instead of the
	// GitHub release API, so builds are reproducible and immune to the
	// unauthenticated API rate limit (60 requests/hour per IP).
	HLAEVersion      = "v2.193.2"
	HLAEArchiveURL   = "https://github.com/AdvancedFX/advancedfx/releases/download/" + HLAEVersion + "/hlae_2_193_2.zip"
	FFmpegArchiveURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
)

// PinnedURL returns the direct download URL for a tool archive, if one is
// pinned for the packaging path.
func PinnedURL(tool string) (string, bool) {
	switch tool {
	case updater.ToolHLAE:
		return HLAEArchiveURL, true
	case updater.ToolFFmpeg:
		return FFmpegArchiveURL, true
	}
	return "", false
}

// ArchiveName returns the sidecar file name for a tool id (updater.ToolHLAE /
// updater.ToolFFmpeg).
func ArchiveName(tool string) string {
	switch tool {
	case updater.ToolHLAE:
		return HLAAArchiveName
	case updater.ToolFFmpeg:
		return FFmpegArchiveName
	}
	return ""
}

// Dirs lists where sidecars are looked up, most specific first: next to the
// executable, its assets/ child, the working directory, then the DataRoot.
func Dirs(dataRoot, exeDir string) []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		dir = filepath.Clean(dir)
		if seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	add(exeDir)
	if exeDir != "" {
		add(filepath.Join(exeDir, "assets"))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(cwd)
	}
	if dataRoot != "" {
		add(dataRoot)
		add(filepath.Join(dataRoot, "cache"))
		add(filepath.Join(dataRoot, "assets"))
	}
	return dirs
}

// Find returns the sidecar path for tool when one of the searched directories
// holds it.
func Find(dataRoot, exeDir, tool string) (string, bool) {
	name := ArchiveName(tool)
	if name == "" {
		return "", false
	}
	for _, dir := range Dirs(dataRoot, exeDir) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Size() > 0 {
			return candidate, true
		}
	}
	return "", false
}

// StagedPath is where a tool is unpacked: <DataRoot>/tools/<tool>.
func StagedPath(dataRoot, tool string) string {
	return filepath.Join(filepath.Clean(dataRoot), "tools", strings.ToLower(tool))
}

// Report summarises one provisioning run.
type Report struct {
	// Installed lists tools extracted from a bundled archive during this run.
	Installed []string
	// Present lists tools that were already staged and therefore left alone.
	Present []string
	// Bundled lists the archives that were found (tool -> path).
	Bundled map[string]string
	// Missing lists tools with neither a staged copy nor a bundled archive.
	Missing []string
}

// Ready reports whether no tool would be left unprovisioned.
func (r Report) Ready() bool { return len(r.Missing) == 0 }

// Provision stages every bundled tool that is not present yet. Tools without a
// sidecar are reported in Missing and never fail the call: a source build
// simply keeps the in-app installer as its path.
func Provision(dataRoot, exeDir string) (Report, error) {
	rep := Report{Bundled: map[string]string{}}
	if strings.TrimSpace(dataRoot) == "" {
		return rep, fmt.Errorf("DataRoot 不能为空")
	}
	for _, tool := range []string{updater.ToolFFmpeg, updater.ToolHLAE} {
		archive, ok := Find(dataRoot, exeDir, tool)
		if ok {
			rep.Bundled[tool] = archive
		}
		if toolStaged(dataRoot, tool) {
			rep.Present = append(rep.Present, tool)
			continue
		}
		if !ok {
			rep.Missing = append(rep.Missing, tool)
			continue
		}
		if _, err := updater.InstallToolFromArchive(tool, archive, dataRoot, nil); err != nil {
			return rep, fmt.Errorf("解压内置 %s 失败: %w", tool, err)
		}
		rep.Installed = append(rep.Installed, tool)
	}
	return rep, nil
}

// toolStaged reports whether the tool's required binaries already exist.
func toolStaged(dataRoot, tool string) bool {
	dir := StagedPath(dataRoot, tool)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return false
	}
	_, err := updater.DetectToolBinaries(tool, dir)
	return err == nil
}

// Summary renders a short Chinese status line for notifications.
func (r Report) Summary() string {
	var parts []string
	if len(r.Installed) > 0 {
		parts = append(parts, "已内置安装 "+strings.Join(r.Installed, " / "))
	}
	if len(r.Present) > 0 {
		parts = append(parts, "已就绪 "+strings.Join(r.Present, " / "))
	}
	if len(r.Missing) > 0 {
		parts = append(parts, "缺 "+strings.Join(r.Missing, " / "))
	}
	if len(parts) == 0 {
		return "无外部工具需要安装"
	}
	return strings.Join(parts, "；")
}
