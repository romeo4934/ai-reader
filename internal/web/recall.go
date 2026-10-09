package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/romeo4934/ai-reader/internal/ai"
	"github.com/romeo4934/ai-reader/internal/store"
)

// Generating a fill-in-the-blank exercise takes Claude a few seconds, and
// used to happen while the reader waited for each card. Now, while one card
// is on screen, the next recallPrefetch cards' exercises are generated in the
// background, so answering a card lands on one that's already prepared.

const (
	recallPrefetch = 2
	// recallMaxAge drops exercises prepared for a session that was abandoned.
	recallMaxAge = 30 * time.Minute
	// recallWait bounds how long a page waits for an exercise still being
	// generated (same budget as generating it inline).
	recallWait = 25 * time.Second
)

type recallKey struct {
	vocabID int64
	native  string // the exercise's translation is in the reader's language
}

type recallEntry struct {
	done    chan struct{} // closed once card is set
	card    *ai.RecallCard
	started time.Time
}

type recallCache struct {
	mu sync.Mutex
	m  map[recallKey]*recallEntry
}

func newRecallCache() *recallCache { return &recallCache{m: map[recallKey]*recallEntry{}} }

func recallKeyFor(user *store.User, card *store.Vocab) recallKey {
	return recallKey{vocabID: card.ID, native: user.NativeLang}
}

// prefetchRecallCards starts generating exercises for the cards after
// cards[0] (the one being shown), skipping new cards that today's
// new-card allowance won't reach.
func (s *Server) prefetchRecallCards(user *store.User, cards []store.Vocab, newLeft int) {
	newSeen := 0
	for i := range cards {
		isNew := cards[i].LastReviewedAt == nil
		if isNew {
			newSeen++
			if newSeen > newLeft {
				continue
			}
		}
		if i > 0 {
			s.prefetchRecall(user, &cards[i])
		}
	}
}

func (s *Server) prefetchRecall(user *store.User, card *store.Vocab) {
	key := recallKeyFor(user, card)
	c := s.recall
	c.mu.Lock()
	now := time.Now()
	for k, e := range c.m {
		if now.Sub(e.started) > recallMaxAge {
			delete(c.m, k)
		}
	}
	if _, ok := c.m[key]; ok {
		c.mu.Unlock()
		return
	}
	e := &recallEntry{done: make(chan struct{}), started: now}
	c.m[key] = e
	c.mu.Unlock()

	u, v := *user, *card
	go func() {
		// Not tied to the request: it outlives the page that started it.
		e.card = s.generateRecallCard(context.Background(), &u, &v)
		close(e.done)
	}()
}

// recallCardFor returns the exercise for the card being shown: the prepared
// one if there is one (waiting for it if it's still being generated), else a
// freshly generated one. A prepared exercise is used once, so the next time
// the word comes up in review it gets a new sentence.
func (s *Server) recallCardFor(ctx context.Context, user *store.User, card *store.Vocab) *ai.RecallCard {
	key := recallKeyFor(user, card)
	c := s.recall
	c.mu.Lock()
	e := c.m[key]
	delete(c.m, key)
	c.mu.Unlock()
	if e == nil {
		return s.generateRecallCard(ctx, user, card)
	}
	select {
	case <-e.done:
		if e.card != nil {
			return e.card
		}
		return s.generateRecallCard(ctx, user, card)
	case <-time.After(recallWait):
		return nil
	case <-ctx.Done():
		return nil
	}
}

type explainRequest struct {
	VocabID  int64  `json:"vocab_id"`
	Sentence string `json:"sentence"`
	Answer   string `json:"answer"`
	Typed    string `json:"typed"`
}

// handleAPIExplain explains, after a wrong typed answer, how the reader's
// word differs from the expected one. It's an AI call like a translation,
// so it counts toward the free plan's monthly quota.
func (s *Server) handleAPIExplain(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	var req explainRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		s.failJSON(w, http.StatusBadRequest, err)
		return
	}
	req.Typed, req.Answer = strings.TrimSpace(req.Typed), strings.TrimSpace(req.Answer)
	if req.Typed == "" || req.Answer == "" || len(req.Typed) > 80 || len(req.Answer) > 80 || len(req.Sentence) > 500 {
		s.failJSON(w, http.StatusBadRequest, errors.New("requête invalide"))
		return
	}
	card, err := s.store.GetVocab(req.VocabID, user.ID)
	if err != nil {
		s.failJSON(w, http.StatusNotFound, err)
		return
	}
	now := time.Now()
	if over, err := s.overQuota(user, now); err != nil {
		s.failJSON(w, http.StatusInternalServerError, err)
		return
	} else if over {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": fmt.Sprintf(s.dictFor(r)["QuotaReached"], s.cfg.FreeQuota)})
		return
	}
	bookLang := ""
	if book, err := s.store.GetBook(card.BookID, user.ID); err == nil {
		bookLang = book.Language
	}
	native := user.NativeLang
	if native == "" {
		native = defaultNativeLang
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	text, err := s.ai.ExplainDifference(ctx, ai.ExplainOptions{
		BookLanguage: bookLang, NativeLang: native,
		Sentence: req.Sentence, Answer: req.Answer, Typed: req.Typed,
	})
	if err != nil {
		s.failJSON(w, http.StatusBadGateway, err)
		return
	}
	if err := s.store.CountTranslation(user.ID, now); err != nil {
		s.log.Error("count translation", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"explanation": text})
}
