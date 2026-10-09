package web

import (
	"strings"
	"testing"
	"time"

	"github.com/romeo4934/ai-reader/internal/i18n"
	"github.com/romeo4934/ai-reader/internal/srs"
	"github.com/romeo4934/ai-reader/internal/store"
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

func TestNextReview(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	now := time.Date(2026, 10, 9, 9, 0, 0, 0, paris)
	at := func(d time.Time) *time.Time { return &d }
	earlierToday := at(time.Date(2026, 10, 9, 8, 50, 0, 0, paris).UTC())
	yesterday := at(time.Date(2026, 10, 8, 20, 0, 0, 0, paris).UTC())
	for _, tc := range []struct {
		name     string
		card     store.Vocab
		result   srs.Result
		wantBox  int
		wantDays int // 0 = due again right away
	}{
		{"new card, known", store.Vocab{Box: 1}, srs.Good, 2, 3},
		{"new card, missed: back this session", store.Vocab{Box: 1}, srs.Again, 1, 0},
		{"missed today, now right: back tomorrow", store.Vocab{Box: 1, LastReviewedAt: earlierToday}, srs.Good, 1, 1},
		{"missed today, missed again", store.Vocab{Box: 1, LastReviewedAt: earlierToday}, srs.Again, 1, 0},
		{"missed yesterday, right today: normal step", store.Vocab{Box: 1, LastReviewedAt: yesterday}, srs.Good, 2, 3},
		{"box 3 known", store.Vocab{Box: 3, LastReviewedAt: yesterday}, srs.Good, 4, 14},
		{"box 4 missed", store.Vocab{Box: 4, LastReviewedAt: yesterday}, srs.Again, 1, 0},
	} {
		box, next := nextReview(tc.card, tc.result, now)
		want := now.UTC().AddDate(0, 0, tc.wantDays)
		if box != tc.wantBox || !next.Equal(want) {
			t.Errorf("%s: got box %d next %s, want box %d next %s", tc.name, box, next, tc.wantBox, want)
		}
	}
}

func TestSuggestBooks(t *testing.T) {
	for _, tc := range []struct {
		lang  string
		level int
	}{{"en", 1}, {"fr", 3}, {"es", 1}, {"it", 2}, {"nl", 3}} {
		got := suggestBooks(tc.lang, tc.level, 3)
		if len(got) != 3 {
			t.Errorf("%s/%d: %d books, want 3", tc.lang, tc.level, len(got))
		}
		if len(got) > 0 && got[0].Level != tc.level {
			t.Errorf("%s/%d: first book is level %d", tc.lang, tc.level, got[0].Level)
		}
		for _, b := range got {
			if b.Lang != tc.lang {
				t.Errorf("%s/%d: got a %s book", tc.lang, tc.level, b.Lang)
			}
		}
	}
}

func TestParseLook(t *testing.T) {
	// an old look (no figure) with a beard is a man's
	if l := parseLook(7, "s=1,h=2,hc=3,b=2,t=lapis,c=red,w=petasos", 0); l.Body != 0 || l.Beard != 2 || l.Tunic != "lapis" || l.Head != "petasos" || l.Cut != "belted" {
		t.Errorf("old look: %+v", l)
	}
	// items above one's rank fall back
	l := parseLook(7, "bd=1,s=1,h=2,hc=3,b=0,t=purple,u=long,c=gold,w=crown,x=beads_large", -1)
	if l.Body != 1 || l.Cape != "none" || l.Head != "none" || l.Tunic == "purple" || l.Cut != "long" || l.Extra != "none" {
		t.Errorf("aspirant look: %+v", l)
	}
	if got := parseLook(7, l.String(), -1); got != l {
		t.Errorf("round trip: %+v != %+v", got, l)
	}
	j := parseLook(7, "bd=1,s=0,h=4,hc=6,b=0,t=beige,c=none,w=headband,x=scarf", -1).json()
	if j.Cut != "simple" || j.Extra != "scarf" {
		t.Errorf("json cut/extra: %+v", j)
	}
	if j.Body != "female" || j.Skin != "light" || j.Hair != "" || j.HairColor != "orange" || j.Tunic != "tan" || j.Cape != "" || j.Head != "headband" {
		t.Errorf("json: %+v", j)
	}
}

func TestComputeRetention(t *testing.T) {
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	at := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d.Add(9 * time.Hour) }
	users := []store.AdminUser{
		{ID: 1, CreatedAt: at("2026-10-01"), Books: 1, Words: 3, Reviews: 5}, // back next day and in week 2
		{ID: 2, CreatedAt: at("2026-10-01"), Books: 1},                       // never back
		{ID: 3, CreatedAt: at("2026-10-18"), Words: 1},                       // too recent for week 2
		{ID: 4, CreatedAt: at("2026-10-19")},                                 // next day is today: not counted yet
	}
	act := map[int64]map[string]bool{
		1: {"2026-10-01": true, "2026-10-02": true, "2026-10-09": true},
		2: {"2026-10-01": true},
		3: {"2026-10-18": true, "2026-10-19": true},
	}
	r := computeRetention(users, act, now)
	if r.OtherDay != (retentionStat{Back: 2, Eligible: 4, Percent: 50}) {
		t.Errorf("other day: %+v", r.OtherDay)
	}
	if r.NextDay != (retentionStat{Back: 2, Eligible: 3, Percent: 66}) {
		t.Errorf("next day: %+v", r.NextDay)
	}
	if r.Week2 != (retentionStat{Back: 1, Eligible: 2, Percent: 50}) {
		t.Errorf("week 2: %+v", r.Week2)
	}
	// weeks start on Monday (the 19th is one), most recent first
	if len(r.Cohorts) != 3 || r.Cohorts[0].Week != "2026-10-19" || r.Cohorts[1].Week != "2026-10-12" || r.Cohorts[2].Week != "2026-09-28" ||
		r.Cohorts[2].Signups != 2 || r.Cohorts[2].Book != 100 || r.Cohorts[2].Review != 50 || r.Cohorts[2].Week2 != (retentionStat{1, 2, 50}) {
		t.Errorf("cohorts: %+v", r.Cohorts)
	}
	if r.DaysActive[1] != 3 {
		t.Errorf("days active: %v", r.DaysActive)
	}
}

func TestDueLabel(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	T := i18n.For("fr")
	now := time.Date(2026, 10, 9, 23, 30, 0, 0, paris)
	for next, want := range map[time.Time]string{
		now.Add(-time.Hour):                            "aujourd'hui",
		time.Date(2026, 10, 9, 21, 45, 0, 0, time.UTC): "aujourd'hui", // 23:45 in Paris
		time.Date(2026, 10, 9, 22, 15, 0, 0, time.UTC): "demain",      // 00:15 in Paris
		time.Date(2026, 10, 10, 8, 0, 0, 0, paris):     "demain",
		time.Date(2026, 10, 13, 1, 0, 0, 0, paris):     "dans 4 jours",
	} {
		if got := dueLabel(T, next, now); got != want {
			t.Errorf("dueLabel(%v) = %q, want %q", next, got, want)
		}
	}
}
