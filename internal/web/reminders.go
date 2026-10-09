package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/romeo4934/ai-reader/internal/auth"
	"github.com/romeo4934/ai-reader/internal/i18n"
	"github.com/romeo4934/ai-reader/internal/store"
)

// The evening reminder: one email a day, from 19:00 in the reader's time
// zone, when today's review challenge isn't done yet — the streak at stake
// is what brings people back. Only for confirmed emails, accounts that
// reviewed in the past week, and never once unsubscribed (one click in
// the email, or the setting).

const (
	reminderHour     = 19
	reminderEvery    = 10 * time.Minute
	reminderActiveIn = 7 // days
	unsubscribeTTL   = 365 * 24 * time.Hour
)

// RunReminders sends due reminders every reminderEvery until ctx ends.
func (s *Server) RunReminders(ctx context.Context) {
	t := time.NewTicker(reminderEvery)
	defer t.Stop()
	for {
		s.sendDueReminders(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Server) sendDueReminders(ctx context.Context, now time.Time) {
	users, err := s.store.ReminderCandidates(dayKey(now.UTC().AddDate(0, 0, -reminderActiveIn)))
	if err != nil {
		s.log.Error("rappels : candidats", "err", err)
		return
	}
	for i := range users {
		if ctx.Err() != nil {
			return
		}
		s.maybeRemind(ctx, &users[i], now)
	}
}

func (s *Server) maybeRemind(ctx context.Context, user *store.User, now time.Time) {
	loc := time.UTC
	if l, err := time.LoadLocation(user.Timezone); user.Timezone != "" && err == nil {
		loc = l
	}
	local := now.In(loc)
	today := dayKey(local)
	if local.Hour() < reminderHour || user.LastReminderDay == today {
		return
	}
	daily, err := s.dailyState(user, local, "")
	if err != nil {
		s.log.Error("rappels : état du jour", "user", user.ID, "err", err)
		return
	}
	if daily.Remaining == 0 {
		return // done (or nothing to do): no email
	}
	// Marked before sending: a failing mail service must not turn into an
	// email every ten minutes.
	if err := s.store.SetLastReminderDay(user.ID, today); err != nil {
		s.log.Error("rappels : marquage", "user", user.ID, "err", err)
		return
	}
	T := i18n.For(user.NativeLang)
	streak := s.streak(user.ID, local)
	subject := fmt.Sprintf(T["MailReminderSubject"], daily.Remaining)
	if streak > 1 {
		subject = fmt.Sprintf(T["MailReminderSubjectStreak"], streak)
	}
	minutes := max(1, (daily.Remaining+3)/4) // ~15 s a card
	unsub := s.cfg.BaseURL + "/unsubscribe?t=" + url.QueryEscape(auth.SignLink(s.secret, auth.PurposeUnsubscribe, user.ID, "", unsubscribeTTL))
	body := fmt.Sprintf(T["MailReminderBody"], daily.Remaining, minutes, s.cfg.BaseURL+"/review", unsub)
	if err := s.mail.Send(ctx, user.Email, subject, body, ""); err != nil {
		s.log.Error("rappels : envoi", "user", user.ID, "err", err)
		return
	}
	s.log.Info("rappel envoyé", "user", user.ID, "cartes", daily.Remaining)
}

func (s *Server) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	token := r.URL.Query().Get("t")
	id, ok := auth.LinkUserID(token)
	if !ok || !auth.VerifyLink(s.secret, auth.PurposeUnsubscribe, token, "") {
		s.renderAccount(w, r, T, "message.html", "ReminderOffTitle", accountView{Error: T["ReminderOffBad"]})
		return
	}
	if err := s.store.SetReminders(id, false); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if u, err := s.store.GetUserByID(id); err == nil {
		T = i18n.For(u.NativeLang)
	}
	s.renderAccount(w, r, T, "message.html", "ReminderOffTitle", accountView{Message: T["ReminderOff"]})
}
