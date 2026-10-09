package web

import (
	"strings"
	"testing"
	"time"

	"github.com/romeo4934/ai-reader/internal/srs"
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

func TestAnswerPoints(t *testing.T) {
	for _, tc := range []struct {
		result srs.Result
		mode   string
		want   int
	}{
		{srs.Good, answerTyped, 3},
		{srs.Good, answerTypedClose, 2},
		{srs.Good, answerOverride, 2},
		{srs.Good, "", 1},
		{srs.Good, "anything", 1},
		{srs.Good, answerCopied, 1},
		{srs.Again, answerCopied, 1},
		{srs.Again, answerTyped, 0},
		{srs.Again, "", 0},
	} {
		if got := answerPoints(tc.result, tc.mode); got != tc.want {
			t.Errorf("answerPoints(%q, %q) = %d, want %d", tc.result, tc.mode, got, tc.want)
		}
	}
}

func TestWeekBounds(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	for _, tc := range []struct{ now, monday, sunday string }{
		{"2026-10-08", "2026-10-05", "2026-10-11"}, // Thursday
		{"2026-10-05", "2026-10-05", "2026-10-11"}, // Monday
		{"2026-10-11", "2026-10-05", "2026-10-11"}, // Sunday
	} {
		d, _ := time.ParseInLocation("2006-01-02", tc.now, paris)
		m, s := weekBounds(d.Add(23 * time.Hour))
		if dayKey(m) != tc.monday || dayKey(s) != tc.sunday {
			t.Errorf("%s: got %s..%s, want %s..%s", tc.now, dayKey(m), dayKey(s), tc.monday, tc.sunday)
		}
	}
}

func TestPublicNameNeverShowsEmail(t *testing.T) {
	if got := publicName(14, "someone@example.com", ""); strings.Contains(got, "@") || got == "" {
		t.Errorf("email account shown as %q", got)
	}
	if publicName(14, "someone@example.com", "") != publicName(14, "someone@example.com", "") {
		t.Error("auto pseudo isn't stable")
	}
	if got := publicName(2, "Danae", ""); got != "Danae" {
		t.Errorf("legacy username shown as %q", got)
	}
	if got := publicName(14, "someone@example.com", "Mimi"); got != "Mimi" {
		t.Errorf("chosen pseudo shown as %q", got)
	}
}
