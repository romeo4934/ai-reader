// Package web is the HTTP surface: library, reader, translate/save APIs used
// from the reading view, and the review deck.
package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/romeo4934/ai-reader/internal/ai"
	"github.com/romeo4934/ai-reader/internal/auth"
	"github.com/romeo4934/ai-reader/internal/epub"
	"github.com/romeo4934/ai-reader/internal/frequency"
	"github.com/romeo4934/ai-reader/internal/i18n"
	"github.com/romeo4934/ai-reader/internal/mail"
	"github.com/romeo4934/ai-reader/internal/srs"
	"github.com/romeo4934/ai-reader/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

const defaultNativeLang = "français"

const maxUploadBytes = 30 << 20 // 30 MiB — plenty for a novel-length epub

// maxVocabWords caps what gets auto-saved as a review card. A short clause
// ("grey-eyed and graceful and slender as a knife") is still worth
// drilling; a full sentence is a comprehension check, not a vocab item.
const maxVocabWords = 12

type Server struct {
	store  *store.Store
	ai     *ai.Client
	tmpl   *template.Template
	log    *slog.Logger
	secret []byte
	mail   *mail.Sender
	cfg    Config
	limit  *rateLimiter
	recall *recallCache
}

// Config holds the settings for open signup.
type Config struct {
	// BaseURL is the public address put in emailed links, without a
	// trailing slash.
	BaseURL string
	// FreeQuota is the number of AI translations a free-plan account gets
	// per calendar month.
	FreeQuota int
}

func New(st *store.Store, aiClient *ai.Client, log *slog.Logger, secret []byte, mailer *mail.Sender, cfg Config) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"highlight": highlightPhrase,
		"freqLabel": frequency.Label,
		"timeAgo":   timeAgo,
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates : %w", err)
	}
	return &Server{store: st, ai: aiClient, tmpl: tmpl, log: log, secret: secret, mail: mailer, cfg: cfg, limit: newRateLimiter(10, 10*time.Minute), recall: newRecallCache()}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /ready", s.handleReady)
	mux.Handle("GET /static/", noCache(http.FileServerFS(staticFS)))

	mux.HandleFunc("GET /login", s.handleLoginGet)
	mux.HandleFunc("POST /login", s.handleLoginPost)
	mux.HandleFunc("GET /signup", s.handleSignupGet)
	mux.HandleFunc("POST /signup", s.handleSignupPost)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /verify", s.handleVerify)
	mux.HandleFunc("POST /verify/resend", s.handleVerifyResend)
	mux.HandleFunc("GET /forgot", s.handleForgotGet)
	mux.HandleFunc("POST /forgot", s.handleForgotPost)
	mux.HandleFunc("GET /reset", s.handleResetGet)
	mux.HandleFunc("POST /reset", s.handleResetPost)

	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("POST /books", s.requireAuth(s.handleUploadBook))
	mux.HandleFunc("GET /books/{id}", s.requireAuth(s.handleReader))

	mux.HandleFunc("GET /review", s.requireAuth(s.handleReviewPage))
	mux.HandleFunc("POST /review/{id}/answer", s.requireAuth(s.handleReviewAnswer))
	mux.HandleFunc("POST /review/more", s.requireAuth(s.handleReviewMore))
	mux.HandleFunc("GET /leaderboard", s.requireAuth(s.handleLeaderboard))
	mux.HandleFunc("GET /friends", s.requireAuth(s.handleFriends))
	mux.HandleFunc("POST /friends/invite", s.requireAuth(s.handleFriendsInvite))
	mux.HandleFunc("POST /friends/{id}/remove", s.requireAuth(s.handleFriendRemove))
	mux.HandleFunc("GET /join", s.handleJoin)
	mux.HandleFunc("POST /join", s.requireAuth(s.handleJoinPost))

	mux.HandleFunc("GET /words", s.requireAuth(s.handleWords))
	mux.HandleFunc("GET /reviewed", s.requireAuth(s.handleReviewed))
	mux.HandleFunc("POST /words/{id}/archive", s.requireAuth(s.handleArchiveWord))
	mux.HandleFunc("POST /words/{id}/delete", s.requireAuth(s.handleDeleteWord))

	mux.HandleFunc("GET /settings", s.requireAuth(s.handleSettingsGet))
	mux.HandleFunc("POST /settings", s.requireAuth(s.handleSettingsPost))
	mux.HandleFunc("POST /settings/email", s.requireAuth(s.handleSettingsEmail))

	mux.HandleFunc("POST /api/translate", s.requireAuth(s.handleAPITranslate))
	mux.HandleFunc("POST /api/explain", s.requireAuth(s.handleAPIExplain))

	return mux
}

// --- auth ---

type ctxKey int

const ctxUser ctxKey = 0

func userFromContext(r *http.Request) *store.User {
	u, _ := r.Context().Value(ctxUser).(*store.User)
	return u
}

// requireAuth resolves the session cookie to a user and makes it available
// via userFromContext, or redirects to /login (GET) / 401s (everything
// else, i.e. the JSON API) when there's no valid session.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r, ok := s.withSessionUser(r)
		if !ok {
			s.unauthenticated(w, r)
			return
		}
		next(w, r)
	}
}

// withSessionUser attaches the logged-in user to the request context, if the
// session cookie is valid; ok is false for a visitor without a session.
func (s *Server) withSessionUser(r *http.Request) (*http.Request, bool) {
	c, err := r.Cookie(auth.SessionCookie)
	if err != nil {
		return r, false
	}
	userID, ok := auth.Verify(s.secret, c.Value)
	if !ok {
		return r, false
	}
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return r, false
	}
	return r.WithContext(context.WithValue(r.Context(), ctxUser, &user)), true
}

// handleHome serves the library to a logged-in user and the landing page to
// everyone else — / is the address people share, so it has to explain the
// app to a visitor instead of bouncing them to /login.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r, ok := s.withSessionUser(r); ok {
		s.handleLibrary(w, r)
		return
	}
	T := s.visitorDict(w, r)
	s.renderDict(w, r, T, "landing.html", T["LandTitle"], landingView{Languages: i18n.Languages})
}

const langCookie = "lang"

// visitorDict picks the UI language for someone not logged in: an explicit
// ?lang= from the landing page's switcher (remembered in a cookie), else
// that cookie, else the browser's Accept-Language.
func (s *Server) visitorDict(w http.ResponseWriter, r *http.Request) i18n.Dict {
	T, ok := i18n.ByCode(r.URL.Query().Get("lang"))
	if ok {
		http.SetCookie(w, &http.Cookie{Name: langCookie, Value: T["LangCode"], Path: "/", MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode})
	} else if c, err := r.Cookie(langCookie); err == nil {
		T, ok = i18n.ByCode(c.Value)
	}
	if !ok {
		T = i18n.ForVisitor(r.Header.Get("Accept-Language"))
	}
	w.Header().Set("Vary", "Accept-Language, Cookie")
	return T
}

type landingView struct {
	Languages []struct{ Code, Name string }
	// InviterName is set when the visitor came through a friend's invitation.
	InviterName string
}

func landingLanguages() []struct{ Code, Name string } { return i18n.Languages }

func (s *Server) unauthenticated(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/api/") {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	http.Error(w, "non connecté", http.StatusUnauthorized)
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, userID int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    auth.Sign(s.secret, userID),
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(auth.SessionTTL),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

// --- pages ---

type pageData struct {
	Title    string
	DueCount int
	LoggedIn bool
	T        i18n.Dict
	Data     any
}

// dictFor resolves the UI language to use for this request: the logged-in
// user's native_lang setting, or French if there's no user yet (login,
// signup — there's nothing to read a preference from before they exist).
func (s *Server) dictFor(r *http.Request) i18n.Dict {
	user := userFromContext(r)
	nativeLang := ""
	if user != nil {
		nativeLang = user.NativeLang
	}
	return i18n.For(nativeLang)
}

// render looks the user up from the request itself (rather than taking it as
// a parameter) so every call site — including the unauthenticated login and
// signup pages — stays uniform. title is literal text (most callers build it
// from dictFor(r), except the reader view, whose title is a book's own name
// — data, not a UI string, so it's never translated).
func (s *Server) render(w http.ResponseWriter, r *http.Request, name, title string, data any) {
	s.renderDict(w, r, s.dictFor(r), name, title, data)
}

// renderDict is render with the UI dictionary chosen by the caller — the
// landing page picks it from the visitor's browser instead of a user setting.
func (s *Server) renderDict(w http.ResponseWriter, r *http.Request, T i18n.Dict, name, title string, data any) {
	pd := pageData{Title: title, T: T, Data: data}
	if user := userFromContext(r); user != nil {
		// The badge is what's left of today's challenge, not the raw due
		// count: new cards beyond the daily limit aren't today's job.
		daily, err := s.dailyState(user, time.Now().In(userLocation(r)))
		if err != nil {
			s.log.Error("daily state", "err", err)
		}
		pd.DueCount = daily.Remaining
		pd.LoggedIn = true
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, pd); err != nil {
		s.log.Error("render template", "template", name, "err", err)
	}
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	books, err := s.store.ListBooks(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.render(w, r, "library.html", s.dictFor(r)["LibTitle"], books)
}

func (s *Server) handleUploadBook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("fichier trop volumineux ou requête invalide : %w", err))
		return
	}
	file, header, err := r.FormFile("epub")
	if err != nil {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("aucun fichier reçu : %w", err))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	book, err := epub.Parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("epub illisible (%s) : %w", header.Filename, err))
		return
	}

	chapters := make([]struct{ Title, Content string }, len(book.Chapters))
	for i, ch := range book.Chapters {
		chapters[i] = struct{ Title, Content string }{ch.Title, ch.Content}
	}
	user := userFromContext(r)
	bookID, err := s.store.InsertBook(user.ID, book.Title, book.Author, book.Language, chapters)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/books/%d", bookID), http.StatusSeeOther)
}

type readerView struct {
	Book           store.Book
	Chapter        store.Chapter
	Paragraphs     []string
	NativeLang     string
	HasPrev        bool // an earlier chapter exists
	HasNext        bool // a later chapter exists
	ChapterIdx     int
	SectionIdx     int
	SectionCount   int
	HasPrevSection bool
	HasNextSection bool
}

// sectionTargetChars caps how much text shows on screen at once — a whole
// chapter was too much to track your place in by scroll alone. Paragraphs
// are never split (so a section boundary is always between sentences, never
// inside one): a section just keeps absorbing whole paragraphs until the
// next one would push it past the target, then starts a new section there.
const sectionTargetChars = 1200

func splitIntoSections(paragraphs []string, target int) [][]string {
	var sections [][]string
	var cur []string
	curLen := 0
	for _, p := range paragraphs {
		if curLen > 0 && curLen+len(p) > target {
			sections = append(sections, cur)
			cur = nil
			curLen = 0
		}
		cur = append(cur, p)
		curLen += len(p)
	}
	if len(cur) > 0 {
		sections = append(sections, cur)
	}
	if len(sections) == 0 {
		sections = [][]string{{}}
	}
	return sections
}

func (s *Server) handleReader(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	bookID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	book, err := s.store.GetBook(bookID, user.ID)
	if err != nil {
		s.fail(w, http.StatusNotFound, fmt.Errorf("livre introuvable : %w", err))
		return
	}

	chQ := r.URL.Query().Get("ch")
	chIdx, secIdx := 0, 0
	if chQ != "" {
		chIdx, _ = strconv.Atoi(chQ)
	} else if savedCh, savedSec, err := s.store.GetProgress(bookID); err == nil {
		chIdx, secIdx = savedCh, savedSec
	}
	if chIdx < 0 {
		chIdx = 0
	}
	if chIdx > book.ChapterCount-1 {
		chIdx = book.ChapterCount - 1
	}

	chapter, err := s.store.GetChapterByIdx(bookID, chIdx)
	if err != nil {
		s.fail(w, http.StatusNotFound, fmt.Errorf("chapitre introuvable : %w", err))
		return
	}

	sections := splitIntoSections(strings.Split(chapter.Content, "\n\n"), sectionTargetChars)
	if q := r.URL.Query().Get("sec"); q != "" {
		secIdx, _ = strconv.Atoi(q)
	} else if r.URL.Query().Get("enter") == "end" {
		secIdx = len(sections) - 1
	} else if chQ != "" {
		secIdx = 0 // explicit jump to a different chapter — not a resume, start at its top
	}
	if secIdx < 0 {
		secIdx = 0
	}
	if secIdx > len(sections)-1 {
		secIdx = len(sections) - 1
	}
	_ = s.store.SetProgress(bookID, chIdx, secIdx)

	native := user.NativeLang
	if native == "" {
		native = defaultNativeLang
	}

	s.render(w, r, "reader.html", book.Title, readerView{
		Book:           book,
		Chapter:        chapter,
		Paragraphs:     sections[secIdx],
		NativeLang:     native,
		HasPrev:        chIdx > 0,
		HasNext:        chIdx < book.ChapterCount-1,
		ChapterIdx:     chIdx,
		SectionIdx:     secIdx,
		SectionCount:   len(sections),
		HasPrevSection: secIdx > 0,
		HasNextSection: secIdx < len(sections)-1,
	})
}

// --- review ---

type reviewView struct {
	Daily  dailyState
	Streak int
	// StreakText is empty while there's no streak to show.
	StreakText string
	// Finished: nothing left today in a non-empty deck — the challenge is done.
	Finished bool
	// PointsDone is the "done" screen's points line (today and total).
	PointsDone string
	Card       *store.Vocab
	IsNew      bool
	IsRetry    bool // missed earlier today, back for another go
	// Recall is nil when generation failed or no API key is set — the
	// template falls back to the plain translation-reveal card.
	Recall *ai.RecallCard
}

func (s *Server) handleReviewPage(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	T := s.dictFor(r)
	now := time.Now().In(userLocation(r))
	daily, err := s.dailyState(user, now)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	view := reviewView{Daily: daily}
	if daily.Remaining > 0 {
		cards, err := s.store.NextDailyCards(user.ID, now.UTC(), dayStart(now), daily.NewLeft > 0, 1+recallPrefetch)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		if len(cards) > 0 {
			view.Card = &cards[0]
			view.IsNew = view.Card.LastReviewedAt == nil
			view.IsRetry = !view.IsNew && !view.Card.LastReviewedAt.Before(dayStart(now))
			// Start on the next cards' exercises before waiting on this
			// one, so they're ready by the time this card is answered.
			s.prefetchRecallCards(user, cards, daily.NewLeft)
			view.Recall = s.recallCardFor(r.Context(), user, view.Card)
		}
	}
	if view.Card == nil && daily.DeckSize > 0 {
		// Also counts a day with nothing due: the reader showed up, and
		// having nothing to do isn't a reason to lose the streak.
		view.Finished = true
		if err := s.store.MarkDayCompleted(user.ID, daily.Day); err != nil {
			s.log.Error("mark day completed", "err", err)
		}
	}
	if view.Finished {
		if total, err := s.store.TotalPoints(user.ID); err != nil {
			s.log.Error("total points", "err", err)
		} else if total > 0 {
			view.PointsDone = fmt.Sprintf(T["DailyPointsDone"], daily.Points, total)
		}
	}
	view.Streak = s.streak(user.ID, now)
	if view.Streak > 0 {
		view.StreakText = streakText(T, view.Streak)
	}
	s.render(w, r, "review.html", T["ReviewTitle"], view)
}

// generateRecallCard asks Claude for a fresh fill-in-the-blank exercise for
// this card. Returns nil on any failure (no API key, network error, refusal)
// so the review page falls back to the static context+translation card
// instead of breaking review entirely. The review page goes through
// recallCardFor, which serves an exercise prepared in advance when there is one.
func (s *Server) generateRecallCard(ctx context.Context, user *store.User, card *store.Vocab) *ai.RecallCard {
	book, err := s.store.GetBook(card.BookID, user.ID)
	bookLang := ""
	if err == nil {
		bookLang = book.Language
	}
	native := user.NativeLang
	if native == "" {
		native = defaultNativeLang
	}
	lemma := card.Lemma
	if strings.TrimSpace(lemma) == "" {
		lemma = card.Phrase
	}

	callCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	start := time.Now()
	defer func() { s.log.Info("recall card", "vocab_id", card.ID, "ms", time.Since(start).Milliseconds()) }()
	rc, err := s.ai.RecallCard(callCtx, ai.RecallCardOptions{
		BookLanguage: bookLang,
		NativeLang:   native,
		Lemma:        lemma,
		Translation:  card.Translation,
	})
	if err != nil {
		s.log.Warn("recall card generation failed, falling back to static card", "vocab_id", card.ID, "err", err)
		return nil
	}
	return &rc
}

func (s *Server) handleReviewAnswer(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	result := srs.Result(r.FormValue("result"))
	if result != srs.Good {
		result = srs.Again
	}
	points := answerPoints(result, r.FormValue("mode"))

	card, err := s.store.GetVocab(id, user.ID)
	if err != nil {
		s.fail(w, http.StatusNotFound, err)
		return
	}
	now := time.Now().In(userLocation(r))
	nextBox, next := nextReview(card, result, now)
	if err := s.store.UpdateVocabReview(id, user.ID, nextBox, next, now.UTC()); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	lang := ""
	if book, err := s.store.GetBook(card.BookID, user.ID); err == nil {
		lang = book.Language
	}
	if err := s.store.RecordReview(user.ID, dayKey(now), card.LastReviewedAt == nil, points, lang); err != nil {
		s.log.Error("record review", "err", err)
	}
	http.Redirect(w, r, "/review", http.StatusSeeOther)
}

func (s *Server) handleWords(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	words, err := s.store.ListVocab(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	T := s.dictFor(r)
	total, err := s.store.TotalPoints(user.ID)
	if err != nil {
		s.log.Error("total points", "err", err)
	}
	view := wordsView{Words: words, Groups: groupByFrequency(T, words)}
	if waiting, learning, known, err := s.store.VocabProgress(user.ID); err != nil {
		s.log.Error("vocab progress", "err", err)
	} else if n := waiting + learning + known; n > 0 {
		view.Progress = &wordsProgress{
			Line:       fmt.Sprintf(T["WordsProgressLine"], learning, known, waiting),
			LearnPct:   learning * 100 / n,
			KnownPct:   known * 100 / n,
			WaitingPct: 100 - learning*100/n - known*100/n,
		}
	}
	if total > 0 {
		view.Points = fmt.Sprintf(T["WordsPointsTotal"], total)
	}
	s.render(w, r, "words.html", T["WordsTitle"], view)
}

type wordsView struct {
	Words    []store.Vocab
	Groups   []wordGroup
	Points   string // empty until the first point is earned
	Progress *wordsProgress
}

// frequencyLevels are the upper corpus ranks of the 10 levels the word list
// is grouped in, most useful words first. The last level takes the rest.
var frequencyLevels = []int{100, 300, 600, 1000, 2000, 3000, 5000, 8000, 15000}

type wordGroup struct {
	Level int
	Label string
	Count int
	Words []store.Vocab
}

// groupByFrequency splits the list (already sorted by frequency rank) into
// the 10 levels, skipping empty ones.
func groupByFrequency(T i18n.Dict, words []store.Vocab) []wordGroup {
	var groups []wordGroup
	for _, w := range words {
		level := len(frequencyLevels) + 1
		for i, upTo := range frequencyLevels {
			if w.Frequency <= upTo {
				level = i + 1
				break
			}
		}
		if len(groups) == 0 || groups[len(groups)-1].Level != level {
			label := T["WordsLevelRare"]
			if level <= len(frequencyLevels) {
				label = fmt.Sprintf(T["WordsLevelTop"], frequencyLevels[level-1])
			}
			groups = append(groups, wordGroup{Level: level, Label: label})
		}
		g := &groups[len(groups)-1]
		g.Words = append(g.Words, w)
		g.Count++
	}
	return groups
}

type wordsProgress struct {
	Line                           string
	KnownPct, LearnPct, WaitingPct int
}

func (s *Server) handleArchiveWord(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	now := time.Now().UTC()
	box, nextReview := srs.Next(srs.MaxBox, srs.Good, now) // "known" = mastered, box 5
	if err := s.store.ArchiveVocab(id, user.ID, box, nextReview, now); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/words", http.StatusSeeOther)
}

func (s *Server) handleDeleteWord(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteVocab(id, user.ID); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	// Deletable from both /words and a /review card; go back where the
	// button was, but only to one of those two — never an arbitrary URL.
	back := "/words"
	if r.FormValue("back") == "/review" {
		back = "/review"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) handleReviewed(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	words, err := s.store.RecentlyReviewed(user.ID, 50)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.render(w, r, "reviewed.html", s.dictFor(r)["ReviewedTitle"], words)
}

// --- settings ---

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	native := user.NativeLang
	if native == "" {
		native = defaultNativeLang
	}
	T := s.dictFor(r)
	used, err := s.store.TranslationsThisMonth(user.ID, time.Now())
	if err != nil {
		s.log.Error("usage", "err", err)
	}
	usage := fmt.Sprintf(T["SettingsUsage"], used, s.quotaFor(user))
	if user.Plan == store.PlanUnlimited {
		usage = fmt.Sprintf(T["SettingsUsageUnlimited"], used)
	}
	s.render(w, r, "settings.html", T["SettingsTitle"], settingsView{
		NativeLang: native, Username: user.Username, Usage: usage,
		DailyNewLimit: dailyNewLimit(user), DailyNewLimits: DailyNewLimits,
		DisplayName: user.DisplayName, PublicName: publicName(user.ID, user.Username, user.DisplayName),
		Email: user.Email, PendingEmail: user.PendingEmail, Message: r.URL.Query().Get("msg"),
	})
}

type settingsView struct {
	NativeLang   string
	Username     string
	Usage        string
	Email        string
	PendingEmail string
	Message      string

	DailyNewLimit  int
	DailyNewLimits []int
	DisplayName    string
	PublicName     string // what others see when DisplayName is empty
}

func (s *Server) handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if err := r.ParseForm(); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	native := strings.TrimSpace(r.FormValue("native_lang"))
	if native == "" {
		native = defaultNativeLang
	}
	if err := s.store.SetUserNativeLang(user.ID, native); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if name, ok := cleanDisplayName(r.FormValue("display_name")); ok {
		if err := s.store.SetDisplayName(user.ID, name); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	if n, err := strconv.Atoi(r.FormValue("daily_new_limit")); err == nil && slices.Contains(DailyNewLimits, n) {
		if err := s.store.SetDailyNewLimit(user.ID, n); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// --- JSON APIs used from the reading view ---

type translateRequest struct {
	BookID    int64  `json:"book_id"`
	ChapterID int64  `json:"chapter_id"`
	Phrase    string `json:"phrase"`
	Context   string `json:"context"`
}

type translateResponse struct {
	ai.Translation
	Saved bool `json:"saved"` // false when this phrase was already in the deck
}

// overQuota reports whether a free-plan user has used up this month's AI
// calls (translations and explanations).
func (s *Server) overQuota(user *store.User, now time.Time) (bool, error) {
	if user.Plan == store.PlanUnlimited {
		return false, nil
	}
	used, err := s.store.TranslationsThisMonth(user.ID, now)
	return used >= s.quotaFor(user), err
}

// quotaFor is a free-plan user's monthly AI calls, referral bonus included.
func (s *Server) quotaFor(user *store.User) int {
	return s.cfg.FreeQuota + user.BonusQuota
}

// handleAPITranslate translates the selection and saves it as a review card
// in the same call — a lookup while reading is treated as "I want to learn
// this", so there's no separate save step for the reader to remember.
func (s *Server) handleAPITranslate(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	var req translateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.failJSON(w, http.StatusBadRequest, err)
		return
	}
	phrase := strings.TrimSpace(req.Phrase)
	if phrase == "" {
		s.failJSON(w, http.StatusBadRequest, errors.New("phrase vide"))
		return
	}
	book, err := s.store.GetBook(req.BookID, user.ID)
	if err != nil {
		s.failJSON(w, http.StatusNotFound, err)
		return
	}
	native := user.NativeLang
	if native == "" {
		native = defaultNativeLang
	}
	now := time.Now()
	if over, err := s.overQuota(user, now); err != nil {
		s.failJSON(w, http.StatusInternalServerError, err)
		return
	} else if over {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": fmt.Sprintf(s.dictFor(r)["QuotaReached"], s.quotaFor(user))})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	tr, err := s.ai.Translate(ctx, ai.TranslateOptions{
		BookLanguage: book.Language,
		NativeLang:   native,
		Context:      req.Context,
		Phrase:       phrase,
	})
	if err != nil {
		if errors.Is(err, ai.ErrNoKey) {
			s.failJSON(w, http.StatusServiceUnavailable, errors.New("ANTHROPIC_API_KEY absente sur le serveur"))
			return
		}
		s.failJSON(w, http.StatusBadGateway, err)
		return
	}

	// Counted for every plan, so the numbers are there when pricing gets
	// decided; only enforced above for the free one.
	if err := s.store.CountTranslation(user.ID, now); err != nil {
		s.log.Error("count translation", "err", err)
	}

	_, alreadySaved, err := s.store.FindVocab(user.ID, phrase, tr.Lemma)
	if err != nil {
		s.failJSON(w, http.StatusInternalServerError, err)
		return
	}
	// A deliberate clause selection (checking syntax, not just a word) is
	// still worth translating and is handled above — but past a certain
	// length it's a whole sentence, not a vocab item, and saving it just
	// clutters the deck (especially the growing near-duplicates a slow
	// touch-drag can produce despite the selection debounce).
	tooLongForVocab := len(strings.Fields(phrase)) > maxVocabWords
	if !alreadySaved && !tooLongForVocab {
		freq := tr.Frequency
		if rank, found := frequency.Rank(book.Language, tr.Lemma); found {
			freq = rank
		} else {
			freq = frequency.FallbackFromEstimate(tr.Frequency)
		}
		if _, err := s.store.InsertVocab(store.Vocab{
			UserID:      user.ID,
			BookID:      req.BookID,
			ChapterID:   req.ChapterID,
			Phrase:      phrase,
			Lemma:       tr.Lemma,
			Context:     req.Context,
			Translation: tr.Translation,
			Note:        tr.Note,
			Frequency:   freq,
		}); err != nil {
			s.failJSON(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, translateResponse{Translation: tr, Saved: !alreadySaved && !tooLongForVocab})
}

// --- helpers ---

func (s *Server) fail(w http.ResponseWriter, status int, err error) {
	s.log.Error("http", "status", status, "err", err)
	http.Error(w, err.Error(), status)
}

func (s *Server) failJSON(w http.ResponseWriter, status int, err error) {
	s.log.Error("api", "status", status, "err", err)
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// noCache forces a fresh fetch on every request for static assets. embed.FS
// files carry no real mtime, so the browser's default HTTP caching has
// nothing to revalidate against and was serving stale CSS/JS straight from
// cache after a deploy — a hard refresh was the only way to see a fix.
// These files are tiny, so there's no real cost to never caching them.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// highlightPhrase wraps the first occurrence of phrase inside context with
// <mark>, escaping both pieces itself so it's safe to mark as template.HTML.
// timeAgo renders a rough, French, human time distance — good enough for an
// activity list where the exact minute never matters.
func timeAgo(t *time.Time) string {
	if t == nil {
		return ""
	}
	d := time.Since(*t)
	switch {
	case d < time.Minute:
		return "à l'instant"
	case d < time.Hour:
		return fmt.Sprintf("il y a %dmin", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("il y a %dh", int(d.Hours()))
	case d < 48*time.Hour:
		return "hier"
	default:
		return fmt.Sprintf("il y a %dj", int(d.Hours()/24))
	}
}

func highlightPhrase(context, phrase string) template.HTML {
	if phrase == "" {
		return template.HTML(template.HTMLEscapeString(context))
	}
	idx := strings.Index(context, phrase)
	if idx < 0 {
		return template.HTML(template.HTMLEscapeString(context))
	}
	before := template.HTMLEscapeString(context[:idx])
	match := template.HTMLEscapeString(context[idx : idx+len(phrase)])
	after := template.HTMLEscapeString(context[idx+len(phrase):])
	return template.HTML(before + "<mark>" + match + "</mark>" + after)
}
