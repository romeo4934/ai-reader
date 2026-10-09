package web

import (
	"fmt"
	"net/http"
	"slices"
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

// New words only enter while fewer than fragilePerNew × the daily new-word
// limit are still fragile (reviewed, box 1-2): someone missing a lot gets a
// pause on new words until their small list holds, instead of a pile that
// never sticks. Scaled on the limit because every new word stays fragile
// for at least the 3 days of box 2 — at 10 a day, ~30 fragile words is just
// the normal pace, and the margin above it is what missed words can take.
const fragilePerNew = 4

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
	Day       string
	Done      int // cards answered today
	Remaining int // due reviews + new cards still allowed today (in the scoped language)
	// AllRemaining is Remaining across all languages: what's left of the
	// day's challenge.
	AllRemaining int
	Total        int // Done + Remaining
	Percent      int
	NewLeft      int // new cards still allowed today
	NewWaiting   int // new cards beyond today's allowance
	DeckSize     int
	Limit        int
	MoreCount    int // what "add more new words" would pull in
	Points       int // earned today
	Fragile      int
	// NewPaused: new words are waiting but held back by fragileCap.
	NewPaused bool
	// Lang is the language the state is scoped to ("" = all), which "add
	// more new words" applies to.
	Lang string
}

// dailyState is today's challenge. The daily new-word limit (and the
// fragile-words cap) apply per language, so learning a second language
// doesn't starve it; the challenge itself is one for all languages (one
// streak). lang != "" scopes Remaining, NewLeft, NewWaiting and MoreCount
// to that language's words, for its review tab; Done, Total and Percent
// stay the whole day's.
func (s *Server) dailyState(user *store.User, now time.Time, lang string) (dailyState, error) {
	st := dailyState{Day: dayKey(now), Limit: dailyNewLimit(user), Lang: lang}
	act, err := s.store.GetDailyActivity(user.ID, st.Day)
	if err != nil {
		return st, err
	}
	decks, err := s.store.DeckCountsByLang(user.ID, now.UTC())
	if err != nil {
		return st, err
	}
	days, err := s.store.LangDays(user.ID, st.Day)
	if err != nil {
		return st, err
	}
	for l, deck := range decks {
		d := days[l]
		allowed := max(0, st.Limit+d.ExtraNew-d.NewCards)
		room := max(0, fragilePerNew*st.Limit-deck.Fragile)
		newLeft := min(allowed, deck.New, room)
		st.AllRemaining += deck.DueReviews + newLeft
		st.DeckSize += deck.Total
		st.Fragile += deck.Fragile
		if lang == "" || l == lang {
			st.NewLeft += newLeft
			st.NewWaiting += deck.New - newLeft
			st.Remaining += deck.DueReviews + newLeft
			if lang != "" {
				st.MoreCount = min(st.Limit, deck.New-newLeft, room-newLeft)
				st.NewPaused = deck.New-newLeft > 0 && room-newLeft <= 0
				st.Fragile = deck.Fragile
			}
		}
	}
	if lang == "" && len(decks) == 1 {
		// One language: the tab-less review page acts on it.
		for l := range decks {
			return s.dailyState(user, now, l)
		}
	}
	st.Done = act.Reviews
	st.Points = act.Points
	st.Total = st.Done + st.AllRemaining
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
	lang := store.LangKey(r.FormValue("lang"))
	if err := s.store.AddExtraNew(user.ID, dayKey(now), lang, dailyNewLimit(user)); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/review", http.StatusSeeOther)
}

// Points reward effort, not just success: typing the word out (production)
// is worth more than recognising it, and even copying out a missed word
// helps it stick. They're motivation only — the Leitner schedule depends on
// good/again alone, so a copied word still comes back tomorrow.
const (
	answerTyped      = "typed"       // typed the exact word
	answerTypedClose = "typed_close" // typed it with a slip (accent, typo, other form)
	answerOverride   = "override"    // typed something marked wrong, then "I was right"
	answerCopied     = "copied"      // missed it, then copied it out
)

func answerPoints(result srs.Result, mode string) int {
	if result != srs.Good {
		if mode == answerCopied {
			return 1
		}
		return 0
	}
	switch mode {
	case answerTyped:
		return 3
	case answerTypedClose, answerOverride:
		return 2
	default:
		return 1 // "I knew it" without typing
	}
}

// dayStart is midnight of now's day, in now's location.
func dayStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// nextReview schedules a card after an answer. On top of the plain Leitner
// step, a missed card comes back at the end of today's session instead of
// only tomorrow (seeing it again right after missing it is what fixes it),
// and getting it right then keeps it in box 1 — back tomorrow, since one
// success minutes after a miss doesn't show it's learned.
func nextReview(card store.Vocab, result srs.Result, now time.Time) (box int, next time.Time) {
	box, next = srs.Next(card.Box, result, now)
	missedToday := card.Box == 1 && card.LastReviewedAt != nil && !card.LastReviewedAt.Before(dayStart(now))
	switch {
	case result != srs.Good:
		return 1, now.UTC()
	case missedToday:
		return 1, now.UTC().AddDate(0, 0, 1)
	}
	return box, next
}

// deckLangCookie remembers the language tab picked on /review and /words.
const deckLangCookie = "deck_lang"

// deckLang picks which language's words /review and /words show, when the
// reader learns more than one: the one asked for (remembered), else the
// remembered one, else the first in langs. "" when there's only one
// language — no tabs, no filter.
func deckLang(w http.ResponseWriter, r *http.Request, langs []string) string {
	if len(langs) < 2 {
		return ""
	}
	if q := r.URL.Query().Get("lang"); slices.Contains(langs, q) {
		http.SetCookie(w, &http.Cookie{Name: deckLangCookie, Value: q, Path: "/", MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode})
		return q
	}
	if c, err := r.Cookie(deckLangCookie); err == nil && slices.Contains(langs, c.Value) {
		return c.Value
	}
	return langs[0]
}
