package web

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"slices"
	"time"
)

// The word game, SongPop-style: a round of gameRoundLen words from the
// player's own list, four translations each, and the faster the right
// answer the more points. Solo against one's own record for now, and only
// shown to admins while it's being tried out.

const (
	gameRoundLen  = 5
	gameChoices   = 4
	gameMaxPoints = 10
	gameTimeLimit = 10 * time.Second
)

type gameQuestion struct {
	ID      int64    `json:"id"`
	Phrase  string   `json:"phrase"`
	Choices []string `json:"choices"`
	Answer  int      `json:"answer"` // index in Choices
}

type gameView struct {
	Lang      string
	Speech    string // BCP 47 tag for the browser's speech synthesis
	Tabs      []deckTab
	Questions []gameQuestion
	Best      int
	TooFew    bool
	MaxScore  int
	SecLimit  int
}

func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if !s.isAdmin(user) {
		http.NotFound(w, r)
		return
	}
	langs, err := s.store.VocabLangs(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	view := gameView{MaxScore: gameRoundLen * gameMaxPoints, SecLimit: int(gameTimeLimit / time.Second)}
	if len(langs) == 0 {
		view.TooFew = true
		s.render(w, r, "game.html", s.dictFor(r)["GameTitle"], view)
		return
	}
	view.Lang = langs[0]
	if l := r.URL.Query().Get("lang"); slices.Contains(langs, l) {
		view.Lang = l
	}
	view.Speech = view.Lang
	if len(langs) > 1 {
		for _, l := range langs {
			view.Tabs = append(view.Tabs, deckTab{Lang: l, Name: langName(l), Active: l == view.Lang})
		}
	}

	words, err := s.store.GameWords(user.ID, view.Lang, gameRoundLen)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	for _, wd := range words {
		decoys, err := s.store.GameDecoys(user.ID, view.Lang, user.NativeLang, wd.Translation, gameChoices-1)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		if len(decoys) < gameChoices-1 {
			continue
		}
		choices := append(decoys, wd.Translation)
		rand.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })
		view.Questions = append(view.Questions, gameQuestion{
			ID: wd.ID, Phrase: wd.Phrase, Choices: choices,
			Answer: slices.Index(choices, wd.Translation),
		})
	}
	if len(view.Questions) < gameRoundLen {
		view.TooFew = true
	}
	if view.Best, err = s.store.GameBest(user.ID, view.Lang); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.render(w, r, "game.html", s.dictFor(r)["GameTitle"], view)
}

type gameAnswer struct {
	ID     int64  `json:"id"`
	Choice string `json:"choice"`
	MS     int64  `json:"ms"`
}

// gamePoints scores one answer: full points for an instant right answer,
// one less per second taken, nothing once time is up or for a wrong one.
func gamePoints(right bool, ms int64) int {
	if !right || ms < 0 || ms >= gameTimeLimit.Milliseconds() {
		return 0
	}
	return max(1, gameMaxPoints-int(ms/1000))
}

// handleGameScore checks a finished round against the words' translations,
// so the score is the server's, and records it.
func (s *Server) handleGameScore(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if !s.isAdmin(user) {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Lang    string       `json:"lang"`
		Answers []gameAnswer `json:"answers"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil ||
		!validLangKey(req.Lang) || len(req.Answers) > gameRoundLen {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ids := make([]int64, len(req.Answers))
	for i, a := range req.Answers {
		ids[i] = a.ID
	}
	translations, err := s.store.GameTranslations(user.ID, ids)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	score := 0
	seen := map[int64]bool{}
	for _, a := range req.Answers {
		t, ok := translations[a.ID]
		if !ok || seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		score += gamePoints(a.Choice == t, a.MS)
	}
	best, err := s.store.SaveGameScore(user.ID, req.Lang, score, time.Now())
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"score": score, "best": max(best, score), "record": score > best && score > 0})
}
