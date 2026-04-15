package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mikanfactory/argentina/nippo/internal/collector"
)

func fixtureSessions() []collector.RawSession {
	return []collector.RawSession{
		{
			SessionID:   "s1",
			Project:     "project-a",
			ProjectPath: "/home/alice/project-a",
			GitBranch:   "main",
			UserEntries: []collector.ParsedUserEntry{
				{Timestamp: "2026-04-05T10:00:00Z", Text: "tokio を使う"},
				{Timestamp: "2026-04-05T10:05:00Z", Text: "別のやり方を試したい"},
				{Timestamp: "2026-04-05T10:10:00Z", Text: "instead of mutex"},
			},
			AssistantEntries: []collector.ParsedAssistantEntry{
				{
					Timestamp:    "2026-04-05T10:02:00Z",
					ToolUses:     []string{"Read", "Grep", "Read"},
					FilePaths:    []string{"/tmp/a.rs"},
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
		},
		{
			SessionID:   "s2",
			Project:     "project-b",
			ProjectPath: "/home/alice/project-b",
			UserEntries: []collector.ParsedUserEntry{
				{Timestamp: "2026-04-06T09:00:00Z", Text: "hello"},
			},
			AssistantEntries: []collector.ParsedAssistantEntry{
				{Timestamp: "2026-04-06T09:01:00Z", ToolUses: []string{"Bash"}, InputTokens: 10, OutputTokens: 5},
			},
		},
	}
}

func TestAggregate(t *testing.T) {
	stats := Aggregate(fixtureSessions())

	if stats.TotalUserMessages != 4 {
		t.Errorf("user msgs = %d", stats.TotalUserMessages)
	}
	if stats.TotalAssistantMsgs != 2 {
		t.Errorf("asst msgs = %d", stats.TotalAssistantMsgs)
	}
	if stats.TotalToolUses != 4 {
		t.Errorf("tool uses = %d", stats.TotalToolUses)
	}
	if stats.TotalInputTokens != 110 || stats.TotalOutputTokens != 55 {
		t.Errorf("tokens: %d/%d", stats.TotalInputTokens, stats.TotalOutputTokens)
	}
	if stats.ToolFrequency["Read"] != 2 {
		t.Errorf("Read freq = %d", stats.ToolFrequency["Read"])
	}
	if len(stats.Projects) != 2 {
		t.Errorf("projects = %d", len(stats.Projects))
	}
	if stats.Projects[0].Name != "project-a" {
		t.Errorf("top project = %q, want project-a", stats.Projects[0].Name)
	}
	// Both JA ("を使う") and EN ("instead") decisions detected.
	if len(stats.Decisions) < 2 {
		t.Errorf("decisions = %d, want ≥2", len(stats.Decisions))
	}
	if stats.AvgPromptLength == 0 {
		t.Error("avg prompt length zero")
	}
	if stats.OverallFrom == "" || stats.OverallTo == "" {
		t.Error("overall range missing")
	}
}

func TestTopTools(t *testing.T) {
	stats := Aggregate(fixtureSessions())
	top := stats.TopTools(2)
	if len(top) != 2 {
		t.Fatalf("top = %d", len(top))
	}
	if top[0].Name != "Read" {
		t.Errorf("top tool = %q", top[0].Name)
	}
	// n <= 0 returns all
	if all := stats.TopTools(0); len(all) != 3 {
		t.Errorf("all tools = %d", len(all))
	}
}

func TestMarkdown(t *testing.T) {
	sessions := fixtureSessions()
	stats := Aggregate(sessions)
	md := Markdown("last-week", sessions, stats)

	wants := []string{
		"# 振り返り: last-week",
		"## 事実",
		"セッション数: **2**",
		"project-a",
		"Read × 2",
		"時間帯ヒートマップ",
		"意思決定の気配",
		"ALACT",
	}
	for _, w := range wants {
		if !strings.Contains(md, w) {
			t.Errorf("missing %q in output", w)
		}
	}
}

func TestMarkdownEmpty(t *testing.T) {
	md := Markdown("today", nil, AggregateStats{})
	if !strings.Contains(md, "記録されたセッションはありません") {
		t.Error("empty message missing")
	}
	if !strings.Contains(md, "ALACT") {
		t.Error("ALACT prompts should still appear on empty days")
	}
}

func TestRenderJSON(t *testing.T) {
	sessions := fixtureSessions()
	stats := Aggregate(sessions)
	data, err := RenderJSON("last-week", sessions, stats)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var out JSONOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if out.Meta.FilterLabel != "last-week" {
		t.Errorf("label = %q", out.Meta.FilterLabel)
	}
	if out.Meta.TotalSessions != 2 {
		t.Errorf("total sessions = %d", out.Meta.TotalSessions)
	}
	if out.Stats.ToolFrequency["Read"] != 2 {
		t.Errorf("tool freq lost")
	}
	if len(out.Sessions) != 2 {
		t.Errorf("sessions lost")
	}
}

func TestIsDecision(t *testing.T) {
	cases := map[string]bool{
		"tokio を使う":        true,
		"instead of that":   true,
		"hello world":       false,
		"やっぱりやめる":          true,
		"Actually, no":      true,
	}
	for text, want := range cases {
		if got := isDecision(text); got != want {
			t.Errorf("isDecision(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestSingleLine(t *testing.T) {
	in := "line1\nline2\r\nline3"
	if got := singleLine(in); strings.Contains(got, "\n") {
		t.Errorf("newline not stripped: %q", got)
	}
}
