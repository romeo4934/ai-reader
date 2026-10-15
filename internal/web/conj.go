package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/romeo4934/ai-reader/internal/ai"
	"github.com/romeo4934/ai-reader/internal/conj"
	"github.com/romeo4934/ai-reader/internal/i18n"
	"github.com/romeo4934/ai-reader/internal/store"
)

// The conjugation exercise: series of conjSeries forms to type ("aller ·
// nous · futur" → "irons"), in the tenses the reader picks, from the verbs
// they looked up in their books and the language's most common ones. The
// tables are worked out by the AI once per verb and kept; the forms missed
// come back in the next series, then less and less often.

const (
	conjSeries = 10
	// conjAgain: at most this many forms missed before in a series.
	conjAgain = 4
	// conjTop: the most common verbs, asked about more often.
	conjTop = 40
)

type conjQuestion struct {
	Verb       string   `json:"verb"`
	Meaning    string   `json:"meaning"`
	Tense      string   `json:"tense"`
	TenseName  string   `json:"tenseName"`
	TenseGloss string   `json:"tenseGloss"`
	Person     int      `json:"person"`
	Pronoun    string   `json:"pronoun"`
	Answers    []string `json:"answers"`
	Full       string   `json:"full"`
	// The sentence the verb was read in, for a verb of the reader's books.
	Sentence string `json:"sentence,omitempty"`
	Phrase   string `json:"phrase,omitempty"`
	Book     string `json:"book,omitempty"`
	Again    bool   `json:"again,omitempty"`
}

type conjTenseView struct {
	ID, Name, Gloss string
	On              bool
	Score           int // percent, -1 before any answer
}

type conjView struct {
	Tabs      []deckTab
	Lang      string
	Tenses    []conjTenseView
	Questions template.JS
	Count     int
	OwnVerbs  int
	Preparing bool // no table ready yet
}

// conjLangs: the languages of the reader's books first, then the others
// but their own.
func (s *Server) conjLangs(user *store.User) []string {
	native := i18n.For(user.NativeLang)["LangCode"]
	var out []string
	if own, err := s.store.VocabLangs(user.ID); err == nil {
		for _, l := range own {
			if conj.Has(l) && !slices.Contains(out, l) {
				out = append(out, l)
			}
		}
	}
	for _, l := range conj.Langs() {
		if l != native && !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out
}

func (s *Server) handleConj(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	T := s.dictFor(r)
	langs := s.conjLangs(user)
	lang := r.FormValue("lang")
	if !slices.Contains(langs, lang) {
		lang = langs[0]
	}

	// The tenses: those just ticked (then kept), else those kept, else the
	// first ones.
	var tenses []string
	if r.URL.Query().Has("go") {
		for _, id := range r.URL.Query()["t"] {
			if _, ok := conj.TenseOf(lang, id); ok && !slices.Contains(tenses, id) {
				tenses = append(tenses, id)
			}
		}
		if len(tenses) > 0 {
			if err := s.store.SetConjTenses(user.ID, lang, tenses); err != nil {
				s.log.Error("conjugaison : temps", "err", err)
			}
		}
	}
	if len(tenses) == 0 {
		kept, err := s.store.ConjTenses(user.ID, lang)
		if err != nil {
			s.log.Error("conjugaison : temps", "err", err)
		}
		for _, id := range kept {
			if _, ok := conj.TenseOf(lang, id); ok {
				tenses = append(tenses, id)
			}
		}
	}
	if len(tenses) == 0 {
		tenses = conj.DefaultTenses(lang)
	}

	scores, err := s.store.ConjScores(user.ID, lang)
	if err != nil {
		s.log.Error("conjugaison : scores", "err", err)
	}
	v := conjView{Lang: lang}
	for _, l := range langs {
		v.Tabs = append(v.Tabs, deckTab{Lang: l, Name: localLangName(T["LangCode"], l), Active: l == lang})
	}
	for _, t := range conj.Tenses(lang) {
		tv := conjTenseView{ID: t.ID, Name: t.Name, Gloss: tenseGloss(T, lang, t), On: slices.Contains(tenses, t.ID), Score: -1}
		if sc, ok := scores[t.ID]; ok && sc.Total > 0 {
			tv.Score = sc.Right * 100 / sc.Total
		}
		v.Tenses = append(v.Tenses, tv)
	}

	qs, own, err := s.conjQuestions(r.Context(), user, T, lang, tenses)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	raw, err := json.Marshal(qs)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	v.Questions = template.JS(raw)
	v.Count = len(qs)
	v.OwnVerbs = own
	v.Preparing = len(qs) == 0
	s.render(w, r, "conjugate.html", T["ConjTitle"], v)
}

// tenseGloss: what the tense is called in the reader's language, when it
// isn't the name the learned language gives it.
func tenseGloss(T i18n.Dict, lang string, t conj.Tense) string {
	g := T["Tense_"+t.ID]
	if lang == T["LangCode"] || strings.EqualFold(g, t.Name) {
		return ""
	}
	return g
}

// conjQuestions makes a series: the forms due again first, then forms of
// the reader's own verbs and of common ones, at random. own is how many of
// the reader's verbs can be drilled.
func (s *Server) conjQuestions(ctx context.Context, user *store.User, T i18n.Dict, lang string, tenses []string) (qs []conjQuestion, own int, err error) {
	native := T["LangCode"]
	ready, err := s.store.ConjVerbs(lang)
	if err != nil {
		return nil, 0, err
	}
	var common []conj.Verb
	for _, v := range conj.Common(lang) {
		if ready[v.Infinitive] {
			common = append(common, v)
		}
	}
	// Nothing worked out yet for this language (the background pass hasn't
	// come by): the most common verbs, now.
	if len(common) < conjSeries && s.ai.Enabled() {
		var missing []string
		for _, v := range conj.Common(lang) {
			if _, done := ready[v.Infinitive]; !done && len(missing) < conjSeries+2 {
				missing = append(missing, v.Infinitive)
			}
		}
		callCtx, cancel := context.WithTimeout(withUsageUser(ctx, user.ID), 40*time.Second)
		s.conjugateAll(callCtx, lang, missing, len(missing))
		cancel()
		if ready, err = s.store.ConjVerbs(lang); err != nil {
			return nil, 0, err
		}
		common = common[:0]
		for _, v := range conj.Common(lang) {
			if ready[v.Infinitive] {
				common = append(common, v)
			}
		}
	}

	ownAll, err := s.store.OwnVerbs(user.ID, lang)
	if err != nil {
		return nil, 0, err
	}
	var mine []store.OwnVerb
	var toDo []string
	for _, v := range ownAll {
		isVerb, done := ready[v.Infinitive]
		switch {
		case isVerb:
			mine = append(mine, v)
		case !done:
			toDo = append(toDo, v.Infinitive)
		}
	}
	s.conjugateLater(user.ID, lang, toDo)

	used := map[string]bool{}
	tables := map[string]map[string][]string{}
	add := func(verb, tense string, person int, again bool) bool {
		t, ok := conj.TenseOf(lang, tense)
		if !ok {
			return false
		}
		table, ok := tables[verb]
		if !ok {
			if table, _, err = s.store.ConjTable(lang, verb); err != nil {
				return false
			}
			tables[verb] = table
		}
		forms := table[tense]
		if t.Impersonal {
			person = 0
		}
		if person < 0 || person >= len(forms) || forms[person] == "" {
			return false
		}
		q := conjQuestion{
			Verb: verb, Tense: tense, TenseName: t.Name, TenseGloss: tenseGloss(T, lang, t),
			Person: person, Pronoun: conj.Pronoun(lang, t, person), Again: again,
			Full: conj.Full(lang, t, person, forms[person]),
		}
		q.Answers = conj.Answers(forms[person])
		if i := slices.IndexFunc(mine, func(o store.OwnVerb) bool { return o.Infinitive == verb }); i >= 0 {
			o := mine[i]
			q.Meaning, q.Phrase, q.Book = o.Translation, o.Phrase, o.Book
			q.Sentence = wordSentence(o.Context, o.Phrase)
		} else if cv, ok := conj.CommonVerb(lang, verb); ok {
			q.Meaning = cv.Meaning(native)
		}
		qs = append(qs, q)
		used[verb] = true
		return true
	}

	due, err := s.store.ConjDue(user.ID, lang, time.Now(), 50)
	if err != nil {
		return nil, 0, err
	}
	for _, it := range due {
		if len(qs) >= conjAgain {
			break
		}
		if slices.Contains(tenses, it.Tense) && ready[it.Verb] && !used[it.Verb] {
			add(it.Verb, it.Tense, it.Person, true)
		}
	}

	for tries := 0; len(qs) < conjSeries && tries < 100; tries++ {
		var verb string
		switch r := rand.Float64(); {
		case len(mine) > 0 && r < 0.5:
			verb = mine[rand.IntN(len(mine))].Infinitive
		case len(common) == 0:
			continue
		case r < 0.85:
			verb = common[rand.IntN(min(conjTop, len(common)))].Infinitive
		default:
			verb = common[rand.IntN(len(common))].Infinitive
		}
		if used[verb] {
			continue
		}
		tense := tenses[rand.IntN(len(tenses))]
		t, _ := conj.TenseOf(lang, tense)
		add(verb, tense, rand.IntN(conj.Count(lang, t)), false)
	}
	return qs, len(mine), nil
}

// conjBusy: the readers whose own verbs are being worked out.
var conjBusy sync.Map

// conjPerVisit: the reader's verbs worked out per visit, in the background.
const conjPerVisit = 10

func (s *Server) conjugateLater(userID int64, lang string, verbs []string) {
	if len(verbs) == 0 || !s.ai.Enabled() {
		return
	}
	if _, busy := conjBusy.LoadOrStore(userID, true); busy {
		return
	}
	if len(verbs) > conjPerVisit {
		verbs = verbs[:conjPerVisit]
	}
	go func() {
		defer conjBusy.Delete(userID)
		ctx, cancel := context.WithTimeout(withUsageUser(context.Background(), userID), 2*time.Minute)
		defer cancel()
		s.conjugateAll(ctx, lang, verbs, 3)
	}()
}

// conjugateAll works out and keeps the tables of verbs, parallel at once.
// After a few failures it gives up (the API is down or out of credit):
// the next pass will try again. ok is false then.
func (s *Server) conjugateAll(ctx context.Context, lang string, verbs []string, parallel int) (ok bool) {
	sem := make(chan struct{}, max(parallel, 1))
	var wg sync.WaitGroup
	var failed atomic.Int32
	for _, v := range verbs {
		if failed.Load() >= 3 {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			if err := s.conjugate(ctx, lang, v); err != nil {
				failed.Add(1)
				s.log.Error("conjugaison", "lang", lang, "verb", v, "err", err)
			}
		}()
	}
	wg.Wait()
	return failed.Load() < 3
}

// conjugate works out a verb's table and keeps it (or that it isn't one).
func (s *Server) conjugate(ctx context.Context, lang, verb string) error {
	opts := ai.ConjugateOptions{Language: lang, Verb: verb, Persons: conj.Persons(lang)}
	for _, t := range conj.Tenses(lang) {
		opts.Tenses = append(opts.Tenses, ai.ConjTense{ID: t.ID, Name: t.Name, Forms: conj.Count(lang, t)})
	}
	var table ai.ConjTable
	var err error
	for try := 0; try < 2; try++ { // a table with a form missing: once more
		if table, err = s.ai.Conjugate(ctx, opts); err == nil || ai.IsNotAVerb(err) || ctx.Err() != nil {
			break
		}
	}
	switch {
	case ai.IsNotAVerb(err):
		return s.store.SetConjTable(lang, verb, nil, time.Now())
	case err != nil:
		return err
	}
	return s.store.SetConjTable(lang, verb, table.Forms, time.Now())
}

// conjEvery: how often the background pass works out the common verbs'
// tables not kept yet (all of them, the first time).
const conjEvery = 30 * time.Minute

// RunConj works out the common verbs' tables every conjEvery until ctx ends.
func (s *Server) RunConj(ctx context.Context) {
	t := time.NewTicker(conjEvery)
	defer t.Stop()
	for {
		if s.ai.Enabled() {
			for _, lang := range conj.Langs() {
				if ctx.Err() != nil {
					return
				}
				ready, err := s.store.ConjVerbs(lang)
				if err != nil {
					s.log.Error("conjugaison : verbes", "err", err)
					break
				}
				var missing []string
				for _, v := range conj.Common(lang) {
					if _, done := ready[v.Infinitive]; !done {
						missing = append(missing, v.Infinitive)
					}
				}
				if !s.conjugateAll(ctx, lang, missing, 3) {
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

type conjAnswerRequest struct {
	Lang   string `json:"lang"`
	Verb   string `json:"verb"`
	Tense  string `json:"tense"`
	Person int    `json:"person"`
	OK     bool   `json:"ok"`
}

// handleConjAnswer records an answer of the series.
func (s *Server) handleConjAnswer(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	var req conjAnswerRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 2048)).Decode(&req); err != nil {
		s.failJSON(w, http.StatusBadRequest, err)
		return
	}
	if _, ok := conj.TenseOf(req.Lang, req.Tense); !ok || req.Verb == "" || len(req.Verb) > 60 || req.Person < 0 || req.Person > 6 {
		s.failJSON(w, http.StatusBadRequest, errors.New("requête invalide"))
		return
	}
	if err := s.store.ConjAnswered(user.ID, req.Lang, store.ConjItem{Verb: req.Verb, Tense: req.Tense, Person: req.Person}, req.OK, time.Now()); err != nil {
		s.failJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type conjExplainRequest struct {
	Lang   string `json:"lang"`
	Verb   string `json:"verb"`
	Tense  string `json:"tense"`
	Person int    `json:"person"`
	Typed  string `json:"typed"`
	Answer string `json:"answer"`
}

// handleConjExplain: why the form is what it is, after a wrong answer.
func (s *Server) handleConjExplain(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	var req conjExplainRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 2048)).Decode(&req); err != nil {
		s.failJSON(w, http.StatusBadRequest, err)
		return
	}
	req.Typed, req.Answer = strings.TrimSpace(req.Typed), strings.TrimSpace(req.Answer)
	t, ok := conj.TenseOf(req.Lang, req.Tense)
	if !ok || req.Verb == "" || len(req.Verb) > 60 || req.Typed == "" || len(req.Typed) > 80 || req.Answer == "" || len(req.Answer) > 80 {
		s.failJSON(w, http.StatusBadRequest, errors.New("requête invalide"))
		return
	}
	now := time.Now()
	if over, err := s.overQuota(user, now); err != nil {
		s.failJSON(w, http.StatusInternalServerError, err)
		return
	} else if over {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": fmt.Sprintf(s.dictFor(r)["QuotaReached"], s.quotaFor(user))})
		return
	}
	native := user.NativeLang
	if native == "" {
		native = defaultNativeLang
	}
	ctx, cancel := context.WithTimeout(withUsageUser(r.Context(), user.ID), 25*time.Second)
	defer cancel()
	text, err := s.ai.ExplainConj(ctx, ai.ConjExplainOptions{
		Language: req.Lang, NativeLang: native, Verb: req.Verb, Tense: t.Name,
		Person: conj.Pronoun(req.Lang, t, req.Person), Typed: req.Typed, Answer: req.Answer,
	})
	if err != nil {
		s.failAI(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"explanation": text})
}
