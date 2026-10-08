package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The weekly leaderboard ranks readers by the review points they earned
// this week (Monday to Sunday, in the viewer's time zone). An email
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
	Points int
	Streak string
	IsYou  bool
}

type leaderboardView struct {
	Entries  []leaderboardEntry
	Ends     string // "ends tonight" / "ends in N days"
	YourName string // how the viewer appears to others
	Ranked   bool   // the viewer has points this week
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
	rows, err := s.store.Leaderboard(dayKey(monday), dayKey(sunday), leaderboardSize)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	view := leaderboardView{YourName: publicName(user.ID, user.Username, user.DisplayName)}
	for i, row := range rows {
		e := leaderboardEntry{Rank: i + 1, Points: row.Points, IsYou: row.UserID == user.ID}
		if i > 0 && row.Points == rows[i-1].Points {
			e.Rank = view.Entries[i-1].Rank // ties share a rank
		}
		e.Name = publicName(row.UserID, row.Username, row.DisplayName)
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
