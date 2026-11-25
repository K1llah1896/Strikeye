package settings

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"strikeye/internal/sqlconsole"
	"strikeye/internal/storage"
)

func TestFormatCell(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"abc", "abc"},
		{[]byte("blob"), "blob"},
		{float64(3), "3"},
		{float64(1.5), "1.5"},
		{int64(-7), "-7"},
		{true, "true"},
		{struct{}{}, "{}"},
	}
	for _, tc := range cases {
		if got := formatCell(tc.in); got != tc.want {
			t.Fatalf("formatCell(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWriteQueryCSV(t *testing.T) {
	root := t.TempDir()
	result := sqlconsole.Result{
		Columns: []string{"name", "adr"},
		Rows:    [][]any{{"KSCERATO", 106.29}, {"yuurih", float64(80)}},
	}
	path, err := writeQueryCSV(result, root, time.Date(2026, 11, 25, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if filepath.Dir(path) != filepath.Join(root, "media", "exports") {
		t.Fatalf("path = %q", path)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %v", records)
	}
	if strings.Join(records[0], ",") != "name,adr" {
		t.Fatalf("header = %v", records[0])
	}
	if records[1][1] != "106.29" {
		t.Fatalf("row = %v", records[1])
	}
	if records[2][1] != "80" {
		t.Fatalf("int-ish float not normalised: %v", records[2])
	}
}

func TestRunConsoleQueryRejectsWrites(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	v := &View{db: db, sqlQuery: "delete from demos", sqlBusy: true}
	v.runConsoleQuery()
	if v.sqlErr == "" {
		t.Fatal("expected validation error")
	}
	if v.sqlBusy {
		t.Fatal("busy flag should be cleared")
	}
}

func TestRunConsoleQuerySelects(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, _, err := db.Demos().UpsertByContentHash(ctx, storage.Demo{
		FilePath: "/d/q.dem", Filename: "q.dem", ContentSHA256: "q", MapName: stringPtr("de_ancient"),
		TeamAName: "A", TeamBName: "B", ParseStatus: "parsed",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	v := &View{db: db, sqlQuery: "select map_name from demos", sqlBusy: true}
	v.runConsoleQuery()
	if v.sqlErr != "" {
		t.Fatalf("query error: %v", v.sqlErr)
	}
	if len(v.sqlResult.Rows) != 1 || formatCell(v.sqlResult.Rows[0][0]) != "de_ancient" {
		t.Fatalf("result = %+v", v.sqlResult)
	}
	if len(sqlPresets) == 0 {
		t.Fatal("presets should not be empty")
	}
}

func stringPtr(s string) *string { return &s }
