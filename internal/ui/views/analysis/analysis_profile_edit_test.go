package analysis

import (
	"context"
	"testing"

	"strikeye/internal/storage"
)

func TestLoadProfileDraftSeedsFromStorage(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.PlayerProfiles().Upsert(ctx, storage.PlayerProfile{
		SteamID64: "76561198000000001", DisplayName: "KSC", Note: "anchor", Tags: []string{"clutch"},
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	v := &View{db: db}
	v.loadProfileDraft("76561198000000001")
	v.mu.RLock()
	draft := v.profileDraft
	v.mu.RUnlock()
	if !draft.Loaded || draft.DisplayName != "KSC" || draft.Note != "anchor" || draft.Tags != "clutch" {
		t.Fatalf("draft = %+v", draft)
	}

	// Loading a different player replaces the draft instead of reusing text.
	v.loadProfileDraft("76561198000000002")
	v.mu.RLock()
	draft = v.profileDraft
	v.mu.RUnlock()
	if draft.SteamID != "76561198000000002" || draft.DisplayName != "" {
		t.Fatalf("second draft = %+v", draft)
	}
}

func TestSaveProfileDraftPersistsThroughProfilePackage(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	v := &View{db: db}
	v.profileDraft = profileDraft{
		SteamID: "76561198000000003", DisplayName: "yuurih", Note: "support", Tags: "support, anchor", Loaded: true,
	}
	if err := v.saveProfileDraft("76561198000000003"); err != nil {
		t.Fatalf("save: %v", err)
	}
	stored, found, err := db.PlayerProfiles().Get(ctx, "76561198000000003")
	if err != nil || !found {
		t.Fatalf("get: %v found=%v", err, found)
	}
	if stored.DisplayName != "yuurih" || stored.Note != "support" {
		t.Fatalf("stored = %+v", stored)
	}
	if len(stored.Tags) != 2 || stored.Tags[0] != "support" || stored.Tags[1] != "anchor" {
		t.Fatalf("tags = %v", stored.Tags)
	}
	v.mu.RLock()
	saved := v.profileDraft.Saved
	v.mu.RUnlock()
	if saved != "已保存" {
		t.Fatalf("saved message = %q", saved)
	}
}

func TestSaveProfileDraftWithoutStorage(t *testing.T) {
	v := &View{}
	if err := v.saveProfileDraft("1"); err == nil {
		t.Fatal("expected an error without storage")
	}
}
