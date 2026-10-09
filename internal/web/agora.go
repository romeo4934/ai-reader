package web

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/romeo4934/ai-reader/internal/i18n"
	"github.com/romeo4934/ai-reader/internal/store"
)

// The Lydi Agora: a status, not a reward. Lydia was the ancient kingdom of
// Sardis and King Croesus, where the first coins were struck; its agora
// welcomes, for good, every reader who has completed agoraThreshold daily
// challenges (not necessarily in a row). Members get a coin-like medallion
// on the temple, and a rank that grows with their total; readers on their
// way stand on the square outside, each with their progress.

const agoraThreshold = 30

// Ranks by total completed challenges.
var agoraRanks = []struct {
	Days int
	Key  string
}{
	{30, "AgoraRank1"}, {100, "AgoraRank2"}, {250, "AgoraRank3"}, {365, "AgoraRank4"},
}

var avatarSymbols = []string{"🦉", "🏺", "🌿", "🏛️", "🎭", "📜", "⚖️", "🪙", "🐎", "🦁", "☀️", "🌙", "⚓", "🍇"}

var avatarColors = []struct{ Key, Hex string }{
	{"terracotta", "#c0603a"}, {"olive", "#6b7d3a"}, {"lapis", "#2f5d9c"}, {"gold", "#b8901f"},
	{"purple", "#6d4c8e"}, {"teal", "#2f7f7a"}, {"rose", "#b5546b"}, {"stone", "#7a7266"},
}

func agoraRank(T i18n.Dict, days int) (name string, next string, nextDays int) {
	for i, r := range agoraRanks {
		if days >= r.Days {
			name = T[r.Key]
			if i+1 < len(agoraRanks) {
				next, nextDays = T[agoraRanks[i+1].Key], agoraRanks[i+1].Days
			} else {
				next, nextDays = "", 0
			}
		}
	}
	return name, next, nextDays
}

// avatarColor is the chosen color, or one picked from the user's id so
// that everyone starts with a different medallion.
func avatarColor(userID int64, key string) string {
	for _, c := range avatarColors {
		if c.Key == key {
			return c.Hex
		}
	}
	h := fnv.New32a()
	fmt.Fprint(h, userID)
	return avatarColors[h.Sum32()%uint32(len(avatarColors))].Hex
}

type medal struct {
	Symbol   string // emoji, or the initial
	Color    string
	Name     string
	Title    string // tooltip
	Laurel   bool   // Sage
	You      bool
	Progress int // aspirants: completed challenges out of agoraThreshold
}

// person is one reader on the game map.
type person struct {
	Name   string   `json:"name"`
	Symbol string   `json:"symbol"`
	Color  string   `json:"color"`
	Member bool     `json:"member"`
	Rank   string   `json:"rank"`
	Days   int      `json:"days"`
	Laurel bool     `json:"laurel"`
	You    bool     `json:"you"`
	Look   lookJSON `json:"look"`
}

type agoraRow struct {
	Name, Rank, Since string
	Days, Percent     int
	Medal             medal
	You               bool
}

type agoraView struct {
	Member            bool
	Days              int
	Rank, NextRank    string
	NextDays, Left    int
	Welcome           string
	Members, Gates    []agoraRow
	Symbols           []string
	Colors            []struct{ Key, Hex string }
	MySymbol, MyColor string
	MyColorKey        string
	MyInitial         string
	Threshold         int
	Percent           int // progress toward the threshold
	People            []person
	Editor            lookEditor
	Visitor           bool // not logged in: watching only
	MemberCount       int
}

func medalFor(userID int64, username, displayName, symbol, color string) medal {
	name := publicName(userID, username, displayName)
	m := medal{Name: name, Color: avatarColor(userID, color), Symbol: symbol}
	if m.Symbol == "" {
		r, _ := utf8.DecodeRuneInString(name)
		m.Symbol = strings.ToUpper(string(r))
	}
	return m
}

func (s *Server) handleAgora(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	T := s.dictFor(r)
	citizens, err := s.store.AgoraCitizens(agoraThreshold)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	v := agoraView{
		Threshold: agoraThreshold, Symbols: avatarSymbols, Colors: avatarColors,
		MySymbol: user.AvatarSymbol, MyColor: avatarColor(user.ID, user.AvatarColor), MyColorKey: user.AvatarColor,
	}
	v.MyInitial = medalFor(user.ID, user.Username, user.DisplayName, "", "").Symbol
	for _, c := range citizens {
		m := medalFor(c.UserID, c.Username, c.DisplayName, c.AvatarSymbol, c.AvatarColor)
		m.You = c.UserID == user.ID
		row := agoraRow{Name: m.Name, Days: c.Days, You: m.You}
		if c.Days >= agoraThreshold {
			row.Rank, _, _ = agoraRank(T, c.Days)
			row.Since = c.EnteredOn
			m.Laurel = c.Days >= agoraRanks[len(agoraRanks)-1].Days
			m.Title = m.Name + " · " + row.Rank
			row.Medal = m
			v.Members = append(v.Members, row)
		} else {
			m.Progress = c.Days
			m.Title = fmt.Sprintf("%s · %d / %d", m.Name, c.Days, agoraThreshold)
			row.Percent = c.Days * 100 / agoraThreshold
			row.Medal = m
			v.Gates = append(v.Gates, row)
		}
		if m.You {
			v.Days = c.Days
		}
		v.People = append(v.People, person{
			Name: m.Name, Symbol: m.Symbol, Color: m.Color, Member: c.Days >= agoraThreshold,
			Rank: row.Rank, Days: c.Days, Laurel: m.Laurel, You: m.You,
			Look: parseLook(c.UserID, c.AvatarLook, agoraTier(c.Days)).json(),
		})
	}
	if v.Days == 0 {
		// Not on the list yet (no challenge done): still on the map,
		// outside, as the viewer.
		me := medalFor(user.ID, user.Username, user.DisplayName, user.AvatarSymbol, user.AvatarColor)
		v.People = append(v.People, person{Name: me.Name, Symbol: me.Symbol, Color: me.Color, You: true,
			Look: parseLook(user.ID, user.AvatarLook, -1).json()})
	}
	v.Member = v.Days >= agoraThreshold
	tier := agoraTier(v.Days)
	v.Editor = newLookEditor(T, parseLook(user.ID, user.AvatarLook, tier), tier)
	if v.Member {
		v.Rank, v.NextRank, v.NextDays = agoraRank(T, v.Days)
		if !user.AgoraWelcomed {
			v.Welcome = fmt.Sprintf(T["AgoraWelcome"], v.Rank, publicName(user.ID, user.Username, user.DisplayName))
			if err := s.store.SetAgoraWelcomed(user.ID); err != nil {
				s.log.Error("agora welcomed", "err", err)
			}
		}
	} else {
		v.Left = agoraThreshold - v.Days
		v.Percent = v.Days * 100 / agoraThreshold
	}
	s.render(w, r, "agora.html", T["AgoraTitle"], v)
}

// handleAgoraPage shows the Agora to everyone: the reader's own view when
// logged in, else the visitor's.
func (s *Server) handleAgoraPage(w http.ResponseWriter, r *http.Request) {
	if r2, ok := s.withSessionUser(r); ok {
		s.handleAgora(w, r2)
		return
	}
	s.handleAgoraVisitor(w, r)
}

// visitorName is how a reader appears to visitors: their chosen pseudo,
// else their automatic one — never the login name, which may be their
// real name.
func visitorName(userID int64, displayName string) string {
	if displayName != "" {
		return displayName
	}
	return autoPseudo(userID)
}

// handleAgoraVisitor: the Agora as a showcase for visitors — the living
// square to watch and explore (no character of their own), with an
// invitation to join.
func (s *Server) handleAgoraVisitor(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	citizens, err := s.store.AgoraCitizens(agoraThreshold)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	v := agoraView{Visitor: true, Threshold: agoraThreshold}
	for _, c := range citizens {
		member := c.Days >= agoraThreshold
		p := person{Name: visitorName(c.UserID, c.DisplayName), Member: member, Days: c.Days,
			Look: parseLook(c.UserID, c.AvatarLook, agoraTier(c.Days)).json()}
		if member {
			v.MemberCount++
			p.Rank, _, _ = agoraRank(T, c.Days)
			p.Laurel = c.Days >= agoraRanks[len(agoraRanks)-1].Days
		}
		v.People = append(v.People, p)
	}
	s.renderDict(w, r, T, "agora.html", T["AgoraTitle"], v)
}

func (s *Server) handleAgoraAvatar(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	symbol := r.FormValue("symbol")
	symbol = validSymbol(symbol)
	color := r.FormValue("color")
	if !slices.ContainsFunc(avatarColors, func(c struct{ Key, Hex string }) bool { return c.Key == color }) {
		color = ""
	}
	if err := s.store.SetAvatar(user.ID, symbol, color); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/agora", http.StatusSeeOther)
}

// agoraStatus is the review page's line about the Agora: progress toward
// it, or the news of entering it.
func (s *Server) agoraStatus(T i18n.Dict, user *store.User) (gauge string, entered bool) {
	days, err := s.store.CompletedDayCount(user.ID)
	if err != nil {
		return "", false
	}
	if days >= agoraThreshold {
		return "", !user.AgoraWelcomed
	}
	return fmt.Sprintf(T["AgoraGauge"], days, agoraThreshold), false
}

func validSymbol(s string) string {
	if slices.Contains(avatarSymbols, s) {
		return s
	}
	return ""
}
