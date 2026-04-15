package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/mikanfactory/argentina/nippo/internal/filter"
)

// fileOpsTools is the set of tool names whose input file_path/path fields
// we record as "files touched" in the reflection summary.
var fileOpsTools = map[string]bool{
	"Read":  true,
	"Write": true,
	"Edit":  true,
	"Glob":  true,
	"Grep":  true,
}

// Options tunes CollectSessions beyond what the date filter covers.
// Zero values mean "no extra filtering".
type Options struct {
	// Project, if non-empty, restricts sessions to those whose project name
	// (CWD basename) contains this substring (case-insensitive).
	Project string
}

// CollectSessions scans all discovered JSONL files in parallel and returns
// the ones that contain at least one entry passing the filter. Files whose
// mtime is older than the filter's `From` bound are skipped unread, and
// (when opts.Project is set) files whose encoded directory name doesn't
// contain the needle are skipped before any reading.
func CollectSessions(ctx context.Context, claudeDir string, f filter.DateFilter, opts Options) ([]RawSession, error) {
	files, err := DiscoverSessionFiles(claudeDir)
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(opts.Project)
	relevant := files[:0]
	for _, sf := range files {
		if !f.FileRelevant(sf.Mtime) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(filepath.Base(filepath.Dir(sf.Path))), needle) {
			continue
		}
		relevant = append(relevant, sf)
	}

	results := make([]*RawSession, len(relevant))
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for i := range relevant {
		g.Go(func() error {
			session, perr := ParseSessionFile(relevant[i].Path, f)
			if perr != nil {
				return perr
			}
			results[i] = session
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	sessions := make([]RawSession, 0, len(results))
	for _, s := range results {
		if s == nil {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(s.Project), needle) {
			continue
		}
		sessions = append(sessions, *s)
	}
	return sessions, nil
}

// ParseSessionFile streams a single JSONL file and returns a RawSession if
// any user or assistant entries pass the filter. Lines that don't decode are
// skipped silently (rather than aborting the whole file).
func ParseSessionFile(path string, f filter.DateFilter) (*RawSession, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	session := &RawSession{}

	scanner := bufio.NewScanner(file)
	// JSONL lines can be long (tool inputs with large payloads).
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var header entryHeader
		if err := json.Unmarshal(line, &header); err != nil {
			continue
		}
		if header.Type != "user" && header.Type != "assistant" {
			continue
		}
		if !f.Matches(header.Timestamp) {
			continue
		}

		switch header.Type {
		case "user":
			var u rawUserEntry
			if err := json.Unmarshal(line, &u); err != nil {
				continue
			}
			applyMeta(session, u.SessionID, u.CWD, u.GitBranch)
			if text := extractUserText(u.Message.Content); text != "" {
				session.UserEntries = append(session.UserEntries, ParsedUserEntry{
					Timestamp: u.Timestamp,
					Text:      text,
				})
			}
		case "assistant":
			var a rawAssistantEntry
			if err := json.Unmarshal(line, &a); err != nil {
				continue
			}
			applyMeta(session, a.SessionID, a.CWD, a.GitBranch)
			tools, paths := extractToolInfo(a.Message.Content)
			session.AssistantEntries = append(session.AssistantEntries, ParsedAssistantEntry{
				Timestamp:    a.Timestamp,
				ToolUses:     tools,
				InputTokens:  a.Message.Usage.InputTokens,
				OutputTokens: a.Message.Usage.OutputTokens,
				FilePaths:    paths,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	if len(session.UserEntries) == 0 && len(session.AssistantEntries) == 0 {
		return nil, nil
	}
	return session, nil
}

// applyMeta fills the project/branch fields the first time a non-empty value
// is seen; subsequent differences are ignored since all entries in one file
// usually share the same session.
func applyMeta(s *RawSession, sessionID, cwd, branch string) {
	if s.SessionID == "" && sessionID != "" {
		s.SessionID = sessionID
	}
	if s.ProjectPath == "" && cwd != "" {
		s.ProjectPath = cwd
		s.Project = filepath.Base(cwd)
	}
	if s.GitBranch == "" && branch != "" {
		s.GitBranch = branch
	}
}

// extractUserText returns the human-readable text of a user message, or "".
// The message.content field may be a plain string or an array of blocks
// (only text blocks contribute; tool_result blocks are ignored).
func extractUserText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return TruncateRunes(strings.TrimSpace(s), MaxPromptLen)
	}
	// Then array of blocks.
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return TruncateRunes(strings.TrimSpace(strings.Join(parts, "\n")), MaxPromptLen)
}

// extractToolInfo walks assistant content blocks, collects tool names, and
// pulls out file_path / path arguments from file-ops tools.
func extractToolInfo(blocks []contentBlock) (tools, files []string) {
	for _, b := range blocks {
		if b.Type != "tool_use" || b.Name == "" {
			continue
		}
		tools = append(tools, b.Name)
		if !fileOpsTools[b.Name] || len(b.Input) == 0 {
			continue
		}
		var args map[string]any
		if err := json.Unmarshal(b.Input, &args); err != nil {
			continue
		}
		if fp, ok := args["file_path"].(string); ok && fp != "" {
			files = append(files, fp)
		}
		if fp, ok := args["path"].(string); ok && fp != "" {
			files = append(files, fp)
		}
	}
	return tools, files
}

// TruncateRunes clips s to max runes, appending "..." when truncation occurs.
// Single rune conversion, so callers can use it on hot paths without fearing
// the double [rune] allocation a naive len-check implementation would make.
func TruncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
