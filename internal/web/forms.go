package web

import (
	"context"
	"sync"
	"time"

	"github.com/romeo4934/ai-reader/internal/ai"
	"github.com/romeo4934/ai-reader/internal/store"
)

// Verb forms on the words page ("rode ← ride · rode · ridden") and the
// review card: worked out by the AI once per word — in the background when
// the reader opens the words page, by a background pass for any word left
// out, and right away for a review card that has none yet.

const formsPerVisit = 30

// formsBusy: the users whose words are being worked out, so that reloading
// the page doesn't start the same calls again.
var formsBusy sync.Map

func (s *Server) fillForms(user store.User, words []store.Vocab) {
	if len(words) == 0 || !s.ai.Enabled() {
		return
	}
	if _, busy := formsBusy.LoadOrStore(user.ID, true); busy {
		return
	}
	if len(words) > formsPerVisit {
		words = words[:formsPerVisit]
	}
	go func() {
		defer formsBusy.Delete(user.ID)
		native := user.NativeLang
		if native == "" {
			native = defaultNativeLang
		}
		langs := map[int64]string{}
		ctx := withUsageUser(context.Background(), user.ID)
		for _, w := range words {
			lang, ok := langs[w.BookID]
			if !ok {
				if b, err := s.store.GetBook(w.BookID, user.ID); err == nil {
					lang = b.Language
				}
				langs[w.BookID] = lang
			}
			callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			forms, err := s.ai.WordForms(callCtx, ai.FormsOptions{
				BookLanguage: lang, NativeLang: native, Phrase: w.Phrase, Lemma: w.Lemma,
				Context: wordSentence(w.Context, w.Phrase),
			})
			cancel()
			if err != nil {
				s.log.Error("formes du mot", "vocab_id", w.ID, "err", err)
				return
			}
			if err := s.store.SetVocabForms(w.ID, user.ID, forms); err != nil {
				s.log.Error("formes du mot", "vocab_id", w.ID, "err", err)
				return
			}
		}
	}()
}

// cardForms gives a review card's forms, working them out now (briefly)
// when they never were: the card is on screen, it can't wait for the
// background pass.
func (s *Server) cardForms(ctx context.Context, user *store.User, card *store.Vocab) string {
	forms, ok, eligible, err := s.store.VocabFormsOf(card.ID, user.ID)
	if err != nil || ok || !eligible || !s.ai.Enabled() {
		return forms
	}
	lang := ""
	if b, err := s.store.GetBook(card.BookID, user.ID); err == nil {
		lang = b.Language
	}
	native := user.NativeLang
	if native == "" {
		native = defaultNativeLang
	}
	callCtx, cancel := context.WithTimeout(withUsageUser(ctx, user.ID), 6*time.Second)
	defer cancel()
	forms, err = s.ai.WordForms(callCtx, ai.FormsOptions{
		BookLanguage: lang, NativeLang: native, Phrase: card.Phrase, Lemma: card.Lemma,
		Context: wordSentence(card.Context, card.Phrase),
	})
	if err != nil {
		s.log.Error("formes du mot", "vocab_id", card.ID, "err", err)
		return ""
	}
	if err := s.store.SetVocabForms(card.ID, user.ID, forms); err != nil {
		s.log.Error("formes du mot", "vocab_id", card.ID, "err", err)
	}
	return forms
}

// formsEvery: how often the background pass looks for words whose forms
// were never worked out (new words, and those from before forms existed).
const (
	formsEvery    = 10 * time.Minute
	formsPerRound = 200
)

// RunForms works out missing forms every formsEvery until ctx ends.
func (s *Server) RunForms(ctx context.Context) {
	t := time.NewTicker(formsEvery)
	defer t.Stop()
	for {
		s.fillMissingForms(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Server) fillMissingForms(ctx context.Context) {
	if !s.ai.Enabled() {
		return
	}
	jobs, err := s.store.VocabNeedingForms(formsPerRound)
	if err != nil {
		s.log.Error("formes : à faire", "err", err)
		return
	}
	for _, j := range jobs {
		if ctx.Err() != nil {
			return
		}
		native := j.NativeLang
		if native == "" {
			native = defaultNativeLang
		}
		callCtx, cancel := context.WithTimeout(withUsageUser(ctx, j.Vocab.UserID), 20*time.Second)
		forms, err := s.ai.WordForms(callCtx, ai.FormsOptions{
			BookLanguage: j.BookLanguage, NativeLang: native, Phrase: j.Vocab.Phrase, Lemma: j.Vocab.Lemma,
			Context: wordSentence(j.Vocab.Context, j.Vocab.Phrase),
		})
		cancel()
		if err != nil {
			s.log.Error("formes du mot", "vocab_id", j.Vocab.ID, "err", err)
			return // the API is down or refusing: next round
		}
		if err := s.store.SetVocabForms(j.Vocab.ID, j.Vocab.UserID, forms); err != nil {
			s.log.Error("formes du mot", "vocab_id", j.Vocab.ID, "err", err)
			return
		}
	}
}
