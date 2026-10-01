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
	"strconv"
	"strings"
	"time"

	"github.com/romeo4934/ai-reader/internal/ai"
	"github.com/romeo4934/ai-reader/internal/auth"
	"github.com/romeo4934/ai-reader/internal/epub"
	"github.com/romeo4934/ai-reader/internal/frequency"
	"github.com/romeo4934/ai-reader/internal/i18n"
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
	store      *store.Store
	ai         *ai.Client
	tmpl       *template.Template
	log        *slog.Logger
	secret     []byte
	inviteCode string
}

func New(st *store.Store, aiClient *ai.Client, log *slog.Logger, secret []byte, inviteCode string) (*Server, error) {
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
	return &Server{store: st, ai: aiClient, tmpl: tmpl, log: log, secret: secret, inviteCode: inviteCode}, nil
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

	mux.HandleFunc("GET /{$}", s.requireAuth(s.handleLibrary))
	mux.HandleFunc("POST /books", s.requireAuth(s.handleUploadBook))
	mux.HandleFunc("GET /books/{id}", s.requireAuth(s.handleReader))

	mux.HandleFunc("GET /review", s.requireAuth(s.handleReviewPage))
	mux.HandleFunc("POST /review/{id}/answer", s.requireAuth(s.handleReviewAnswer))

	mux.HandleFunc("GET /words", s.requireAuth(s.handleWords))
	mux.HandleFunc("GET /reviewed", s.requireAuth(s.handleReviewed))
	mux.HandleFunc("POST /words/{id}/archive", s.requireAuth(s.handleArchiveWord))

	mux.HandleFunc("GET /settings", s.requireAuth(s.handleSettingsGet))
	mux.HandleFunc("POST /settings", s.requireAuth(s.handleSettingsPost))

	mux.HandleFunc("POST /api/translate", s.requireAuth(s.handleAPITranslate))

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
		c, err := r.Cookie(auth.SessionCookie)
		if err != nil {
			s.unauthenticated(w, r)
			return
		}
		userID, ok := auth.Verify(s.secret, c.Value)
		if !ok {
			s.unauthenticated(w, r)
			return
		}
		user, err := s.store.GetUserByID(userID)
		if err != nil {
			s.unauthenticated(w, r)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxUser, &user)))
	}
}

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

type authPageView struct {
	Error string
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login.html", "Connexion", authPageView{})
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	user, err := s.store.GetUserByUsername(username)
	if err != nil || !auth.CheckPassword(user.PasswordHash, password) {
		s.render(w, r, "login.html", "Connexion", authPageView{Error: "identifiant ou mot de passe incorrect"})
		return
	}
	s.issueSession(w, r, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleSignupGet(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "signup.html", "Créer un compte", authPageView{})
}

func (s *Server) handleSignupPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	invite := r.FormValue("invite_code")

	fail := func(msg string) {
		s.render(w, r, "signup.html", "Créer un compte", authPageView{Error: msg})
	}
	switch {
	case s.inviteCode == "":
		fail("inscriptions désactivées (pas de code d'invitation configuré sur le serveur)")
		return
	case invite != s.inviteCode:
		fail("code d'invitation incorrect")
		return
	case len(username) < 3:
		fail("identifiant trop court (3 caractères minimum)")
		return
	case len(password) < 8:
		fail("mot de passe trop court (8 caractères minimum)")
		return
	}
	if _, err := s.store.GetUserByUsername(username); err == nil {
		fail("cet identifiant est déjà pris")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	userID, err := s.store.CreateUser(username, hash, defaultNativeLang)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.issueSession(w, r, userID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
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
	T := s.dictFor(r)
	pd := pageData{Title: title, T: T, Data: data}
	if user := userFromContext(r); user != nil {
		due, err := s.store.CountDueVocab(user.ID, time.Now().UTC())
		if err != nil {
			s.log.Error("count due vocab", "err", err)
		}
		pd.DueCount = due
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
	Card *store.Vocab
	// Recall is nil when generation failed or no API key is set — the
	// template falls back to the plain translation-reveal card.
	Recall *ai.RecallCard
}

func (s *Server) handleReviewPage(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	cards, err := s.store.DueVocab(user.ID, time.Now().UTC(), 1)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	view := reviewView{}
	if len(cards) > 0 {
		view.Card = &cards[0]
		view.Recall = s.generateRecallCard(r.Context(), user, view.Card)
	}
	s.render(w, r, "review.html", s.dictFor(r)["ReviewTitle"], view)
}

// generateRecallCard asks Claude for a fresh fill-in-the-blank exercise for
// this card. Returns nil on any failure (no API key, network error, refusal)
// so the review page falls back to the static context+translation card
// instead of breaking review entirely.
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

	card, err := s.store.GetVocab(id, user.ID)
	if err != nil {
		s.fail(w, http.StatusNotFound, err)
		return
	}
	now := time.Now().UTC()
	nextBox, nextReview := srs.Next(card.Box, result, now)
	if err := s.store.UpdateVocabReview(id, user.ID, nextBox, nextReview, now); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
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
	s.render(w, r, "words.html", s.dictFor(r)["WordsTitle"], words)
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
	s.render(w, r, "settings.html", s.dictFor(r)["SettingsTitle"], settingsView{NativeLang: native, Username: user.Username})
}

type settingsView struct {
	NativeLang string
	Username   string
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

	_, alreadySaved, err := s.store.FindVocabByPhrase(user.ID, phrase)
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
