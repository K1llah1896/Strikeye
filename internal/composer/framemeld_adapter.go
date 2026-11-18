package composer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FrameMeldConfig configures optional external FrameMeld CLI post-processing.
type FrameMeldConfig struct {
	Enabled    bool
	ExePath    string
	TargetFPS  int
	MotionBlur float64
}

// RunFrameMeld invokes the external FrameMeld CLI if enabled and configured;
// otherwise it returns inPath unchanged without error.
func RunFrameMeld(ctx context.Context, cfg FrameMeldConfig, inPath, outPath string) (string, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.ExePath) == "" {
		return inPath, nil
	}
	if _, err := os.Stat(cfg.ExePath); err != nil {
		if _, lookErr := exec.LookPath(cfg.ExePath); lookErr != nil {
			return inPath, fmt.Errorf("framemeld executable not found %q: %w", cfg.ExePath, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", err
	}

	args := []string{"-i", inPath, "-o", outPath}
	if cfg.TargetFPS > 0 {
		args = append(args, "--fps", fmt.Sprintf("%d", cfg.TargetFPS))
	}
	if cfg.MotionBlur > 0 {
		args = append(args, "--blur", fmt.Sprintf("%.2f", cfg.MotionBlur))
	}
	out, err := exec.CommandContext(ctx, cfg.ExePath, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("framemeld failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return outPath, nil
}
