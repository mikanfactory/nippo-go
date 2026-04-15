package report

import (
	"encoding/json"

	"github.com/mikanfactory/argentina/nippo/internal/collector"
)

// JSON renders the same information as Markdown in a stable JSON shape so
// downstream tools (dashboards, scripts) can consume it.
type JSONOutput struct {
	Meta     JSONMeta                 `json:"meta"`
	Sessions []JSONSession            `json:"sessions"`
	Stats    JSONStats                `json:"stats"`
}

type JSONMeta struct {
	FilterLabel   string `json:"filter_label"`
	TotalSessions int    `json:"total_sessions"`
}

type JSONSession struct {
	SessionID    string   `json:"session_id"`
	Project      string   `json:"project"`
	ProjectPath  string   `json:"project_path"`
	GitBranch    string   `json:"git_branch,omitempty"`
	UserPrompts  []string `json:"user_prompts"`
	ToolUses     []string `json:"tool_uses"`
	InputTokens  uint64   `json:"input_tokens"`
	OutputTokens uint64   `json:"output_tokens"`
}

type JSONStats struct {
	ToolFrequency      map[string]int `json:"tool_frequency"`
	TotalToolUses      int            `json:"total_tool_uses"`
	TotalInputTokens   uint64         `json:"total_input_tokens"`
	TotalOutputTokens  uint64         `json:"total_output_tokens"`
	TotalUserMessages  int            `json:"total_user_messages"`
	TotalAssistantMsgs int            `json:"total_assistant_msgs"`
	Projects           []ProjectStat  `json:"projects"`
	Decisions          []Decision     `json:"decisions"`
}

// RenderJSON serializes sessions + stats as indented JSON.
func RenderJSON(label string, sessions []collector.RawSession, stats AggregateStats) ([]byte, error) {
	out := JSONOutput{
		Meta: JSONMeta{
			FilterLabel:   label,
			TotalSessions: len(sessions),
		},
		Sessions: make([]JSONSession, 0, len(sessions)),
		Stats: JSONStats{
			ToolFrequency:      stats.ToolFrequency,
			TotalToolUses:      stats.TotalToolUses,
			TotalInputTokens:   stats.TotalInputTokens,
			TotalOutputTokens:  stats.TotalOutputTokens,
			TotalUserMessages:  stats.TotalUserMessages,
			TotalAssistantMsgs: stats.TotalAssistantMsgs,
			Projects:           stats.Projects,
			Decisions:          stats.Decisions,
		},
	}
	for _, s := range sessions {
		js := JSONSession{
			SessionID:   s.SessionID,
			Project:     s.Project,
			ProjectPath: s.ProjectPath,
			GitBranch:   s.GitBranch,
		}
		for _, u := range s.UserEntries {
			js.UserPrompts = append(js.UserPrompts, u.Text)
		}
		for _, a := range s.AssistantEntries {
			js.ToolUses = append(js.ToolUses, a.ToolUses...)
			js.InputTokens += a.InputTokens
			js.OutputTokens += a.OutputTokens
		}
		out.Sessions = append(out.Sessions, js)
	}
	return json.MarshalIndent(out, "", "  ")
}
