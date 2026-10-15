package main

import (
	"testing"
	"time"
)

// newDate returns midnight UTC of the given date.
func newDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestDaysUntilNewYear(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want int
	}{
		{"start of non-leap year", newDate(2025, time.January, 1), 365},
		{"start of leap year", newDate(2024, time.January, 1), 366},
		{"end of non-leap year", newDate(2025, time.December, 31), 1},
		{"end of leap year", newDate(2024, time.December, 31), 1},
		{"before Feb 29 in leap year", newDate(2024, time.February, 28), 308},
		{"Feb 29 in leap year", newDate(2024, time.February, 29), 307},
		{"after Feb 29 in leap year", newDate(2024, time.March, 1), 306},
		{"end of Feb in non-leap year", newDate(2025, time.February, 28), 307},
		{"after Feb in non-leap year", newDate(2025, time.March, 1), 306},
		{"ordinary day", newDate(2026, time.October, 14), 79},
		{"century year is not leap", newDate(2100, time.January, 1), 365},
		{"year divisible by 400 is leap", newDate(2000, time.January, 1), 366},
		{
			"time of day is ignored",
			time.Date(2025, time.December, 31, 23, 59, 59, 0, time.UTC),
			1,
		},
		{
			"time zone is ignored",
			time.Date(2025, time.December, 31, 23, 30, 0, 0, time.FixedZone("UTC+3", 3*60*60)),
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DaysUntilNewYear(tt.date)
			if got != tt.want {
				t.Errorf("DaysUntilNewYear(%s) = %d, want %d",
					tt.date.Format(time.DateOnly), got, tt.want)
			}
		})
	}
}

func BenchmarkDaysUntilNewYear(b *testing.B) {
	date := newDate(2026, time.October, 14)

	for b.Loop() {
		DaysUntilNewYear(date)
	}
}
