package voice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ToolExitError describes a non-zero exit from an external decoder. The
// conventional decoder (csgo-voice-extractor) uses distinct exit codes for
// argument, parse, codec and format failures, so the code is surfaced verbatim.
type ToolExitError struct {
	Code   int
	Output string
}

func (e *ToolExitError) Error() string {
	msg := fmt.Sprintf("voice: decoder exited with code %d", e.Code)
	if hint := exitHint(e.Code); hint != "" {
		msg += ": " + hint
	}
	if out := strings.TrimSpace(e.Output); out != "" {
		msg += " (" + truncate(out, 240) + ")"
	}
	return msg
}

// exitHint maps the documented csgo-voice-extractor exit codes to a hint.
func exitHint(code int) string {
	switch code {
	case 10:
		return "bad arguments"
	case 11:
		return "demo could not be opened"
	case 12:
		return "demo has no voice data"
	case 13:
		return "unsupported voice codec"
	case 14:
		return "audio write failed"
	}
	if code >= 10 && code <= 20 {
		return "decoder reported a handled failure"
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// DecodeWithTool runs an external Steam-voice decoder over the demo and returns
// the WAV files it produced. The tool receives "<tool> <demo> <outDir>" which is
// the calling convention of csgo-voice-extractor; any tool honouring it works.
func DecodeWithTool(ctx context.Context, tool, demoPath, outDir string) ([]string, error) {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return nil, fmt.Errorf("voice: decoder tool path is required")
	}
	if _, err := os.Stat(demoPath); err != nil {
		return nil, fmt.Errorf("voice: demo not found: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	before, err := listWAVs(outDir)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, tool, demoPath, outDir)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, &ToolExitError{Code: exitErr.ExitCode(), Output: out.String()}
		}
		return nil, fmt.Errorf("voice: run decoder: %w", err)
	}

	after, err := listWAVs(outDir)
	if err != nil {
		return nil, err
	}
	produced := make([]string, 0, len(after))
	for path := range after {
		if !before[path] {
			produced = append(produced, path)
		}
	}
	sort.Strings(produced)
	return produced, nil
}

func listWAVs(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".wav") {
			continue
		}
		out[filepath.Join(dir, e.Name())] = true
	}
	return out, nil
}
