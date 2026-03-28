package api

import (
	"testing"
	"time"
)

func TestPeriodRange_Week(t *testing.T) {
	tests := []struct {
		name      string
		ref       time.Time
		wantStart string
		wantEnd   string
	}{
		{
			name:      "Wednesday mid-week",
			ref:       time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC), // Wednesday
			wantStart: "2026-03-23",                                   // Monday
			wantEnd:   "2026-03-29",                                   // Sunday
		},
		{
			name:      "Monday (start of week)",
			ref:       time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-03-23",
			wantEnd:   "2026-03-29",
		},
		{
			name:      "Sunday (end of week)",
			ref:       time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-03-23",
			wantEnd:   "2026-03-29",
		},
		{
			name:      "Saturday",
			ref:       time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-03-23",
			wantEnd:   "2026-03-29",
		},
		{
			name:      "week spanning month boundary",
			ref:       time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), // Wednesday
			wantStart: "2026-03-30",                                   // Monday
			wantEnd:   "2026-04-05",                                   // Sunday
		},
		{
			name:      "week spanning year boundary",
			ref:       time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), // Wednesday
			wantStart: "2025-12-29",                                    // Monday
			wantEnd:   "2026-01-04",                                    // Sunday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := periodRange("week", tt.ref)
			gotStart := start.Format("2006-01-02")
			gotEnd := end.Format("2006-01-02")
			if gotStart != tt.wantStart {
				t.Errorf("start = %s, want %s", gotStart, tt.wantStart)
			}
			if gotEnd != tt.wantEnd {
				t.Errorf("end = %s, want %s", gotEnd, tt.wantEnd)
			}
			// Verify start is Monday
			if start.Weekday() != time.Monday {
				t.Errorf("start weekday = %s, want Monday", start.Weekday())
			}
			// Verify end is Sunday
			if end.Weekday() != time.Sunday {
				t.Errorf("end weekday = %s, want Sunday", end.Weekday())
			}
		})
	}
}

func TestPeriodRange_Month(t *testing.T) {
	tests := []struct {
		name      string
		ref       time.Time
		wantStart string
		wantEnd   string
	}{
		{
			name:      "March (31 days)",
			ref:       time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-03-01",
			wantEnd:   "2026-03-31",
		},
		{
			name:      "April (30 days)",
			ref:       time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-04-01",
			wantEnd:   "2026-04-30",
		},
		{
			name:      "February non-leap year",
			ref:       time.Date(2025, 2, 14, 0, 0, 0, 0, time.UTC),
			wantStart: "2025-02-01",
			wantEnd:   "2025-02-28",
		},
		{
			name:      "February leap year",
			ref:       time.Date(2024, 2, 14, 0, 0, 0, 0, time.UTC),
			wantStart: "2024-02-01",
			wantEnd:   "2024-02-29",
		},
		{
			name:      "first day of month",
			ref:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-01-01",
			wantEnd:   "2026-01-31",
		},
		{
			name:      "last day of month",
			ref:       time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-06-01",
			wantEnd:   "2026-06-30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := periodRange("month", tt.ref)
			gotStart := start.Format("2006-01-02")
			gotEnd := end.Format("2006-01-02")
			if gotStart != tt.wantStart {
				t.Errorf("start = %s, want %s", gotStart, tt.wantStart)
			}
			if gotEnd != tt.wantEnd {
				t.Errorf("end = %s, want %s", gotEnd, tt.wantEnd)
			}
		})
	}
}

func TestPeriodRange_Year(t *testing.T) {
	tests := []struct {
		name      string
		ref       time.Time
		wantStart string
		wantEnd   string
	}{
		{
			name:      "regular year",
			ref:       time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-01-01",
			wantEnd:   "2026-12-31",
		},
		{
			name:      "leap year",
			ref:       time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			wantStart: "2024-01-01",
			wantEnd:   "2024-12-31",
		},
		{
			name:      "Jan 1",
			ref:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-01-01",
			wantEnd:   "2026-12-31",
		},
		{
			name:      "Dec 31",
			ref:       time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-01-01",
			wantEnd:   "2026-12-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := periodRange("year", tt.ref)
			gotStart := start.Format("2006-01-02")
			gotEnd := end.Format("2006-01-02")
			if gotStart != tt.wantStart {
				t.Errorf("start = %s, want %s", gotStart, tt.wantStart)
			}
			if gotEnd != tt.wantEnd {
				t.Errorf("end = %s, want %s", gotEnd, tt.wantEnd)
			}
		})
	}
}

func TestPeriodRange_Default(t *testing.T) {
	ref := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	start, end := periodRange("invalid", ref)
	if !start.Equal(ref) || !end.Equal(ref) {
		t.Errorf("default: got start=%s end=%s, want both %s", start, end, ref)
	}
}
