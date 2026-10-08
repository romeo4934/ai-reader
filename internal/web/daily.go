package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/romeo4934/ai-reader/internal/i18n"
	"github.com/romeo4934/ai-reader/internal/srs"
	"github.com/romeo4934/ai-reader/internal/store"
)

// The daily challenge: each day's review session is every due review plus
// at most the user's DailyNewLimit never-reviewed cards. Finishing it marks
// the day completed, and consecutive completed days make the streak.

// tzCookie holds the browser's IANA time zone (set by the "head" partial),
// so "today" and the streak follow the reader's own midnight, not UTC's.
const tzCookie = "tz"

// DailyNewLimits are the choices offered in the settings.
var DailyNewLimits = []int{5, 10, 15, 20, 30}

const defaultDailyNewLimit = 10

func userLocation(r *http.Request) *time.Location {
	if c, err := r.Cookie(tzCookie); err == nil && c.Value != "" && len(c.Value) < 64 {
		if loc, err := time.LoadLocation(c.Value); err == nil {
			return loc
		}
	}
	return time.UTC
}

func dayKey(t time.Time) string { return t.Format("2006-01-02") }

func dailyNewLimit(u *store.User) int {
	if u.DailyNewLimit < 1 {
		return defaultDailyNewLimit
	}
	return u.DailyNewLimit
}

type dailyState struct {
	Day        string
	Done       int // cards answered today
	Remaining  int // due reviews + new cards still allowed today
	Total      int // Done + Remaining
	Percent    int
	NewLeft    int // new cards still allowed today
	NewWaiting int // new cards beyond today's allowance
	DeckSize   int
	Limit      int
	MoreCount  int // what "add more new words" would pull in
	Points     int // earned today
}

func (s *Server) dailyState(user *store.User, now time.Time) (dailyState, error) {
	st := dailyState{Day: dayKey(now), Limit: dailyNewLimit(user)}
	act, err := s.store.GetDailyActivity(user.ID, st.Day)
	if err != nil {
		return st, err
	}
	due, newCards, total, err := s.store.DeckCounts(user.ID, now.UTC())
	if err != nil {
		return st, err
	}
	allowed := st.Limit + act.ExtraNew - act.NewCards
	if allowed < 0 {
		allowed = 0
	}
	st.NewLeft = min(allowed, newCards)
	st.NewWaiting = newCards - st.NewLeft
	st.Done = act.Reviews
	st.Points = act.Points
	st.Remaining = due + st.NewLeft
	st.Total = st.Done + st.Remaining
	st.DeckSize = total
	st.MoreCount = min(st.Limit, st.NewWaiting)
	if st.Total > 0 {
		st.Percent = st.Done * 100 / st.Total
	}
	return st, nil
}

// streakDays counts consecutive completed days ending today (or yesterday,
// when today's challenge isn't done yet: the streak isn't lost until the day
// is over). One missed day per 7 is forgiven, so a single busy evening
// doesn't wipe out weeks of work.
func streakDays(completed map[string]bool, now time.Time) int {
	// Noon avoids DST edges when stepping back one calendar day at a time.
	day := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	if !completed[dayKey(day)] {
		day = day.AddDate(0, 0, -1)
	}
	n, lastJoker := 0, -1
	for i := 0; i < streakLookback; i++ {
		d := day.AddDate(0, 0, -i)
		if completed[dayKey(d)] {
			n++
			continue
		}
		if completed[dayKey(d.AddDate(0, 0, -1))] && (lastJoker < 0 || i-lastJoker >= 7) {
			lastJoker = i
			continue
		}
		break
	}
	return n
}

const streakLookback = 400

func (s *Server) streak(userID int64, now time.Time) int {
	since := dayKey(now.AddDate(0, 0, -streakLookback-2))
	completed, err := s.store.CompletedDays(userID, since)
	if err != nil {
		s.log.Error("completed days", "err", err)
		return 0
	}
	return streakDays(completed, now)
}

func streakText(T i18n.Dict, n int) string {
	if n == 1 {
		return T["DailyStreakOne"]
	}
	return fmt.Sprintf(T["DailyStreakMany"], n)
}

// handleReviewMore lets someone who finished today's challenge pull in
// another batch of new cards right away.
func (s *Server) handleReviewMore(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	now := time.Now().In(userLocation(r))
	if err := s.store.AddExtraNew(user.ID, dayKey(now), dailyNewLimit(user)); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/review", http.StatusSeeOther)
}

// Points reward effort, not just success: typing the word out (production)
// is worth more than recognising it. They're motivation only — the Leitner
// schedule depends on good/again alone.
const (
	answerTyped      = "typed"       // typed the exact word
	answerTypedClose = "typed_close" // typed it with a slip (accent, typo, other form)
)

func answerPoints(result srs.Result, mode string) int {
	if result != srs.Good {
		return 0
	}
	if mode == answerTyped {
		return 2
	}
	return 1 // typed_close, or "I knew it" without typing
}
