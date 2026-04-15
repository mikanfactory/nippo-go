package filter

import (
	"testing"
	"time"
)

// fixedNow: 2026-04-10 (Friday) 12:00:00 UTC
func fixedNow() time.Time {
	return time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
}

func TestFromDays(t *testing.T) {
	tests := []struct {
		name        string
		days        uint
		wantHasFrom bool
		wantFromDay int
	}{
		{"all time", 0, false, 0},
		{"one day", 1, true, 9},
		{"seven days", 7, true, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FromDaysAt(tt.days, fixedNow())
			if (f.From != nil) != tt.wantHasFrom {
				t.Fatalf("FromDays(%d): From presence = %v, want %v", tt.days, f.From != nil, tt.wantHasFrom)
			}
			if tt.wantHasFrom && f.From.Day() != tt.wantFromDay {
				t.Errorf("FromDays(%d): From.Day = %d, want %d", tt.days, f.From.Day(), tt.wantFromDay)
			}
			if tt.wantHasFrom && (f.From.Hour() != 0 || f.From.Minute() != 0) {
				t.Errorf("FromDays(%d): From must be midnight, got %v", tt.days, f.From)
			}
		})
	}
}

func TestFromRange(t *testing.T) {
	t.Run("both dates", func(t *testing.T) {
		f, err := FromRange("2026-03-01", "2026-03-15")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.From == nil || f.From.Year() != 2026 || f.From.Month() != 3 || f.From.Day() != 1 {
			t.Errorf("From wrong: %v", f.From)
		}
		if f.To == nil || f.To.Day() != 15 || f.To.Hour() != 23 {
			t.Errorf("To wrong: %v", f.To)
		}
	})
	t.Run("from only", func(t *testing.T) {
		f, err := FromRange("2026-03-01", "")
		if err != nil || f.From == nil || f.To != nil {
			t.Errorf("want from-only, got err=%v f=%+v", err, f)
		}
	})
	t.Run("invalid from", func(t *testing.T) {
		_, err := FromRange("bad-date", "")
		if err == nil {
			t.Error("expected error for invalid date")
		}
	})
	t.Run("invalid to", func(t *testing.T) {
		_, err := FromRange("", "2026/03/01")
		if err == nil {
			t.Error("expected error for invalid to date")
		}
	})
}

func TestFromPeriod(t *testing.T) {
	now := fixedNow() // Fri 2026-04-10
	tests := []struct {
		period   string
		fromDate string
		toDate   string
	}{
		{"today", "2026-04-10", "2026-04-10"},
		{"yesterday", "2026-04-09", "2026-04-09"},
		{"this-week", "2026-04-06", "2026-04-10"},  // Mon..Fri
		{"last-week", "2026-03-30", "2026-04-05"},  // prev Mon..Sun
		{"this-month", "2026-04-01", "2026-04-10"}, // 1st..today
		{"last-month", "2026-03-01", "2026-03-31"},
	}
	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			f, err := FromPeriodAt(tt.period, now)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if f.From.Format("2006-01-02") != tt.fromDate {
				t.Errorf("from = %s, want %s", f.From.Format("2006-01-02"), tt.fromDate)
			}
			if f.To.Format("2006-01-02") != tt.toDate {
				t.Errorf("to = %s, want %s", f.To.Format("2006-01-02"), tt.toDate)
			}
		})
	}
}

func TestFromPeriodInvalid(t *testing.T) {
	_, err := FromPeriodAt("nope", fixedNow())
	if err == nil {
		t.Error("expected error for unknown period")
	}
}

func TestMatches(t *testing.T) {
	f, _ := FromRange("2026-04-01", "2026-04-10")

	cases := []struct {
		name string
		ts   string
		want bool
	}{
		{"before", "2026-03-25T12:00:00Z", false},
		{"within", "2026-04-05T12:00:00Z", true},
		{"after", "2026-04-20T12:00:00Z", false},
		{"unparseable returns true", "not-a-timestamp", true},
		{"boundary from", "2026-04-01T00:00:00Z", true},
		{"boundary to", "2026-04-10T23:59:59Z", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := f.Matches(c.ts); got != c.want {
				t.Errorf("Matches(%q) = %v, want %v", c.ts, got, c.want)
			}
		})
	}
}

func TestMatchesOpenRange(t *testing.T) {
	f := DateFilter{} // no limits
	if !f.Matches("2020-01-01T00:00:00Z") {
		t.Error("empty filter should match everything")
	}
}

func TestLabel(t *testing.T) {
	empty := DateFilter{}
	if got := empty.Label(); got != "all time" {
		t.Errorf("empty label = %q", got)
	}
	f, _ := FromRange("2026-04-01", "2026-04-10")
	if got := f.Label(); got == "" {
		t.Error("range label empty")
	}
}
