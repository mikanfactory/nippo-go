// Package collector reads Claude Code JSONL session files and extracts
// structured session summaries suitable for aggregation.
package collector

import (
	"encoding/json"
	"time"
)

// MaxPromptLen limits how many characters of any single user prompt are kept
// in the in-memory session. Longer prompts are truncated with an ellipsis.
const MaxPromptLen = 500

// entryHeader is the minimum shape needed for the first parse pass:
// we only need `type` and `timestamp` to decide whether to keep a line.
type entryHeader struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
}

// rawUserEntry is the full shape of a `type: "user"` line.
type rawUserEntry struct {
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	GitBranch string `json:"gitBranch"`
	Message   struct {
		// Content is either a plain string or an array of ContentBlock.
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// rawAssistantEntry is the full shape of a `type: "assistant"` line.
type rawAssistantEntry struct {
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	GitBranch string `json:"gitBranch"`
	Message   struct {
		Content []contentBlock `json:"content"`
		Usage   tokenUsage     `json:"usage"`
	} `json:"message"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type tokenUsage struct {
	InputTokens  uint64 `json:"input_tokens"`
	OutputTokens uint64 `json:"output_tokens"`
}

// RawSession is the parsed, filtered contents of one JSONL session file.
// A session file may contain entries from multiple working directories; we
// take the CWD/branch from the first entry that carries them.
type RawSession struct {
	SessionID   string
	Project     string // last path component of CWD
	ProjectPath string // full CWD
	GitBranch   string
	UserEntries      []ParsedUserEntry
	AssistantEntries []ParsedAssistantEntry
}

// ParsedUserEntry is a single user prompt, time-stamped and truncated.
type ParsedUserEntry struct {
	Timestamp string
	Text      string
}

// ParsedAssistantEntry is the effect of one assistant turn.
type ParsedAssistantEntry struct {
	Timestamp    string
	ToolUses     []string
	InputTokens  uint64
	OutputTokens uint64
	FilePaths    []string
}

// SessionFile is a discovered JSONL file with its modification time, used
// for mtime pre-filtering before parsing.
type SessionFile struct {
	Path  string
	Mtime time.Time
}
