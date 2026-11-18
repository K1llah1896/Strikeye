package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"strikeye/internal/parser"
)

func TestRunDoctor(t *testing.T) {
	var buf bytes.Buffer
	dataRoot := t.TempDir()
	if err := runDoctor(context.Background(), dataRoot, &buf); err != nil {
		t.Fatalf("runDoctor failed: %v", err)
	}

	var summary doctorSummary
	if err := json.Unmarshal(buf.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal doctor summary: %v\nraw:\n%s", err, buf.String())
	}
	if summary.SchemaVersion < 1 || summary.IPCSelfCheck != "ok" {
		t.Fatalf("unexpected doctor summary: %+v", summary)
	}
}

func TestRunInspectReplayFile(t *testing.T) {
	tmp := t.TempDir()
	rplPath := filepath.Join(tmp, "sample.rpl")
	if err := parser.WriteReplayFileAtomic(rplPath, 64.0, parser.InputCapEntityButtons, []parser.ReplayRoundData{
		{RoundNumber: 1, StartTick: 100, EndTick: 400},
	}); err != nil {
		t.Fatalf("write replay: %v", err)
	}

	var buf bytes.Buffer
	if err := runInspect(context.Background(), tmp, 0, rplPath, 1, &buf); err != nil {
		t.Fatalf("runInspect failed: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal inspect output: %v", err)
	}
	if out["input_capability"] != parser.InputCapEntityButtons {
		t.Fatalf("unexpected input_capability: %v", out["input_capability"])
	}
}

func TestRunRenderDryRun(t *testing.T) {
	dataRoot := t.TempDir()
	var buf bytes.Buffer
	if err := runRenderDryRun(context.Background(), dataRoot, 0, true, &buf); err != nil {
		t.Fatalf("runRenderDryRun: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if out["dry_run"] != true {
		t.Fatalf("expected dry_run=true, got %+v", out)
	}
	cfgStr, _ := out["sample_boot_cfg"].(string)
	if !strings.Contains(cfgStr, "mirv_cmd addAtTick") {
		t.Fatalf("expected mirv_cmd addAtTick in sample_boot_cfg, got %q", cfgStr)
	}
}
