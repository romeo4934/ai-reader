package web

import (
	"sort"
	"time"

	"github.com/romeo4934/ai-reader/internal/store"
)

// Whether readers come back: for the admin dashboard, from the days each
// one used Lydi (reviewed, looked up a word, made an AI call). A reader
// counts for a measure only once its window is over — "back the next day"
// once that day has passed — so recent signups don't drag the rates down.

type retentionStat struct {
	Back, Eligible, Percent int
}

func (r *retentionStat) add(back bool) {
	r.Eligible++
	if back {
		r.Back++
	}
	r.Percent = r.Back * 100 / r.Eligible
}

// cohortRow: the readers who signed up in one week, and how far they got.
type cohortRow struct {
	Week               string // its Monday
	Signups            int
	Book, Word, Review int // percent of the cohort
	OtherDay           int // percent back on another day than signup
	NextDay, Week2     retentionStat
}

type retention struct {
	OtherDay, NextDay, Week2 retentionStat
	Cohorts                  []cohortRow
	DaysActive               map[int64]int
}

func day(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }

// computeRetention measures users (admins left out by the caller) against
// their activity days, as of today:
//   - another day: active on any day other than the signup day;
//   - next day: active the day after signing up (once that day is over);
//   - week 2: active between days 7 and 13 after signing up (once that
//     window is over).
func computeRetention(users []store.AdminUser, activity map[int64]map[string]bool, now time.Time) retention {
	today := day(now.UTC())
	r := retention{DaysActive: map[int64]int{}}
	byWeek := map[string]*cohortRow{}
	var weeks []string
	pct := func(n, of int) int {
		if of == 0 {
			return 0
		}
		return n * 100 / of
	}
	type counts struct{ book, word, review, other int }
	weekCounts := map[string]*counts{}
	for _, u := range users {
		signup := day(u.CreatedAt.UTC())
		days := activity[u.ID]
		r.DaysActive[u.ID] = len(days)
		other := false
		for d := range days {
			if d != dayKey(signup) {
				other = true
				break
			}
		}
		r.OtherDay.add(other)
		monday := signup.AddDate(0, 0, -((int(signup.Weekday()) + 6) % 7))
		wk := dayKey(monday)
		c := byWeek[wk]
		if c == nil {
			c = &cohortRow{Week: wk}
			byWeek[wk] = c
			weekCounts[wk] = &counts{}
			weeks = append(weeks, wk)
		}
		c.Signups++
		wc := weekCounts[wk]
		if u.Books > 0 {
			wc.book++
		}
		if u.Words > 0 {
			wc.word++
		}
		if u.Reviews > 0 {
			wc.review++
		}
		if other {
			wc.other++
		}
		if next := signup.AddDate(0, 0, 1); next.Before(today) {
			back := days[dayKey(next)]
			r.NextDay.add(back)
			c.NextDay.add(back)
		}
		if end := signup.AddDate(0, 0, 13); end.Before(today) {
			back := false
			for i := 7; i <= 13; i++ {
				if days[dayKey(signup.AddDate(0, 0, i))] {
					back = true
					break
				}
			}
			r.Week2.add(back)
			c.Week2.add(back)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(weeks))) // most recent week first
	for _, wk := range weeks {
		c, wc := byWeek[wk], weekCounts[wk]
		c.Book, c.Word, c.Review, c.OtherDay = pct(wc.book, c.Signups), pct(wc.word, c.Signups), pct(wc.review, c.Signups), pct(wc.other, c.Signups)
		r.Cohorts = append(r.Cohorts, *c)
	}
	return r
}
