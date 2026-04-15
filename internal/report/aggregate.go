// Package report turns collected sessions into aggregate stats and the
// final reflection Markdown (or JSON) output.
package report

import (
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mikanfactory/argentina/nippo/internal/collector"
)

// AggregateStats summarizes a slice of sessions.
type AggregateStats struct {
	Projects           []ProjectStat
	TotalUserMessages  int
	TotalAssistantMsgs int
	TotalToolUses      int
	ToolFrequency      map[string]int
	TotalInputTokens   uint64
	TotalOutputTokens  uint64
	SessionsByHour     map[int]int // local-time hour (0-23) -> sessions active in that hour
	OverallFrom        string
	OverallTo          string
	Decisions          []Decision
	DecisionsByProject []RankedItem
	PromptCount        int
	AvgPromptLength    int
}

// ProjectStat describes one project's slice of the aggregate.
type ProjectStat struct {
	Name         string
	SessionCount int
	MessageCount int
	ToolUsage    map[string]int
	FilesTouched []string
}

// RankedItem is a (name, count) pair used for top-N rankings such as
// "most used tools" or "projects with the most decisions".
type RankedItem struct {
	Name  string
	Count int
}

// Decision is a user turn that contains a decision signal word.
type Decision struct {
	Timestamp string
	Project   string
	Context   string // short excerpt (≤80 chars)
	Prompt    string
}

// Japanese / English decision signal words, ported from the original Rust.
var (
	decisionSignalsJA = []string{
		"にする", "を選ぶ", "の方がいい", "ではなく", "より", "じゃなくて",
		"そうじゃなくて", "いや、", "やっぱり", "を使う", "に変える", "にして",
		"に変更", "のほうが",
	}
	decisionSignalsEN = []string{
		"instead", "rather than", "go with", "let's use", "prefer",
		"switch to", "change to", "not that", "actually,", "no,",
	}
)

// Aggregate computes stats over a slice of sessions. Local time is derived
// from the current process location so the hourly histogram reflects the
// user's wall clock, not UTC.
func Aggregate(sessions []collector.RawSession) AggregateStats {
	stats := AggregateStats{
		ToolFrequency:  map[string]int{},
		SessionsByHour: map[int]int{},
	}
	projectMap := map[string]*ProjectStat{}
	projectSeen := map[string]map[string]bool{}
	decisionCount := map[string]int{}

	var promptLenSum int
	var earliest, latest time.Time

	for _, s := range sessions {
		stats.TotalUserMessages += len(s.UserEntries)
		stats.TotalAssistantMsgs += len(s.AssistantEntries)

		ps, ok := projectMap[s.Project]
		if !ok {
			ps = &ProjectStat{Name: s.Project, ToolUsage: map[string]int{}}
			projectMap[s.Project] = ps
			projectSeen[s.Project] = map[string]bool{}
		}
		ps.SessionCount++
		ps.MessageCount += len(s.UserEntries) + len(s.AssistantEntries)

		for _, u := range s.UserEntries {
			stats.PromptCount++
			promptLenSum += utf8.RuneCountInString(u.Text)
			if isDecision(u.Text) {
				ctx := u.Text
				if utf8.RuneCountInString(u.Text) > 80 {
					ctx = string([]rune(u.Text)[:80])
				}
				stats.Decisions = append(stats.Decisions, Decision{
					Timestamp: u.Timestamp,
					Project:   s.Project,
					Context:   ctx,
					Prompt:    u.Text,
				})
				decisionCount[s.Project]++
			}
			updateRange(&earliest, &latest, u.Timestamp)
		}

		// Tool usage + file touches + tokens
		seenFiles := projectSeen[s.Project]
		sessionHours := map[int]bool{}
		for _, a := range s.AssistantEntries {
			for _, tool := range a.ToolUses {
				stats.ToolFrequency[tool]++
				ps.ToolUsage[tool]++
				stats.TotalToolUses++
			}
			for _, fp := range a.FilePaths {
				if !seenFiles[fp] {
					seenFiles[fp] = true
					ps.FilesTouched = append(ps.FilesTouched, fp)
				}
			}
			stats.TotalInputTokens += a.InputTokens
			stats.TotalOutputTokens += a.OutputTokens
			if t, err := time.Parse(time.RFC3339, a.Timestamp); err == nil {
				sessionHours[t.Local().Hour()] = true
				updateRange(&earliest, &latest, a.Timestamp)
			}
		}
		for h := range sessionHours {
			stats.SessionsByHour[h]++
		}
	}

	if stats.PromptCount > 0 {
		stats.AvgPromptLength = promptLenSum / stats.PromptCount
	}
	if !earliest.IsZero() {
		stats.OverallFrom = earliest.Format(time.RFC3339)
	}
	if !latest.IsZero() {
		stats.OverallTo = latest.Format(time.RFC3339)
	}

	stats.Projects = make([]ProjectStat, 0, len(projectMap))
	for _, ps := range projectMap {
		stats.Projects = append(stats.Projects, *ps)
	}
	sort.Slice(stats.Projects, func(i, j int) bool {
		return stats.Projects[i].SessionCount > stats.Projects[j].SessionCount
	})

	stats.DecisionsByProject = make([]RankedItem, 0, len(decisionCount))
	for p, c := range decisionCount {
		stats.DecisionsByProject = append(stats.DecisionsByProject, RankedItem{Name: p, Count: c})
	}
	sort.Slice(stats.DecisionsByProject, func(i, j int) bool {
		return stats.DecisionsByProject[i].Count > stats.DecisionsByProject[j].Count
	})

	return stats
}

func isDecision(text string) bool {
	for _, s := range decisionSignalsJA {
		if strings.Contains(text, s) {
			return true
		}
	}
	lower := strings.ToLower(text)
	for _, s := range decisionSignalsEN {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func updateRange(earliest, latest *time.Time, ts string) {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return
	}
	if earliest.IsZero() || t.Before(*earliest) {
		*earliest = t
	}
	if latest.IsZero() || t.After(*latest) {
		*latest = t
	}
}

// TopTools returns the top n tool names by frequency, descending.
func (s AggregateStats) TopTools(n int) []RankedItem {
	items := make([]RankedItem, 0, len(s.ToolFrequency))
	for k, v := range s.ToolFrequency {
		items = append(items, RankedItem{Name: k, Count: v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	if n > 0 && len(items) > n {
		items = items[:n]
	}
	return items
}
