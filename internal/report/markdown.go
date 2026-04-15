package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mikanfactory/argentina/nippo/internal/collector"
)

const (
	maxPromptsPerProject = 5
	promptLineMaxRunes   = 120
)

var newlineReplacer = strings.NewReplacer("\n", " ", "\r", " ")

// Markdown renders sessions + stats as a reflection-oriented Markdown document.
// The "facts" are machine generated; the ALACT prompts are left blank for the
// user to fill in.
func Markdown(label string, sessions []collector.RawSession, stats AggregateStats) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# 振り返り: %s\n\n", label)

	if len(sessions) == 0 {
		b.WriteString("> この期間に記録されたセッションはありません。\n\n")
		b.WriteString(alactPrompts)
		return b.String()
	}

	b.WriteString("## 事実（機械生成）\n\n")
	fmt.Fprintf(&b, "- セッション数: **%d**\n", len(sessions))
	fmt.Fprintf(&b, "- ユーザー発言: %d / アシスタント応答: %d\n",
		stats.TotalUserMessages, stats.TotalAssistantMsgs)
	fmt.Fprintf(&b, "- ツール呼び出し回数: %d\n", stats.TotalToolUses)
	fmt.Fprintf(&b, "- トークン: 入力 %d / 出力 %d\n",
		stats.TotalInputTokens, stats.TotalOutputTokens)
	if stats.OverallFrom != "" {
		fmt.Fprintf(&b, "- 期間: %s 〜 %s\n", stats.OverallFrom, stats.OverallTo)
	}
	if stats.AvgPromptLength > 0 {
		fmt.Fprintf(&b, "- 平均プロンプト長: %d 文字（%d 件）\n",
			stats.AvgPromptLength, stats.PromptCount)
	}
	b.WriteString("\n")

	if len(stats.Projects) > 0 {
		b.WriteString("### プロジェクト別\n\n")
		b.WriteString("| プロジェクト | セッション | メッセージ |\n")
		b.WriteString("|---|---:|---:|\n")
		for _, p := range stats.Projects {
			fmt.Fprintf(&b, "| %s | %d | %d |\n", p.Name, p.SessionCount, p.MessageCount)
		}
		b.WriteString("\n")
	}

	if top := stats.TopTools(10); len(top) > 0 {
		b.WriteString("### よく使ったツール (top 10)\n\n")
		for _, t := range top {
			fmt.Fprintf(&b, "- %s × %d\n", t.Name, t.Count)
		}
		b.WriteString("\n")
	}

	if len(stats.SessionsByHour) > 0 {
		b.WriteString("### 時間帯ヒートマップ（ローカル時刻）\n\n")
		b.WriteString("```\n")
		maxCount := 0
		for _, c := range stats.SessionsByHour {
			if c > maxCount {
				maxCount = c
			}
		}
		for h := 0; h < 24; h++ {
			c := stats.SessionsByHour[h]
			bar := ""
			if maxCount > 0 {
				barLen := c * 20 / maxCount
				bar = strings.Repeat("█", barLen)
			}
			fmt.Fprintf(&b, "%02d  %s %d\n", h, bar, c)
		}
		b.WriteString("```\n\n")
	}

	if len(stats.Decisions) > 0 {
		b.WriteString("### 意思決定の気配（シグナル語を含む発言）\n\n")
		limit := len(stats.Decisions)
		if limit > 10 {
			limit = 10
		}
		for _, d := range stats.Decisions[:limit] {
			fmt.Fprintf(&b, "- `%s` [%s] %s\n", d.Timestamp, d.Project, d.Context)
		}
		if len(stats.Decisions) > 10 {
			fmt.Fprintf(&b, "- …他 %d 件\n", len(stats.Decisions)-10)
		}
		b.WriteString("\n")
	}

	// Top prompts (up to 5 per project) help ground the reflection.
	b.WriteString("### 印象的なプロンプト（各プロジェクト上位5件）\n\n")
	byProject := groupPromptsByProject(sessions)
	names := make([]string, 0, len(byProject))
	for k := range byProject {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "#### %s\n\n", name)
		prompts := byProject[name]
		if len(prompts) > maxPromptsPerProject {
			prompts = prompts[:maxPromptsPerProject]
		}
		for _, p := range prompts {
			fmt.Fprintf(&b, "- %s\n", singleLine(p))
		}
		b.WriteString("\n")
	}

	b.WriteString(alactPrompts)
	return b.String()
}

func groupPromptsByProject(sessions []collector.RawSession) map[string][]string {
	out := map[string][]string{}
	for _, s := range sessions {
		for _, u := range s.UserEntries {
			out[s.Project] = append(out[s.Project], u.Text)
		}
	}
	return out
}

func singleLine(s string) string {
	return collector.TruncateRunes(newlineReplacer.Replace(s), promptLineMaxRunes)
}
