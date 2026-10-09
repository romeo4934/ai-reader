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
	X, Y, R  int
	Symbol   string // emoji, or the initial
	Color    string
	Name     string
	Label    string // short name under the medallion
	Title    string // tooltip
	Laurel   bool   // Sage
	You      bool
	Progress int // aspirants: completed challenges out of agoraThreshold
	Dash     string
	More     int // a "+N" placeholder for those who don't fit
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
	Inside, Outside   []medal
	Members, Gates    []agoraRow
	Symbols           []string
	Colors            []struct{ Key, Hex string }
	MySymbol, MyColor string
	MyColorKey        string
	MyInitial         string
	Threshold         int
	Percent           int // progress toward the threshold
}

func medalFor(userID int64, username, displayName, symbol, color string) medal {
	name := publicName(userID, username, displayName)
	m := medal{Name: name, Color: avatarColor(userID, color), Symbol: symbol, Label: name}
	if r := []rune(name); len(r) > 8 {
		m.Label = string(r[:7]) + "…"
	}
	if m.Symbol == "" {
		r, _ := utf8.DecodeRuneInString(name)
		m.Symbol = strings.ToUpper(string(r))
	}
	return m
}

// Scene layout, in the SVG's 400×330 coordinate space: members stand in
// rows on the temple's steps and between its columns, aspirants on the
// square in front.
var (
	insideRows  = []struct{ Y, Count int }{{190, 9}, {161, 8}, {132, 7}}
	outsideRows = []struct{ Y, Count int }{{262, 11}, {296, 12}}
)

func place(medals []medal, rows []struct{ Y, Count int }, r int) []medal {
	var out []medal
	i := 0
	for _, row := range rows {
		n := min(row.Count, len(medals)-i)
		if n <= 0 {
			break
		}
		step := 300 / max(row.Count, 1)
		x0 := 200 - (n-1)*step/2
		for k := 0; k < n; k++ {
			m := medals[i]
			m.X, m.Y, m.R = x0+k*step, row.Y, r
			out = append(out, m)
			i++
		}
	}
	if left := len(medals) - i; left > 0 && len(out) > 0 {
		// The last spot shows how many more there are.
		last := &out[len(out)-1]
		*last = medal{X: last.X, Y: last.Y, R: r, More: left + 1, Color: "var(--muted)"}
	}
	return out
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
	var inside, outside []medal
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
			inside = append(inside, m)
			v.Members = append(v.Members, row)
		} else {
			m.Progress = c.Days
			circ := 2 * 3.14159 * 12.5
			m.Dash = fmt.Sprintf("%.1f %.1f", circ*float64(c.Days)/agoraThreshold, circ)
			m.Title = fmt.Sprintf("%s · %d / %d", m.Name, c.Days, agoraThreshold)
			row.Percent = c.Days * 100 / agoraThreshold
			row.Medal = m
			outside = append(outside, m)
			v.Gates = append(v.Gates, row)
		}
		if m.You {
			v.Days = c.Days
		}
	}
	v.Inside = place(inside, insideRows, 11)
	v.Outside = place(outside, outsideRows, 10)
	v.Member = v.Days >= agoraThreshold
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

func (s *Server) handleAgoraAvatar(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	symbol := r.FormValue("symbol")
	if symbol != "" && !slices.Contains(avatarSymbols, symbol) {
		symbol = ""
	}
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
