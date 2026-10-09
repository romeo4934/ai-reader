package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/romeo4934/ai-reader/internal/auth"
	"github.com/romeo4934/ai-reader/internal/store"
)

// Friends and invitations. Each reader has a personal invitation link (a
// signed token naming them). Opening it while logged in offers to become
// friends; opening it logged out remembers the inviter in a cookie, so the
// account created next is linked to them — and once its email is
// confirmed, both become friends and get the referral bonus. Invitations
// can also be sent by email, with limits so Lydi can't be used to spam.

const (
	inviteCookie   = "invite"
	inviteTTL      = 60 * 24 * time.Hour
	invitesPerDay  = 10
	reinviteWindow = 7 * 24 * time.Hour
)

func (s *Server) inviteLink(userID int64) string {
	return s.cfg.BaseURL + "/join?i=" + url.QueryEscape(auth.SignLink(s.secret, auth.PurposeInvite, userID, "", inviteTTL))
}

// inviter resolves an invitation token to the user who sent it.
func (s *Server) inviter(token string) (store.User, bool) {
	id, ok := auth.LinkUserID(token)
	if !ok || !auth.VerifyLink(s.secret, auth.PurposeInvite, token, "") {
		return store.User{}, false
	}
	u, err := s.store.GetUserByID(id)
	return u, err == nil
}

// pendingInviter is the inviter remembered from an invitation link opened
// before logging in or signing up.
func (s *Server) pendingInviter(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(inviteCookie)
	if err != nil {
		return store.User{}, false
	}
	return s.inviter(c.Value)
}

func clearInviteCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: inviteCookie, Value: "", Path: "/", MaxAge: -1})
}

// --- /friends ---

type friendsView struct {
	Link        string
	ShareText   string
	NeedName    bool // no pseudo yet: the invite email would name a "Panda 42"
	DisplayName string
	Message     string
	Error       string
	Friends     []friendRow
	Invites     []store.Invite
	Bonus       int
}

type friendRow struct {
	ID   int64
	Name string
}

func (s *Server) handleFriends(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	T := s.dictFor(r)
	v := friendsView{
		Link:        s.inviteLink(user.ID),
		ShareText:   T["FriendsShareText"],
		NeedName:    user.DisplayName == "" && user.Email != "",
		DisplayName: user.DisplayName,
		Message:     r.URL.Query().Get("msg"),
		Error:       r.URL.Query().Get("err"),
		Bonus:       user.BonusQuota,
	}
	friends, err := s.store.Friends(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	for _, f := range friends {
		v.Friends = append(v.Friends, friendRow{ID: f.ID, Name: publicName(f.ID, f.Username, f.DisplayName)})
	}
	if v.Invites, err = s.store.ListInvites(user.ID, 20); err != nil {
		s.log.Error("list invites", "err", err)
	}
	s.render(w, r, "friends.html", T["FriendsTitle"], v)
}

func (s *Server) handleFriendsInvite(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	T := s.dictFor(r)
	back := func(key, val string) {
		http.Redirect(w, r, "/friends?"+key+"="+url.QueryEscape(val), http.StatusSeeOther)
	}
	if raw := r.FormValue("display_name"); raw != "" {
		name, ok := cleanDisplayName(raw)
		if ok && name != "" {
			if err := s.store.SetDisplayName(user.ID, name); err != nil {
				s.fail(w, http.StatusInternalServerError, err)
				return
			}
			user.DisplayName = name
		}
	}
	if user.DisplayName == "" && user.Email != "" {
		back("err", T["FriendsErrName"])
		return
	}
	email := normalizeEmail(r.FormValue("email"))
	now := time.Now()
	if !validEmail(email) {
		back("err", T["AuthErrBadEmail"])
		return
	}
	if email == normalizeEmail(user.Email) {
		back("err", T["FriendsErrSelf"])
		return
	}
	if n, err := s.store.CountInvitesSince(user.ID, now.Add(-24*time.Hour)); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	} else if n >= invitesPerDay {
		back("err", fmt.Sprintf(T["FriendsErrLimit"], invitesPerDay))
		return
	}
	if _, err := s.store.GetUserByLogin(email); err == nil {
		back("err", T["FriendsErrUser"])
		return
	}
	if recent, err := s.store.InvitedSince(email, now.Add(-reinviteWindow)); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	} else if recent {
		back("err", T["FriendsErrRecent"])
		return
	}
	if err := s.store.CreateInvite(user.ID, email); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	name := publicName(user.ID, user.Username, user.DisplayName)
	body := fmt.Sprintf(T["MailInviteBody"], name, s.inviteLink(user.ID), name)
	if err := s.mail.Send(r.Context(), email, fmt.Sprintf(T["MailInviteSubject"], name), body, ""); err != nil {
		s.log.Error("envoi invitation", "user", user.ID, "err", err)
		back("err", T["FriendsErrSend"])
		return
	}
	back("msg", fmt.Sprintf(T["FriendsSent"], email))
}

func (s *Server) handleFriendRemove(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.RemoveFriendship(user.ID, id); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}

// --- /join ---

type joinView struct {
	Token       string
	InviterName string
	Self        bool
	Already     bool
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("i")
	inviter, ok := s.inviter(token)
	r, loggedIn := s.withSessionUser(r)
	if !loggedIn {
		T := s.visitorDict(w, r)
		v := landingView{Languages: landingLanguages()}
		if ok {
			http.SetCookie(w, &http.Cookie{
				Name: inviteCookie, Value: token, Path: "/", MaxAge: int((30 * 24 * time.Hour).Seconds()),
				HttpOnly: true, Secure: r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteLaxMode,
			})
			v.InviterName = publicName(inviter.ID, inviter.Username, inviter.DisplayName)
		}
		s.renderDict(w, r, T, "landing.html", T["LandTitle"], v)
		return
	}
	user := userFromContext(r)
	T := s.dictFor(r)
	if !ok {
		s.render(w, r, "join.html", T["JoinTitle"], joinView{})
		return
	}
	v := joinView{Token: token, InviterName: publicName(inviter.ID, inviter.Username, inviter.DisplayName), Self: inviter.ID == user.ID}
	if !v.Self {
		var err error
		if v.Already, err = s.store.AreFriends(user.ID, inviter.ID); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	s.render(w, r, "join.html", T["JoinTitle"], v)
}

func (s *Server) handleJoinPost(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	inviter, ok := s.inviter(r.FormValue("i"))
	if !ok {
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
		return
	}
	if err := s.store.AddFriendship(user.ID, inviter.ID); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	clearInviteCookie(w)
	http.Redirect(w, r, "/leaderboard?scope=friends", http.StatusSeeOther)
}

// linkPendingInvite befriends the inviter remembered in the cookie when an
// existing account logs in from an invitation (no bonus: that's for
// bringing in new readers).
func (s *Server) linkPendingInvite(w http.ResponseWriter, r *http.Request, userID int64) {
	inviter, ok := s.pendingInviter(r)
	if !ok {
		return
	}
	if inviter.ID != userID {
		if err := s.store.AddFriendship(userID, inviter.ID); err != nil {
			s.log.Error("ami depuis invitation", "err", err)
		}
	}
	clearInviteCookie(w)
}
