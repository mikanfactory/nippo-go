// Command nippo scans Claude Code JSONL session logs and prints a
// reflection-oriented Markdown (or JSON) summary for a chosen period.
//
// Usage:
//
//	nippo [flags]
//
// Flags:
//
//	--days N              過去N日 (default: 1, 0 = 全期間)
//	--from YYYY-MM-DD     開始日 (--days より優先)
//	--to   YYYY-MM-DD     終了日
//	--period NAME         today | yesterday | this-week | last-week | this-month | last-month
//	--project SUBSTR      プロジェクト名の部分一致フィルタ
//	--format FMT          markdown | json (default: markdown)
//	--claude-dir PATH     default: $HOME/.claude
//	--max-sessions N      出力セッション数の上限 (0 = 無制限)
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mikanfactory/argentina/nippo/internal/collector"
	"github.com/mikanfactory/argentina/nippo/internal/filter"
	"github.com/mikanfactory/argentina/nippo/internal/report"
)

const (
	formatMarkdown = "markdown"
	formatJSON     = "json"
)

type cliOptions struct {
	days        uint
	from        string
	to          string
	period      string
	project     string
	format      string
	claudeDir   string
	maxSessions int
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return err
	}

	f, err := buildFilter(opts)
	if err != nil {
		return err
	}

	sessions, err := collector.CollectSessions(
		context.Background(), opts.claudeDir, f,
		collector.Options{Project: opts.project},
	)
	if err != nil {
		return err
	}

	if opts.maxSessions > 0 && len(sessions) > opts.maxSessions {
		sessions = sessions[:opts.maxSessions]
	}

	stats := report.Aggregate(sessions)

	switch opts.format {
	case formatMarkdown, "":
		_, err := io.WriteString(stdout, report.Markdown(f.Label(), sessions, stats))
		return err
	case formatJSON:
		data, err := report.RenderJSON(f.Label(), sessions, stats)
		if err != nil {
			return err
		}
		_, werr := stdout.Write(append(data, '\n'))
		return werr
	default:
		return fmt.Errorf("unknown --format %q (want markdown|json)", opts.format)
	}
}

func parseFlags(args []string, stderr io.Writer) (cliOptions, error) {
	fs := flag.NewFlagSet("nippo", flag.ContinueOnError)
	fs.SetOutput(stderr)

	home, _ := os.UserHomeDir()
	opts := cliOptions{}
	fs.UintVar(&opts.days, "days", 1, "look-back window in days (0 = all time)")
	fs.StringVar(&opts.from, "from", "", "start date YYYY-MM-DD (overrides --days)")
	fs.StringVar(&opts.to, "to", "", "end date YYYY-MM-DD")
	fs.StringVar(&opts.period, "period", "", "named period: today, yesterday, this-week, last-week, this-month, last-month")
	fs.StringVar(&opts.project, "project", "", "filter sessions by project-name substring")
	fs.StringVar(&opts.format, "format", formatMarkdown, "output format: markdown or json")
	fs.StringVar(&opts.claudeDir, "claude-dir", filepath.Join(home, ".claude"), "Claude Code data directory")
	fs.IntVar(&opts.maxSessions, "max-sessions", 0, "cap on number of sessions in output (0 = unlimited)")

	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	return opts, nil
}

func buildFilter(opts cliOptions) (filter.DateFilter, error) {
	switch {
	case opts.period != "":
		return filter.FromPeriod(opts.period)
	case opts.from != "" || opts.to != "":
		return filter.FromRange(opts.from, opts.to)
	default:
		return filter.FromDays(opts.days), nil
	}
}
