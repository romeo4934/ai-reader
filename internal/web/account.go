package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/romeo4934/ai-reader/internal/auth"
	"github.com/romeo4934/ai-reader/internal/i18n"
	"github.com/romeo4934/ai-reader/internal/store"
)

const (
	verifyTTL = 3 * 24 * time.Hour
	resetTTL  = time.Hour
)

// accountView is the data for every account page; each template uses the
// fields it needs.
type accountView struct {
	Error      string
	Email      string
	Message    string
	Token      string
	ShowResend bool
	FreeNote   string
}

func (s *Server) renderAccount(w http.ResponseWriter, r *http.Request, T i18n.Dict, name, titleKey string, v accountView) {
	s.renderDict(w, r, T, name, T[titleKey], v)
}

// --- login ---

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	s.renderAccount(w, r, s.visitorDict(w, r), "login.html", "AuthLoginTitle", accountView{})
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	login := strings.TrimSpace(r.FormValue("login"))
	password := r.FormValue("password")
	fail := func(msg string, showResend bool) {
		s.renderAccount(w, r, T, "login.html", "AuthLoginTitle", accountView{Error: msg, Email: login, ShowResend: showResend})
	}
	if !s.limit.allow("login:" + clientIP(r)) {
		fail(T["AuthErrTooMany"], false)
		return
	}
	user, err := s.store.GetUserByLogin(login)
	if err != nil || !auth.CheckPassword(user.PasswordHash, password) {
		fail(T["AuthErrBadLogin"], false)
		return
	}
	if user.Email != "" && !user.EmailVerified {
		fail(T["AuthErrUnverified"], true)
		return
	}
	s.issueSession(w, r, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- signup ---

func (s *Server) handleSignupGet(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	s.renderAccount(w, r, T, "signup.html", "AuthSignupTitle", accountView{FreeNote: fmt.Sprintf(T["AuthFreeNote"], s.cfg.FreeQuota)})
}

func (s *Server) handleSignupPost(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	email := normalizeEmail(r.FormValue("email"))
	password := r.FormValue("password")
	fail := func(msg string) {
		s.renderAccount(w, r, T, "signup.html", "AuthSignupTitle", accountView{Error: msg, Email: email, FreeNote: fmt.Sprintf(T["AuthFreeNote"], s.cfg.FreeQuota)})
	}
	switch {
	case !s.limit.allow("signup:" + clientIP(r)):
		fail(T["AuthErrTooMany"])
		return
	case !validEmail(email):
		fail(T["AuthErrBadEmail"])
		return
	case len(password) < 8:
		fail(T["AuthErrShortPassword"])
		return
	}
	if _, err := s.store.GetUserByLogin(email); err == nil {
		fail(T["AuthErrEmailTaken"])
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	userID, err := s.store.CreateEmailUser(email, hash, i18n.NativeLangFor(T["LangCode"]))
	if err != nil {
		// Most likely a double submit racing the check above.
		fail(T["AuthErrEmailTaken"])
		return
	}
	s.sendVerification(r.Context(), T, userID, email)
	s.renderAccount(w, r, T, "check_email.html", "AuthCheckTitle", accountView{Email: email, Message: fmt.Sprintf(T["AuthCheckBody"], email)})
}

func (s *Server) sendVerification(ctx context.Context, T i18n.Dict, userID int64, email string) {
	token := auth.SignLink(s.secret, auth.PurposeVerify, userID, strings.ToLower(email), verifyTTL)
	link := s.cfg.BaseURL + "/verify?t=" + url.QueryEscape(token)
	if err := s.mail.Send(ctx, email, T["MailVerifySubject"], fmt.Sprintf(T["MailVerifyBody"], link), ""); err != nil {
		s.log.Error("envoi email de vérification", "user", userID, "err", err)
	}
}

// --- email verification ---

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	token := r.URL.Query().Get("t")
	bad := func() {
		s.renderAccount(w, r, T, "message.html", "AuthLoginTitle", accountView{Error: T["AuthVerifyBad"]})
	}
	userID, ok := auth.LinkUserID(token)
	if !ok {
		bad()
		return
	}
	user, err := s.store.GetUserByID(userID)
	if err != nil || user.Email == "" || !auth.VerifyLink(s.secret, auth.PurposeVerify, token, strings.ToLower(user.Email)) {
		bad()
		return
	}
	if err := s.store.MarkEmailVerified(user.ID); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.issueSession(w, r, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleVerifyResend answers the same way whether or not the email has an
// unverified account, so it can't be used to probe who's signed up.
func (s *Server) handleVerifyResend(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	email := normalizeEmail(r.FormValue("email"))
	if !s.limit.allow("resend:" + clientIP(r)) {
		s.renderAccount(w, r, T, "login.html", "AuthLoginTitle", accountView{Error: T["AuthErrTooMany"], Email: email})
		return
	}
	if user, err := s.store.GetUserByEmail(email); err == nil && !user.EmailVerified {
		s.sendVerification(r.Context(), T, user.ID, user.Email)
	}
	s.renderAccount(w, r, T, "check_email.html", "AuthCheckTitle", accountView{Email: email, Message: fmt.Sprintf(T["AuthCheckBody"], email)})
}

// --- password reset ---

func (s *Server) handleForgotGet(w http.ResponseWriter, r *http.Request) {
	s.renderAccount(w, r, s.visitorDict(w, r), "forgot.html", "AuthForgotTitle", accountView{Email: r.URL.Query().Get("email")})
}

func (s *Server) handleForgotPost(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	email := normalizeEmail(r.FormValue("email"))
	if !s.limit.allow("forgot:" + clientIP(r)) {
		s.renderAccount(w, r, T, "forgot.html", "AuthForgotTitle", accountView{Error: T["AuthErrTooMany"], Email: email})
		return
	}
	if user, err := s.store.GetUserByEmail(email); err == nil {
		// The reset email goes out in the account's own language, not
		// whatever browser asked for it.
		UT := i18n.For(user.NativeLang)
		token := auth.SignLink(s.secret, auth.PurposeReset, user.ID, user.PasswordHash, resetTTL)
		link := s.cfg.BaseURL + "/reset?t=" + url.QueryEscape(token)
		if err := s.mail.Send(r.Context(), user.Email, UT["MailResetSubject"], fmt.Sprintf(UT["MailResetBody"], link), ""); err != nil {
			s.log.Error("envoi email de réinitialisation", "user", user.ID, "err", err)
		}
	}
	s.renderAccount(w, r, T, "message.html", "AuthForgotTitle", accountView{Message: fmt.Sprintf(T["AuthForgotSent"], email)})
}

// resetUser resolves a reset token to its account, or ok=false if the token
// is forged, expired, or already used (the password changed since).
func (s *Server) resetUser(token string) (store.User, bool) {
	userID, ok := auth.LinkUserID(token)
	if !ok {
		return store.User{}, false
	}
	user, err := s.store.GetUserByID(userID)
	if err != nil || !auth.VerifyLink(s.secret, auth.PurposeReset, token, user.PasswordHash) {
		return store.User{}, false
	}
	return user, true
}

func (s *Server) handleResetGet(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	token := r.URL.Query().Get("t")
	if _, ok := s.resetUser(token); !ok {
		s.renderAccount(w, r, T, "message.html", "AuthResetTitle", accountView{Error: T["AuthResetBad"]})
		return
	}
	s.renderAccount(w, r, T, "reset.html", "AuthResetTitle", accountView{Token: token})
}

func (s *Server) handleResetPost(w http.ResponseWriter, r *http.Request) {
	T := s.visitorDict(w, r)
	token := r.FormValue("t")
	user, ok := s.resetUser(token)
	if !ok {
		s.renderAccount(w, r, T, "message.html", "AuthResetTitle", accountView{Error: T["AuthResetBad"]})
		return
	}
	password := r.FormValue("password")
	if len(password) < 8 {
		s.renderAccount(w, r, T, "reset.html", "AuthResetTitle", accountView{Token: token, Error: T["AuthErrShortPassword"]})
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.SetPasswordHash(user.ID, hash); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	// Clicking the emailed link proves the address too.
	if user.Email != "" {
		if err := s.store.MarkEmailVerified(user.ID); err != nil {
			s.log.Error("vérification email après réinitialisation", "user", user.ID, "err", err)
		}
	}
	s.issueSession(w, r, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- helpers ---

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validEmail(email string) bool {
	if len(email) > 254 || strings.ContainsAny(email, " <>") {
		return false
	}
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email {
		return false
	}
	at := strings.LastIndex(email, "@")
	return at > 0 && strings.Contains(email[at+1:], ".")
}

// clientIP is the visitor's address as nginx saw it: the service only
// listens on loopback, so X-Real-IP always comes from nginx, never the client.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimiter allows `max` events per key in a sliding `window` — enough to
// stop password guessing and signup/email floods from one address, kept in
// memory since a restart forgetting the counts is harmless.
type rateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, hits: map[string][]time.Time{}}
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.max {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	// Drop idle keys now and then so the map doesn't grow forever.
	if len(l.hits) > 10000 {
		for k, ts := range l.hits {
			if len(ts) == 0 || ts[len(ts)-1].Before(cutoff) {
				delete(l.hits, k)
			}
		}
	}
	return true
}
