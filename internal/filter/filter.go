// Package filter provides date-range filtering for Claude Code session entries.
package filter

import (
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// DateFilter restricts entries by timestamp range. Nil bounds mean "unbounded".
type DateFilter struct {
	From *time.Time
	To   *time.Time
	// label overrides the auto-derived label when set (e.g. "today", "last week").
	label string
}

// FromDays returns a filter for "the last N days, ending now" in UTC.
// days == 0 means "all time" (no bounds).
func FromDays(days uint) DateFilter {
	return FromDaysAt(days, time.Now().UTC())
}

// FromDaysAt is FromDays with an injectable "now" for tests.
func FromDaysAt(days uint, now time.Time) DateFilter {
	if days == 0 {
		return DateFilter{label: "all time"}
	}
	start := now.AddDate(0, 0, -int(days))
	from := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	label := fmt.Sprintf("%d days", days)
	if days == 1 {
		label = "today"
	}
	return DateFilter{From: &from, label: label}
}

// FromRange parses explicit YYYY-MM-DD dates. Empty strings mean "no bound".
func FromRange(from, to string) (DateFilter, error) {
	f := DateFilter{}
	if from != "" {
		t, err := time.ParseInLocation(dateLayout, from, time.UTC)
		if err != nil {
			return f, fmt.Errorf("invalid --from date %q: %w", from, err)
		}
		f.From = &t
	}
	if to != "" {
		t, err := time.ParseInLocation(dateLayout, to, time.UTC)
		if err != nil {
			return f, fmt.Errorf("invalid --to date %q: %w", to, err)
		}
		end := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
		f.To = &end
	}
	f.label = rangeLabel(from, to)
	return f, nil
}

func rangeLabel(from, to string) string {
	if from == "" && to == "" {
		return "all time"
	}
	if from == "" {
		return "... ~ " + to
	}
	if to == "" {
		return from + " ~ today"
	}
	return from + " ~ " + to
}

// FromPeriod returns a filter for a named period such as "today" or "last-week".
func FromPeriod(name string) (DateFilter, error) {
	return FromPeriodAt(name, time.Now().UTC())
}

// FromPeriodAt is FromPeriod with an injectable "now".
func FromPeriodAt(name string, now time.Time) (DateFilter, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	var fromDate, toDate time.Time
	switch name {
	case "today":
		fromDate, toDate = today, today
	case "yesterday":
		d := today.AddDate(0, 0, -1)
		fromDate, toDate = d, d
	case "this-week":
		fromDate, toDate = mondayOf(today), today
	case "last-week":
		thisMon := mondayOf(today)
		fromDate = thisMon.AddDate(0, 0, -7)
		toDate = thisMon.AddDate(0, 0, -1)
	case "this-month":
		fromDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		toDate = today
	case "last-month":
		firstThis := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		lastPrev := firstThis.AddDate(0, 0, -1)
		fromDate = time.Date(lastPrev.Year(), lastPrev.Month(), 1, 0, 0, 0, 0, time.UTC)
		toDate = lastPrev
	default:
		return DateFilter{}, fmt.Errorf("unknown period %q (valid: today, yesterday, this-week, last-week, this-month, last-month)", name)
	}

	from := fromDate
	to := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 23, 59, 59, 0, time.UTC)
	return DateFilter{From: &from, To: &to, label: name}, nil
}

// mondayOf returns the Monday (00:00 UTC) of the week that contains d.
func mondayOf(d time.Time) time.Time {
	// Go's time.Weekday(): Sunday=0..Saturday=6. We want Monday=0..Sunday=6.
	wd := int(d.Weekday()) - 1
	if wd < 0 {
		wd = 6
	}
	return d.AddDate(0, 0, -wd)
}

// Matches reports whether the RFC3339 timestamp string falls inside the filter.
// Unparseable timestamps pass through (we never silently drop data).
func (f DateFilter) Matches(timestamp string) bool {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return true
	}
	if f.From != nil && t.Before(*f.From) {
		return false
	}
	if f.To != nil && t.After(*f.To) {
		return false
	}
	return true
}

// Label returns a human-readable period name for output headers.
func (f DateFilter) Label() string {
	if f.label != "" {
		return f.label
	}
	return "all time"
}

// FileRelevant reports whether a file whose last-modified time is mtime
// could contain any entry that passes the filter. Used for mtime prefiltering.
func (f DateFilter) FileRelevant(mtime time.Time) bool {
	if f.From != nil && mtime.Before(*f.From) {
		return false
	}
	return true
}
