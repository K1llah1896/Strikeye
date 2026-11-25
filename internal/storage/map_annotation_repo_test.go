package storage

import (
	"context"
	"testing"
)

func TestMapAnnotationsUpsertAndDelete(t *testing.T) {
	db, err := OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	repo := db.MapAnnotations()

	first, err := repo.Add(ctx, MapAnnotation{DemoID: 7, RoundNumber: 3, CalloutID: "LongA", Note: "默认双架"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if first.ID == 0 || first.Color != "amber" || first.CreatedAt == "" {
		t.Fatalf("stored = %+v", first)
	}

	// Same key updates the note instead of creating a second row.
	updated, err := repo.Add(ctx, MapAnnotation{DemoID: 7, RoundNumber: 3, CalloutID: "LongA", Note: "改走 A 小", Color: "red"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ID != first.ID || updated.Note != "改走 A 小" || updated.Color != "red" {
		t.Fatalf("updated = %+v", updated)
	}

	if _, err := repo.Add(ctx, MapAnnotation{DemoID: 7, RoundNumber: 0, CalloutID: "Mid", Color: "nonsense"}); err != nil {
		t.Fatalf("add match wide: %v", err)
	}
	if _, err := repo.Add(ctx, MapAnnotation{DemoID: 8, RoundNumber: 1, CalloutID: "B"}); err != nil {
		t.Fatalf("add other demo: %v", err)
	}

	all, err := repo.ListByDemo(ctx, 7)
	if err != nil || len(all) != 2 {
		t.Fatalf("list = %+v err=%v", all, err)
	}
	if all[1].Color != "amber" {
		t.Fatalf("unknown color should fall back to amber, got %q", all[1].Color)
	}

	ok, err := repo.DeleteByKey(ctx, 7, 3, "LongA")
	if err != nil || !ok {
		t.Fatalf("delete by key = %v err=%v", ok, err)
	}
	if ok, _ := repo.DeleteByKey(ctx, 7, 3, "LongA"); ok {
		t.Fatal("second delete should report false")
	}
	if ok, err := repo.Delete(ctx, all[1].ID); err != nil || !ok {
		t.Fatalf("delete by id = %v err=%v", ok, err)
	}
	if rest, _ := repo.ListByDemo(ctx, 7); len(rest) != 0 {
		t.Fatalf("remaining = %+v", rest)
	}

	if _, err := repo.Add(ctx, MapAnnotation{DemoID: 7, CalloutID: "  "}); err == nil {
		t.Fatal("expected callout requirement")
	}
}
