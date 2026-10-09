package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/romeo4934/ai-reader/internal/store"
)

// The weekly leaderboard ranks readers by the review points they earned
// this week (Monday to Sunday, in the viewer's time zone), one board per
// language being learned — points count for the language of the reviewed
// card's book. An email
// account's username is its email address, which is never shown: such an
// account appears under the pseudo it chose in the settings, or else a
// random-looking one derived from its id ("Panda 42"). Older accounts have
// a real username, shown as is.

const (
	leaderboardSize = 50
	displayNameMax  = 24
)

type leaderboardEntry struct {
	Rank   int
	Name   string
	Agora  bool // member of the Lydi Agora
	Points int
	Streak string
	IsYou  bool
}

type leaderboardTab struct {
	Lang, Name string
	Active     bool
}

type leaderboardView struct {
	// Friends: showing the viewer's private league rather than everyone.
	Friends    bool
	HasFriends bool
	Lang       string
	Tabs       []leaderboardTab
	LangName   string
	Entries    []leaderboardEntry
	Ends       string // "ends tonight" / "ends in N days"
	YourName   string // how the viewer appears to others
	Ranked     bool   // the viewer has points this week
}

func publicName(userID int64, username, displayName string) string {
	switch {
	case displayName != "":
		return displayName
	case username != "" && !strings.Contains(username, "@"):
		return username
	default:
		return autoPseudo(userID)
	}
}

// Animal names that read the same in every UI language.
var pseudoAnimals = []string{
	"Panda", "Koala", "Puma", "Jaguar", "Cobra", "Condor", "Bison", "Orca",
	"Yak", "Tapir", "Lama", "Gecko", "Manta", "Ibis", "Okapi", "Dingo",
}

// autoPseudo is stable for an account (so it can be recognised week after
// week) without being stored, and reveals nothing about it.
func autoPseudo(userID int64) string {
	h := uint32(userID) * 2654435761 // Knuth's multiplicative hash
	return fmt.Sprintf("%s %d", pseudoAnimals[h%uint32(len(pseudoAnimals))], 10+(h>>8)%90)
}

// weekBounds returns the Monday and Sunday of now's week, at noon.
func weekBounds(now time.Time) (monday, sunday time.Time) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	monday = day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	return monday, monday.AddDate(0, 0, 6)
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	T := s.dictFor(r)
	now := time.Now().In(userLocation(r))
	monday, sunday := weekBounds(now)
	from, to := dayKey(monday), dayKey(sunday)
	friends, err := s.store.Friends(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	// Friends league by default once there are friends; ?scope=all for
	// everyone.
	var only []int64
	showFriends := len(friends) > 0 && r.URL.Query().Get("scope") != "all"
	if showFriends {
		only = []int64{user.ID}
		for _, f := range friends {
			only = append(only, f.ID)
		}
	}
	langs, err := s.store.LeaderboardLangs(from, to, only)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	// The tab shown: the one asked for, else the language of the viewer's
	// latest book, else the busiest board.
	// LangKey maps "" to "und" (a book without a language): only for an
	// actual ?lang=, or the viewer's own default below would be skipped.
	lang := ""
	if q := r.URL.Query().Get("lang"); q != "" && validLangKey(store.LangKey(q)) {
		lang = store.LangKey(q)
	}
	// Default: where the viewer scored most this week, else the language
	// of their latest book.
	if lang == "" {
		if lang, err = s.store.UserTopLang(user.ID, from, to); err != nil {
			s.log.Error("user top lang", "err", err)
		}
	}
	if lang == "" {
		if lang, err = s.store.UserBookLang(user.ID); err != nil {
			s.log.Error("user book lang", "err", err)
		}
	}
	if lang == "" && len(langs) > 0 {
		lang = langs[0].Lang
	}
	view := leaderboardView{
		YourName: publicName(user.ID, user.Username, user.DisplayName), LangName: langName(lang),
		Friends: showFriends, HasFriends: len(friends) > 0, Lang: lang,
	}
	hasTab := false
	for _, l := range langs {
		view.Tabs = append(view.Tabs, leaderboardTab{Lang: l.Lang, Name: langName(l.Lang), Active: l.Lang == lang})
		hasTab = hasTab || l.Lang == lang
	}
	if !hasTab && lang != "" && len(view.Tabs) > 0 {
		// The viewer's own language has no points yet this week: still
		// offer its (empty) board next to the others.
		view.Tabs = append(view.Tabs, leaderboardTab{Lang: lang, Name: langName(lang), Active: true})
	}
	rows, err := s.store.Leaderboard(from, to, lang, only, leaderboardSize)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	agora := map[int64]bool{}
	if citizens, err := s.store.AgoraCitizens(agoraThreshold); err == nil {
		for _, c := range citizens {
			agora[c.UserID] = c.Days >= agoraThreshold
		}
	}
	for i, row := range rows {
		e := leaderboardEntry{Rank: i + 1, Points: row.Points, IsYou: row.UserID == user.ID}
		if i > 0 && row.Points == rows[i-1].Points {
			e.Rank = view.Entries[i-1].Rank // ties share a rank
		}
		e.Name = publicName(row.UserID, row.Username, row.DisplayName)
		e.Agora = agora[row.UserID]
		if n := s.streak(row.UserID, now); n > 0 {
			e.Streak = streakText(T, n)
		}
		view.Ranked = view.Ranked || e.IsYou
		view.Entries = append(view.Entries, e)
	}
	switch left := int(sunday.Sub(time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())).Hours()+1) / 24; left {
	case 0:
		view.Ends = T["LeaderboardEndsTonight"]
	default:
		view.Ends = fmt.Sprintf(T["LeaderboardEndsIn"], left)
	}
	s.render(w, r, "leaderboard.html", T["LeaderboardTitle"], view)
}

// cleanDisplayName trims a pseudo and drops control characters; ok is false
// when it's too long.
func cleanDisplayName(raw string) (name string, ok bool) {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(raw))
	name = strings.Join(strings.Fields(name), " ")
	return name, utf8.RuneCountInString(name) <= displayNameMax
}

// Languages by their own name, the same whatever the UI language.
var langNames = map[string]string{
	"en": "English", "fr": "Français", "es": "Español", "pt": "Português", "it": "Italiano",
	"de": "Deutsch", "nl": "Nederlands", "sv": "Svenska", "da": "Dansk", "no": "Norsk",
	"nb": "Norsk", "fi": "Suomi", "pl": "Polski", "cs": "Čeština", "ru": "Русский",
	"uk": "Українська", "el": "Ελληνικά", "tr": "Türkçe", "ar": "العربية", "he": "עברית",
	"ja": "日本語", "zh": "中文", "und": "?", "ko": "한국어", "ca": "Català", "ro": "Română", "hu": "Magyar",
}

func langName(code string) string {
	if name, ok := langNames[code]; ok {
		return name
	}
	if code == "" {
		return "?"
	}
	return strings.ToUpper(code)
}

// validLangKey accepts a primary language subtag (2-3 letters).
func validLangKey(s string) bool {
	if len(s) < 2 || len(s) > 3 {
		return false
	}
	for _, c := range s {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}
