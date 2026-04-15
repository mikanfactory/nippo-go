package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupFakeClaude(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	projects := filepath.Join(dir, "projects", "example")
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("testdata/sample.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projects, "s.jsonl"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunMarkdown(t *testing.T) {
	dir := setupFakeClaude(t)
	var out, errb bytes.Buffer
	err := run([]string{
		"--claude-dir=" + dir,
		"--from=2026-04-01",
		"--to=2026-04-30",
	}, &out, &errb)
	if err != nil {
		t.Fatalf("run: %v (stderr=%s)", err, errb.String())
	}
	body := out.String()
	for _, want := range []string{"# 振り返り", "project-a", "Read", "ALACT"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in output:\n%s", want, body)
		}
	}
}

func TestRunJSON(t *testing.T) {
	dir := setupFakeClaude(t)
	var out bytes.Buffer
	err := run([]string{
		"--claude-dir=" + dir,
		"--from=2026-04-01",
		"--to=2026-04-30",
		"--format=json",
	}, &out, os.Stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("json: %v body=%s", err, out.String())
	}
	if meta, _ := parsed["meta"].(map[string]any); meta["total_sessions"].(float64) != 1 {
		t.Errorf("meta = %+v", meta)
	}
}

func TestRunProjectFilterNoMatch(t *testing.T) {
	dir := setupFakeClaude(t)
	var out bytes.Buffer
	err := run([]string{
		"--claude-dir=" + dir,
		"--from=2026-04-01",
		"--to=2026-04-30",
		"--project=nonexistent",
	}, &out, os.Stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "記録されたセッションはありません") {
		t.Errorf("expected empty msg, got:\n%s", out.String())
	}
}

func TestRunPeriod(t *testing.T) {
	dir := setupFakeClaude(t)
	var out bytes.Buffer
	err := run([]string{
		"--claude-dir=" + dir,
		"--period=today",
	}, &out, os.Stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// The fixture dates don't match "today" so it should just produce the
	// empty-state markdown without error.
	if !strings.Contains(out.String(), "振り返り") {
		t.Errorf("missing heading")
	}
}

func TestRunMissingClaudeDir(t *testing.T) {
	dir := t.TempDir() // no projects/ subdir
	err := run([]string{"--claude-dir=" + dir}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Error("expected error for missing projects dir")
	}
}

func TestRunInvalidFormat(t *testing.T) {
	dir := setupFakeClaude(t)
	err := run([]string{
		"--claude-dir=" + dir,
		"--from=2026-04-01",
		"--to=2026-04-30",
		"--format=xml",
	}, &bytes.Buffer{}, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown --format") {
		t.Errorf("err = %v", err)
	}
}

func TestRunInvalidPeriod(t *testing.T) {
	err := run([]string{"--period=nope"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Error("expected error for bad period")
	}
}

func TestRunInvalidFromDate(t *testing.T) {
	err := run([]string{"--from=not-a-date"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Error("expected error for bad from date")
	}
}

func TestRunMaxSessions(t *testing.T) {
	dir := setupFakeClaude(t)
	var out bytes.Buffer
	err := run([]string{
		"--claude-dir=" + dir,
		"--from=2026-04-01",
		"--to=2026-04-30",
		"--max-sessions=0", // unlimited
	}, &out, os.Stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}
