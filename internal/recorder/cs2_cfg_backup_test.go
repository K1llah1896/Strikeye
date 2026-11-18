package recorder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCFGBackupAndRestore(t *testing.T) {
	tmp := t.TempDir()
	steamDir := filepath.Join(tmp, "Steam")
	cs2Dir := filepath.Join(tmp, "CS2")
	backupRoot := filepath.Join(tmp, "DataRoot", "backup", "cs2_user_configs")

	userCfgDir := filepath.Join(steamDir, "userdata", "123456", "730", "local", "cfg")
	gameCfgDir := filepath.Join(cs2Dir, "game", "csgo", "cfg")
	if err := os.MkdirAll(userCfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gameCfgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	userConvar := filepath.Join(userCfgDir, "cs2_user_convars_0_slot0.vcfg")
	videoTxt := filepath.Join(userCfgDir, "cs2_video.txt")
	gameCfg := filepath.Join(gameCfgDir, "config.cfg")
	unrelated := filepath.Join(gameCfgDir, "gamemode_competitive.cfg")

	mustWrite := func(p, body string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(userConvar, "original_convar")
	mustWrite(videoTxt, "original_video")
	mustWrite(gameCfg, "original_game_cfg")
	mustWrite(unrelated, "leave_me_alone")

	mgr := NewCFGBackupManager(backupRoot, steamDir, cs2Dir)
	manifest, err := mgr.BackupSession("sess-1")
	if err != nil {
		t.Fatalf("BackupSession: %v", err)
	}
	if len(manifest.Entries) != 3 {
		t.Fatalf("expected 3 backed-up files, got %d", len(manifest.Entries))
	}

	// Simulate CS2 overwriting user configs during render.
	mustWrite(userConvar, "dirty_convar")
	mustWrite(videoTxt, "dirty_video")
	mustWrite(gameCfg, "dirty_game_cfg")

	// Starting a second backup while manifest is still pending must restore first.
	if _, err := mgr.BackupSession("sess-2"); err != nil {
		t.Fatalf("BackupSession sess-2: %v", err)
	}
	got, err := os.ReadFile(userConvar)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original_convar" {
		t.Fatalf("expected restored original_convar before sess-2 backup, got %q", string(got))
	}

	mustWrite(userConvar, "dirty_again")
	count, err := mgr.RestoreIfPending()
	if err != nil {
		t.Fatalf("RestoreIfPending: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 restored files, got %d", count)
	}
	if _, err := os.Stat(mgr.ManifestPath()); !os.IsNotExist(err) {
		t.Fatalf("expected manifest removed after restore, stat err=%v", err)
	}

	// Idempotent when nothing is pending.
	count2, err := mgr.RestoreIfPending()
	if err != nil || count2 != 0 {
		t.Fatalf("expected (0, nil) on second restore, got (%d, %v)", count2, err)
	}
}

func TestCFGBackupPartialFailurePreservesManifest(t *testing.T) {
	tmp := t.TempDir()
	steamDir := filepath.Join(tmp, "Steam")
	backupRoot := filepath.Join(tmp, "backup")
	userCfgDir := filepath.Join(steamDir, "userdata", "99", "730", "local", "cfg")
	if err := os.MkdirAll(userCfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(userCfgDir, "config.cfg")
	if err := os.WriteFile(cfg, []byte("clean"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewCFGBackupManager(backupRoot, steamDir, "")
	manifest, err := mgr.BackupSession("partial")
	if err != nil {
		t.Fatal(err)
	}
	// Temporarily move backup file away to simulate restore failure.
	bak := manifest.Entries[0].BackupPath
	hidden := bak + ".missing"
	if err := os.Rename(bak, hidden); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.RestoreIfPending(); err == nil {
		t.Fatalf("expected error when backup file is missing")
	}
	if _, err := os.Stat(mgr.ManifestPath()); err != nil {
		t.Fatalf("expected manifest preserved after partial failure: %v", err)
	}
	// Restore backup file and retry.
	if err := os.Rename(hidden, bak); err != nil {
		t.Fatal(err)
	}
	if n, err := mgr.RestoreIfPending(); err != nil || n != 1 {
		t.Fatalf("expected (1, nil) on retry, got (%d, %v)", n, err)
	}
}
