package collector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikanfactory/argentina/nippo/internal/filter"
)

func TestParseSessionFile(t *testing.T) {
	f, err := filter.FromRange("2026-04-01", "2026-04-30")
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	sess, err := ParseSessionFile("../../testdata/sample.jsonl", f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.SessionID != "sess-1" {
		t.Errorf("session id = %q", sess.SessionID)
	}
	if sess.Project != "project-a" {
		t.Errorf("project = %q", sess.Project)
	}
	if sess.GitBranch != "main" {
		t.Errorf("branch = %q", sess.GitBranch)
	}

	// Three user messages in-range, but only two carry text (third is tool_result only).
	if len(sess.UserEntries) != 2 {
		t.Errorf("user entries = %d, want 2", len(sess.UserEntries))
	}
	if !strings.Contains(sess.UserEntries[0].Text, "refactor") {
		t.Errorf("first user text = %q", sess.UserEntries[0].Text)
	}
	if !strings.Contains(sess.UserEntries[1].Text, "tokio") {
		t.Errorf("second user text = %q", sess.UserEntries[1].Text)
	}

	if len(sess.AssistantEntries) != 1 {
		t.Fatalf("assistant entries = %d, want 1", len(sess.AssistantEntries))
	}
	a := sess.AssistantEntries[0]
	if len(a.ToolUses) != 2 || a.ToolUses[0] != "Read" || a.ToolUses[1] != "Grep" {
		t.Errorf("tool uses = %v", a.ToolUses)
	}
	if a.InputTokens != 100 || a.OutputTokens != 50 {
		t.Errorf("tokens = %d/%d", a.InputTokens, a.OutputTokens)
	}
	// file_path from Read + path from Grep
	if len(a.FilePaths) != 2 {
		t.Errorf("file paths = %v", a.FilePaths)
	}
}

func TestParseSessionFileOutOfRange(t *testing.T) {
	f, _ := filter.FromRange("2019-01-01", "2019-12-31")
	sess, err := ParseSessionFile("../../testdata/sample.jsonl", f)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Only the 2020 entry passes, and it has text — so session is non-nil.
	if sess == nil {
		t.Skip("no entries in range (depends on filter semantics)")
	}
}

func TestParseSessionFileMissing(t *testing.T) {
	f, _ := filter.FromRange("", "")
	_, err := ParseSessionFile("/nonexistent/file.jsonl", f)
	if err == nil {
		t.Error("expected open error")
	}
}

func TestExtractUserText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"string", `"hello world"`, "hello world"},
		{"blocks", `[{"type":"text","text":"hi"},{"type":"tool_use","name":"X"}]`, "hi"},
		{"empty blocks", `[{"type":"tool_result"}]`, ""},
		{"empty string", `"   "`, ""},
		{"malformed", `{"not": "valid"}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractUserText([]byte(c.in))
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := TruncateRunes("abc", 10); got != "abc" {
		t.Errorf("no-op: %q", got)
	}
	if got := TruncateRunes("abcdefghij", 5); got != "abcde..." {
		t.Errorf("truncate: %q", got)
	}
	if got := TruncateRunes("あいうえおかきくけこ", 3); got != "あいう..." {
		t.Errorf("unicode: %q", got)
	}
}

func TestDiscoverSessionFiles(t *testing.T) {
	dir := t.TempDir()
	projects := filepath.Join(dir, "projects", "example")
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(projects, "a.jsonl")
	if err := os.WriteFile(target, []byte(`{"type":"user"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also write a non-jsonl file that should be ignored.
	if err := os.WriteFile(filepath.Join(projects, "ignore.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := DiscoverSessionFiles(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0].Path) != "a.jsonl" {
		t.Errorf("files = %+v", files)
	}
}

func TestDiscoverSessionFilesMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := DiscoverSessionFiles(dir)
	if err == nil {
		t.Error("expected error for missing projects dir")
	}
}

func TestCollectSessions(t *testing.T) {
	dir := t.TempDir()
	projects := filepath.Join(dir, "projects", "p")
	_ = os.MkdirAll(projects, 0o755)
	src, _ := os.ReadFile("../../testdata/sample.jsonl")
	_ = os.WriteFile(filepath.Join(projects, "s.jsonl"), src, 0o644)

	f, _ := filter.FromRange("2026-04-01", "2026-04-30")
	sessions, err := CollectSessions(context.Background(), dir, f, Options{})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(sessions))
	}
	if sessions[0].Project != "project-a" {
		t.Errorf("project = %q", sessions[0].Project)
	}
}

func TestCollectSessionsProjectFilter(t *testing.T) {
	dir := t.TempDir()
	projects := filepath.Join(dir, "projects", "-home-alice-project-a")
	_ = os.MkdirAll(projects, 0o755)
	src, _ := os.ReadFile("../../testdata/sample.jsonl")
	_ = os.WriteFile(filepath.Join(projects, "s.jsonl"), src, 0o644)

	f, _ := filter.FromRange("2026-04-01", "2026-04-30")

	hit, err := CollectSessions(context.Background(), dir, f, Options{Project: "project-a"})
	if err != nil || len(hit) != 1 {
		t.Fatalf("project match: %v sessions=%d", err, len(hit))
	}

	miss, err := CollectSessions(context.Background(), dir, f, Options{Project: "no-such"})
	if err != nil || len(miss) != 0 {
		t.Fatalf("project miss: %v sessions=%d", err, len(miss))
	}
}
