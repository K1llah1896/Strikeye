// Package recorder implements Strikeye's zero-VPK HLAE + CS2 background
// capture pipeline, process safety guard, config backup/rollback, and render scheduler.
package recorder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const activeRestoreManifestName = "active_restore_manifest.json"

// BackupEntry records a single backed-up CS2 configuration file.
type BackupEntry struct {
	OriginalPath string `json:"original_path"`
	BackupPath   string `json:"backup_path"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
}

// RestoreManifest is persisted before any CS2 render session starts so an
// interrupted session can always roll back player settings atomically.
type RestoreManifest struct {
	SessionID string        `json:"session_id"`
	CreatedAt string        `json:"created_at"`
	Entries   []BackupEntry `json:"entries"`
}

// CFGBackupManager snapshots and atomically restores player CS2 configs.
type CFGBackupManager struct {
	backupRoot string
	steamDir   string
	cs2Dir     string
}

// NewCFGBackupManager constructs a manager rooted at backupRoot (<DataRoot>/backup/cs2_user_configs).
func NewCFGBackupManager(backupRoot, steamDir, cs2Dir string) *CFGBackupManager {
	return &CFGBackupManager{
		backupRoot: filepath.Clean(backupRoot),
		steamDir:   steamDir,
		cs2Dir:     cs2Dir,
	}
}

// ManifestPath returns the path to active_restore_manifest.json.
func (m *CFGBackupManager) ManifestPath() string {
	return filepath.Join(m.backupRoot, activeRestoreManifestName)
}

// DiscoverConfigFiles scans Steam userdata and CS2 cfg directories for player
// configuration files that must be protected during rendering.
func DiscoverConfigFiles(steamDir, cs2Dir string) ([]string, error) {
	var searchDirs []string
	if steamDir != "" {
		matches, err := filepath.Glob(filepath.Join(steamDir, "userdata", "*", "730", "local", "cfg"))
		if err != nil {
			return nil, fmt.Errorf("glob steam userdata cfg: %w", err)
		}
		searchDirs = append(searchDirs, matches...)
	}
	if cs2Dir != "" {
		searchDirs = append(searchDirs, filepath.Join(cs2Dir, "game", "csgo", "cfg"))
	}

	patterns := []string{
		"config.cfg",
		"cs2_user_keys_*.vcfg",
		"cs2_user_convars_*.vcfg",
		"cs2_video.txt",
	}

	seen := make(map[string]struct{})
	var files []string
	for _, dir := range searchDirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		for _, pat := range patterns {
			matches, err := filepath.Glob(filepath.Join(dir, pat))
			if err != nil {
				return nil, fmt.Errorf("glob %s in %s: %w", pat, dir, err)
			}
			for _, p := range matches {
				clean := filepath.Clean(p)
				fi, err := os.Stat(clean)
				if err != nil || fi.IsDir() {
					continue
				}
				if _, ok := seen[clean]; !ok {
					seen[clean] = struct{}{}
					files = append(files, clean)
				}
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

// BackupSession snapshots all discovered player CS2 config files and writes
// active_restore_manifest.json. If a pending manifest already exists, it is
// restored first so dirty render state never overwrites a clean snapshot.
func (m *CFGBackupManager) BackupSession(sessionID string) (RestoreManifest, error) {
	if sessionID == "" {
		return RestoreManifest{}, fmt.Errorf("sessionID is required")
	}
	if _, err := m.RestoreIfPending(); err != nil {
		return RestoreManifest{}, fmt.Errorf("restore pending manifest before new backup: %w", err)
	}
	if err := os.MkdirAll(m.backupRoot, 0o755); err != nil {
		return RestoreManifest{}, fmt.Errorf("create backup root: %w", err)
	}

	files, err := DiscoverConfigFiles(m.steamDir, m.cs2Dir)
	if err != nil {
		return RestoreManifest{}, err
	}

	sessionDir := filepath.Join(m.backupRoot, "session_"+sanitizeToken(sessionID))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return RestoreManifest{}, fmt.Errorf("create session backup dir: %w", err)
	}

	manifest := RestoreManifest{
		SessionID: sessionID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Entries:   make([]BackupEntry, 0, len(files)),
	}
	for i, src := range files {
		dstName := fmt.Sprintf("%03d_%s", i, filepath.Base(src))
		dst := filepath.Join(sessionDir, dstName)
		sum, size, err := copyFileWithSHA256(src, dst)
		if err != nil {
			return RestoreManifest{}, fmt.Errorf("backup %q: %w", src, err)
		}
		manifest.Entries = append(manifest.Entries, BackupEntry{
			OriginalPath: src,
			BackupPath:   dst,
			SHA256:       sum,
			Size:         size,
		})
	}

	if err := writeJSONAtomic(m.ManifestPath(), manifest); err != nil {
		return RestoreManifest{}, fmt.Errorf("write restore manifest: %w", err)
	}
	return manifest, nil
}

// RestoreIfPending rolls back any pending active_restore_manifest.json entries
// to their original paths and removes the manifest only when all entries succeed.
func (m *CFGBackupManager) RestoreIfPending() (int, error) {
	manifestPath := m.ManifestPath()
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read restore manifest: %w", err)
	}

	var manifest RestoreManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return 0, fmt.Errorf("parse restore manifest: %w", err)
	}

	restored := 0
	for _, entry := range manifest.Entries {
		if err := os.MkdirAll(filepath.Dir(entry.OriginalPath), 0o755); err != nil {
			return restored, fmt.Errorf("ensure restore dir for %q: %w", entry.OriginalPath, err)
		}
		if err := copyFileAtomic(entry.BackupPath, entry.OriginalPath); err != nil {
			return restored, fmt.Errorf("restore %q from %q: %w", entry.OriginalPath, entry.BackupPath, err)
		}
		restored++
	}

	if err := os.Remove(manifestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return restored, fmt.Errorf("remove restore manifest: %w", err)
	}
	return restored, nil
}

func copyFileWithSHA256(src, dst string) (string, int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", 0, err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", 0, err
	}

	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(out, h), in)
	closeErr := out.Close()
	if copyErr != nil {
		return "", 0, copyErr
	}
	if closeErr != nil {
		return "", 0, closeErr
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".strikeye-restore-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".strikeye-manifest-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func sanitizeToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "session"
	}
	return b.String()
}
