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
	"github.com/romeo4934/ai-reader/internal/epub"
	"github.com/romeo4934/ai-reader/internal/frequency"
	"github.com/romeo4934/ai-reader/internal/srs"
	"github.com/romeo4934/ai-reader/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

const settingNativeLang = "native_lang"
const defaultNativeLang = "français"

const maxUploadBytes = 30 << 20 // 30 MiB — plenty for a novel-length epub

type Server struct {
	store *store.Store
	ai    *ai.Client
	tmpl  *template.Template
	log   *slog.Logger
}

func New(st *store.Store, aiClient *ai.Client, log *slog.Logger) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"highlight": highlightPhrase,
		"freqLabel": frequency.Label,
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates : %w", err)
	}
	return &Server{store: st, ai: aiClient, tmpl: tmpl, log: log}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /ready", s.handleReady)
	mux.Handle("GET /static/", http.FileServerFS(staticFS))

	mux.HandleFunc("GET /{$}", s.handleLibrary)
	mux.HandleFunc("POST /books", s.handleUploadBook)
	mux.HandleFunc("GET /books/{id}", s.handleReader)

	mux.HandleFunc("GET /review", s.handleReviewPage)
	mux.HandleFunc("POST /review/{id}/answer", s.handleReviewAnswer)

	mux.HandleFunc("GET /words", s.handleWords)

	mux.HandleFunc("GET /settings", s.handleSettingsGet)
	mux.HandleFunc("POST /settings", s.handleSettingsPost)

	mux.HandleFunc("POST /api/translate", s.handleAPITranslate)

	return mux
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

// --- pages ---

type pageData struct {
	Title    string
	DueCount int
	Data     any
}

func (s *Server) render(w http.ResponseWriter, name, title string, data any) {
	due, err := s.store.CountDueVocab(time.Now().UTC())
	if err != nil {
		s.log.Error("count due vocab", "err", err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, pageData{Title: title, DueCount: due, Data: data}); err != nil {
		s.log.Error("render template", "template", name, "err", err)
	}
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	books, err := s.store.ListBooks()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.render(w, "library.html", "Bibliothèque", books)
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
	bookID, err := s.store.InsertBook(book.Title, book.Author, book.Language, chapters)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/books/%d", bookID), http.StatusSeeOther)
}

type readerView struct {
	Book       store.Book
	Chapter    store.Chapter
	Paragraphs []string
	NativeLang string
	HasPrev    bool
	HasNext    bool
	ChapterIdx int
}

func (s *Server) handleReader(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	book, err := s.store.GetBook(bookID)
	if err != nil {
		s.fail(w, http.StatusNotFound, fmt.Errorf("livre introuvable : %w", err))
		return
	}

	chIdx := 0
	if q := r.URL.Query().Get("ch"); q != "" {
		chIdx, _ = strconv.Atoi(q)
	} else if saved, err := s.store.GetProgress(bookID); err == nil {
		chIdx = saved
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
	_ = s.store.SetProgress(bookID, chIdx)

	native, _ := s.store.GetSetting(settingNativeLang)
	if native == "" {
		native = defaultNativeLang
	}

	s.render(w, "reader.html", book.Title, readerView{
		Book:       book,
		Chapter:    chapter,
		Paragraphs: strings.Split(chapter.Content, "\n\n"),
		NativeLang: native,
		HasPrev:    chIdx > 0,
		HasNext:    chIdx < book.ChapterCount-1,
		ChapterIdx: chIdx,
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
	cards, err := s.store.DueVocab(time.Now().UTC(), 1)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	view := reviewView{}
	if len(cards) > 0 {
		view.Card = &cards[0]
		view.Recall = s.generateRecallCard(r.Context(), view.Card)
	}
	s.render(w, "review.html", "Révision", view)
}

// generateRecallCard asks Claude for a fresh fill-in-the-blank exercise for
// this card. Returns nil on any failure (no API key, network error, refusal)
// so the review page falls back to the static context+translation card
// instead of breaking review entirely.
func (s *Server) generateRecallCard(ctx context.Context, card *store.Vocab) *ai.RecallCard {
	book, err := s.store.GetBook(card.BookID)
	bookLang := ""
	if err == nil {
		bookLang = book.Language
	}
	native, _ := s.store.GetSetting(settingNativeLang)
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

	card, err := s.store.GetVocab(id)
	if err != nil {
		s.fail(w, http.StatusNotFound, err)
		return
	}
	now := time.Now().UTC()
	nextBox, nextReview := srs.Next(card.Box, result, now)
	if err := s.store.UpdateVocabReview(id, nextBox, nextReview, now); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/review", http.StatusSeeOther)
}

func (s *Server) handleWords(w http.ResponseWriter, r *http.Request) {
	words, err := s.store.ListVocab()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.render(w, "words.html", "Mes mots", words)
}

// --- settings ---

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	native, _ := s.store.GetSetting(settingNativeLang)
	if native == "" {
		native = defaultNativeLang
	}
	s.render(w, "settings.html", "Réglages", native)
}

func (s *Server) handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	native := strings.TrimSpace(r.FormValue("native_lang"))
	if native == "" {
		native = defaultNativeLang
	}
	if err := s.store.SetSetting(settingNativeLang, native); err != nil {
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
	book, err := s.store.GetBook(req.BookID)
	if err != nil {
		s.failJSON(w, http.StatusNotFound, err)
		return
	}
	native, _ := s.store.GetSetting(settingNativeLang)
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

	_, alreadySaved, err := s.store.FindVocabByPhrase(phrase)
	if err != nil {
		s.failJSON(w, http.StatusInternalServerError, err)
		return
	}
	if !alreadySaved {
		freq := tr.Frequency
		if rank, found := frequency.Rank(book.Language, tr.Lemma); found {
			freq = rank
		} else {
			freq = frequency.FallbackFromEstimate(tr.Frequency)
		}
		if _, err := s.store.InsertVocab(store.Vocab{
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
	writeJSON(w, http.StatusOK, translateResponse{Translation: tr, Saved: !alreadySaved})
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// highlightPhrase wraps the first occurrence of phrase inside context with
// <mark>, escaping both pieces itself so it's safe to mark as template.HTML.
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
