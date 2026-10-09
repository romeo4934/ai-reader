package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/romeo4934/ai-reader/internal/ai"
	"github.com/romeo4934/ai-reader/internal/store"
)

// The admin dashboard: the signup funnel, daily activity, and what the AI
// costs per reader — to know whether people get to the point where Lydi
// helps them, and when a paid plan has to exist. Only for cfg.Admins.

type usageUserKey struct{}

// withUsageUser tags a context with the user an AI call is made for, when
// it isn't a request's (recall cards prepared in the background).
func withUsageUser(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, usageUserKey{}, userID)
}

func (s *Server) recordAIUsage(ctx context.Context, kind string, u ai.Usage) {
	userID, _ := ctx.Value(usageUserKey{}).(int64)
	if user, ok := ctx.Value(ctxUser).(*store.User); ok && userID == 0 {
		userID = user.ID
	}
	err := s.store.RecordAIUsage(dayKey(time.Now().UTC()), userID, kind, store.TokenUsage{
		Input: u.Input, CacheWrite: u.CacheWrite, CacheRead: u.CacheRead, Output: u.Output,
	})
	if err != nil {
		s.log.Error("enregistrement conso IA", "err", err)
	}
}

func (s *Server) isAdmin(u *store.User) bool { return s.isAdminLogin(u.Username, u.Email) }

func (s *Server) isAdminLogin(username, email string) bool {
	for _, a := range s.cfg.Admins {
		if a != "" && (strings.EqualFold(a, username) || strings.EqualFold(a, email)) {
			return true
		}
	}
	return false
}

// cost estimates a token bill in USD: cache writes cost 1.25× input, cache
// reads 0.1× (Anthropic's pricing structure).
func (s *Server) cost(u store.TokenUsage) float64 {
	in := float64(u.Input) + 1.25*float64(u.CacheWrite) + 0.1*float64(u.CacheRead)
	return (in*s.cfg.PriceIn + float64(u.Output)*s.cfg.PriceOut) / 1e6
}

type funnelStep struct {
	Label   string
	Count   int
	Percent int
}

type adminUserRow struct {
	store.AdminUser
	Name       string
	Cost       string
	DaysActive int
}

type adminKindRow struct {
	Kind  string
	Calls int64
	Cost  string
}

type adminView struct {
	Users, Verified, New7d, New30d int
	Active1d, Active7d             int
	Funnel                         []funnelStep
	Days                           []store.DayStat
	Kinds                          []adminKindRow
	CostMonth, CostPerActive       string
	TranslationsMonth              int
	Rows                           []adminUserRow
	Month                          string
	Retention                      retention
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if !s.isAdmin(user) {
		http.NotFound(w, r)
		return
	}
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	week := dayKey(now.AddDate(0, 0, -6))
	users, err := s.store.AdminUsers(week, dayKey(monthStart), monthStart.Format("2006-01"))
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	v := adminView{Users: len(users), Month: monthStart.Format("01/2006")}
	activity, err := s.store.ActivityDays()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	// Retention leaves the admins out: their own testing isn't readers
	// coming back.
	var readers []store.AdminUser
	for _, u := range users {
		if !s.isAdminLogin(u.Username, u.Email) {
			readers = append(readers, u)
		}
	}
	v.Retention = computeRetention(readers, activity, now)
	var withBook, withWords, reviewed int
	var total store.TokenUsage
	for _, u := range users {
		if u.Verified || u.Email == "" {
			v.Verified++
		}
		if now.Sub(u.CreatedAt) <= 7*24*time.Hour {
			v.New7d++
		}
		if now.Sub(u.CreatedAt) <= 30*24*time.Hour {
			v.New30d++
		}
		if u.Books > 0 {
			withBook++
		}
		if u.Words > 0 {
			withWords++
		}
		if u.Reviews > 0 {
			reviewed++
		}
		if u.Reviews7d > 0 {
			v.Active7d++
		}
		if u.LastActive == dayKey(now) {
			v.Active1d++
		}
		v.TranslationsMonth += u.TranslationsMonth
		total.Input += u.AI.Input
		total.CacheWrite += u.AI.CacheWrite
		total.CacheRead += u.AI.CacheRead
		total.Output += u.AI.Output
		v.Rows = append(v.Rows, adminUserRow{AdminUser: u, Name: publicName(u.ID, u.Username, u.DisplayName), Cost: usd(s.cost(u.AI)), DaysActive: len(activity[u.ID])})
	}
	sort.SliceStable(v.Rows, func(i, j int) bool { return v.Rows[i].LastActive > v.Rows[j].LastActive })
	for _, st := range []struct {
		label string
		n     int
	}{
		{"Inscrits (email confirmé ou ancien compte)", v.Verified},
		{"Ont ajouté un livre", withBook},
		{"Ont cherché au moins un mot", withWords},
		{"Ont fait au moins une révision", reviewed},
		{"Actifs sur 7 jours (au moins une révision)", v.Active7d},
	} {
		step := funnelStep{Label: st.label, Count: st.n}
		if v.Verified > 0 {
			step.Percent = st.n * 100 / v.Verified
		}
		v.Funnel = append(v.Funnel, step)
	}
	days, err := s.store.DayStats(dayKey(now.AddDate(0, 0, -13)))
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	for i := 0; i < 14; i++ {
		d := dayKey(now.AddDate(0, 0, -i))
		st := days[d]
		st.Day = d
		v.Days = append(v.Days, st)
	}
	kinds, err := s.store.AIUsageByKind(dayKey(monthStart))
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	var allCalls int64
	for _, k := range []string{"translate", "recall", "explain"} {
		u := kinds[k]
		allCalls += u.Calls
		v.Kinds = append(v.Kinds, adminKindRow{Kind: k, Calls: u.Calls, Cost: usd(s.cost(u))})
	}
	v.CostMonth = usd(s.cost(total))
	if v.Active7d > 0 {
		v.CostPerActive = usd(s.cost(total) / float64(v.Active7d))
	}
	s.render(w, r, "admin.html", "Tableau de bord", v)
}

func usd(x float64) string { return fmt.Sprintf("$%.2f", x) }

// handleAdminPlan switches an account between the free and unlimited plans.
func (s *Server) handleAdminPlan(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(userFromContext(r)) {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	plan := r.FormValue("plan")
	if err != nil || (plan != store.PlanFree && plan != store.PlanUnlimited) {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("plan invalide"))
		return
	}
	if err := s.store.SetPlan(id, plan); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("plan changé", "user", id, "plan", plan)
	http.Redirect(w, r, "/admin#lecteurs", http.StatusSeeOther)
}
