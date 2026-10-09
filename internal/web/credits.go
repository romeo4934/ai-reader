package web

import (
	"encoding/csv"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"
)

// The credits page: the LPC sprites' authors and licenses, which their
// licenses (CC-BY-SA 3.0, GPL 3.0, OGA-BY 3.0) require us to show. They
// come from static/lpc/CREDITS.csv — the LPC generator's credits for the
// sheets we use — grouped by item (an item's walk, idle and sit sheets).

type creditItem struct {
	Name     string
	Authors  []string
	Licenses []string
	URLs     []string
}

var (
	creditsOnce  sync.Once
	creditsItems []creditItem
)

func loadCredits() []creditItem {
	creditsOnce.Do(func() {
		f, err := staticFS.Open("static/lpc/CREDITS.csv")
		if err != nil {
			return
		}
		defer f.Close()
		rows, err := csv.NewReader(f).ReadAll()
		if err != nil || len(rows) < 2 {
			return
		}
		byName := map[string]*creditItem{}
		var order []string
		add := func(list []string, field string) []string {
			for _, v := range strings.Split(field, ",") {
				if v = strings.TrimSpace(v); v != "" && !slices.Contains(list, v) {
					list = append(list, v)
				}
			}
			return list
		}
		for _, r := range rows[1:] {
			if len(r) < 5 {
				continue
			}
			name := path.Dir(r[0])
			it := byName[name]
			if it == nil {
				it = &creditItem{Name: name}
				byName[name] = it
				order = append(order, name)
			}
			it.Authors = add(it.Authors, r[2])
			it.Licenses = add(it.Licenses, r[3])
			it.URLs = add(it.URLs, r[4])
		}
		for _, n := range order {
			creditsItems = append(creditsItems, *byName[n])
		}
	})
	return creditsItems
}

func (s *Server) handleCredits(w http.ResponseWriter, r *http.Request) {
	r, loggedIn := s.withSessionUser(r)
	T := s.dictFor(r)
	if !loggedIn {
		T = s.visitorDict(w, r)
	}
	s.renderDict(w, r, T, "credits.html", T["CreditsTitle"], loadCredits())
}
