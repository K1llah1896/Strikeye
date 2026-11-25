package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"strikeye/internal/storage"
)

func TestRunNotesLifecycle(t *testing.T) {
	dataRoot := t.TempDir()
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	demo, _, err := db.Demos().UpsertByContentHash(context.Background(), storage.Demo{
		FilePath:      "/demos/n.dem",
		Filename:      "n.dem",
		ContentSHA256: strings.Repeat("e", 64),
		ParseStatus:   "parsed",
		TotalRounds:   16,
	})
	if err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	db.Close()

	var out bytes.Buffer
	if err := runNotes(context.Background(), dataRoot, demo.ID, 3, "LongA", "默认双架", "red", 0, false, &out); err != nil {
		t.Fatalf("add: %v", err)
	}
	var listed struct {
		Count       int                     `json:"count"`
		Annotations []storage.MapAnnotation `json:"annotations"`
	}
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil || listed.Count != 1 {
		t.Fatalf("add output = %s err=%v", out.String(), err)
	}
	first := listed.Annotations[0]
	if first.RoundNumber != 3 || first.Note != "默认双架" || first.Color != "red" {
		t.Fatalf("annotation = %+v", first)
	}

	// Re-pinning the same callout at match scope adds a second row.
	out.Reset()
	if err := runNotes(context.Background(), dataRoot, demo.ID, 0, "Mid", "", "", 0, false, &out); err != nil {
		t.Fatalf("add match scope: %v", err)
	}
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil || listed.Count != 2 {
		t.Fatalf("count after second = %s err=%v", out.String(), err)
	}

	out.Reset()
	if err := runNotes(context.Background(), dataRoot, demo.ID, 0, "", "", "", first.ID, false, &out); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil || listed.Count != 1 {
		t.Fatalf("after delete = %s err=%v", out.String(), err)
	}

	out.Reset()
	if err := runNotes(context.Background(), dataRoot, demo.ID, 0, "", "", "", 0, true, &out); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil || listed.Count != 0 {
		t.Fatalf("after clear = %s err=%v", out.String(), err)
	}
	if !strings.Contains(out.String(), "\"annotations\": []") {
		t.Fatalf("empty list should serialise as []: %s", out.String())
	}

	if err := runNotes(context.Background(), dataRoot, demo.ID, 0, "", "", "", 999, false, &out); err == nil {
		t.Fatal("expected missing id error")
	}
}
