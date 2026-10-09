package web

import (
	"context"
	"sync"
	"time"

	"github.com/romeo4934/ai-reader/internal/ai"
	"github.com/romeo4934/ai-reader/internal/store"
)

// Verb forms on the words page ("rode ← ride · rode · ridden"): worked out
// by the AI once per word, in the background when the reader opens the
// page, and shown from the next visit on.

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
