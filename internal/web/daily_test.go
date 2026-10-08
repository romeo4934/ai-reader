package web

import (
	"testing"
	"time"
)

func TestStreakDays(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	now := time.Date(2026, 10, 8, 21, 0, 0, 0, paris)
	days := func(ds ...string) map[string]bool {
		m := map[string]bool{}
		for _, d := range ds {
			m[d] = true
		}
		return m
	}
	for _, tc := range []struct {
		name      string
		completed map[string]bool
		want      int
		now       time.Time // zero = now
	}{
		{"nothing", days(), 0, time.Time{}},
		{"today only", days("2026-10-08"), 1, time.Time{}},
		{"today not done yet keeps yesterday's streak", days("2026-10-07", "2026-10-06"), 2, time.Time{}},
		{"three in a row", days("2026-10-08", "2026-10-07", "2026-10-06"), 3, time.Time{}},
		{"one missed day is forgiven", days("2026-10-08", "2026-10-06", "2026-10-05"), 3, time.Time{}},
		{"missed yesterday, today pending", days("2026-10-06", "2026-10-05"), 2, time.Time{}},
		{"two missed days in a row break it", days("2026-10-08", "2026-10-05"), 1, time.Time{}},
		{"second miss within a week breaks it", days("2026-10-08", "2026-10-06", "2026-10-04", "2026-10-03"), 2, time.Time{}},
		{"misses a week apart are both forgiven", days(
			"2026-10-08", "2026-10-06", "2026-10-05", "2026-10-04", "2026-10-03", "2026-10-02", "2026-10-01", "2026-09-30",
			"2026-09-28"), 9, time.Time{}},
		{"across the DST change", days("2026-10-26", "2026-10-25", "2026-10-24"), 3, time.Date(2026, 10, 26, 8, 0, 0, 0, paris)},
	} {
		at := now
		if !tc.now.IsZero() {
			at = tc.now
		}
		got := streakDays(tc.completed, at)
		if got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
