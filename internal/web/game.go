package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/romeo4934/ai-reader/internal/store"
)

// DLingo Destruction, the word game, SongPop-style: a round of gameRoundLen
// words, four translations each, and the faster the right answer the more
// points. Its heart is the duel: a reader plays a round against a friend,
// who then plays the same questions — half the challenger's words, half
// their own. Solo rounds are practice against one's record.
//
// While it's tried out, only admins can start rounds; anyone challenged to
// a duel can play it and ask for a rematch.

const (
	gameRoundLen  = 5
	gameChoices   = 4
	gameMaxPoints = 10
	gameTimeLimit = 10 * time.Second
	gameMaxScore  = gameRoundLen * gameMaxPoints
)

type gameQuestion struct {
	Phrase  string   `json:"phrase"`
	Choices []string `json:"choices"`
	Answer  int      `json:"answer"` // index in Choices
}

// gamePoints scores one answer: full points for an instant right answer,
// one less per second taken, nothing once time is up or for a wrong one.
func gamePoints(right bool, ms int64) int {
	if !right || ms < 0 || ms >= gameTimeLimit.Milliseconds() {
		return 0
	}
	return max(1, gameMaxPoints-int(ms/1000))
}

// canPlay: the game isn't public yet.
func (s *Server) canPlay(user *store.User) bool {
	if s.isAdmin(user) {
		return true
	}
	_, involved, err := s.store.GameTurns(user.ID)
	return err == nil && involved
}

// gameQuestions builds a round in lang from the words of owners, taking
// turns between them, each word's decoys drawn from its owner's list.
func (s *Server) gameQuestions(lang string, owners ...store.User) ([]gameQuestion, error) {
	pools := make([][]store.GameWord, len(owners))
	for i, o := range owners {
		words, err := s.store.GameWords(o.ID, lang, gameRoundLen*2)
		if err != nil {
			return nil, err
		}
		pools[i] = words
	}
	var out []gameQuestion
	seen := map[string]bool{}
	for turn := 0; len(out) < gameRoundLen; turn++ {
		i := turn % len(owners)
		if len(pools[i]) == 0 {
			if !slices.ContainsFunc(pools, func(p []store.GameWord) bool { return len(p) > 0 }) {
				break
			}
			continue
		}
		wd := pools[i][0]
		pools[i] = pools[i][1:]
		key := strings.ToLower(strings.TrimSpace(wd.Phrase))
		if seen[key] {
			continue
		}
		decoys, err := s.store.GameDecoys(owners[i].ID, lang, owners[i].NativeLang, wd.Translation, gameChoices-1)
		if err != nil {
			return nil, err
		}
		if len(decoys) < gameChoices-1 {
			continue
		}
		seen[key] = true
		choices := append(decoys, wd.Translation)
		rand.Shuffle(len(choices), func(a, b int) { choices[a], choices[b] = choices[b], choices[a] })
		out = append(out, gameQuestion{Phrase: wd.Phrase, Choices: choices, Answer: slices.Index(choices, wd.Translation)})
	}
	return out, nil
}

type gameDuel struct {
	ID         int64
	OtherID    int64
	Other      string
	Mine       *int
	Theirs     *int
	Challenged bool // the other one started it
	Lang       string
}

func (d gameDuel) Result(T map[string]string) string {
	switch {
	case d.Mine == nil || d.Theirs == nil:
		return ""
	case *d.Mine > *d.Theirs:
		return fmt.Sprintf(T["GameYouWon"], d.Other, *d.Mine, *d.Theirs)
	case *d.Mine < *d.Theirs:
		return fmt.Sprintf(T["GameYouLost"], d.Other, *d.Theirs, *d.Mine)
	default:
		return fmt.Sprintf(T["GameDraw"], d.Other, *d.Mine)
	}
}

func duelFor(g store.GameRound, userID int64) gameDuel {
	d := gameDuel{ID: g.ID, Lang: g.Lang}
	if g.ChallengerID == userID {
		d.OtherID, d.Other = g.OpponentID, publicName(g.OpponentID, g.OpponentUser, g.OpponentDisp)
		d.Mine, d.Theirs = g.ChallengerScore, g.OpponentScore
	} else {
		d.OtherID, d.Other = g.ChallengerID, publicName(g.ChallengerID, g.ChallengerUser, g.ChallengerDisp)
		d.Mine, d.Theirs = g.OpponentScore, g.ChallengerScore
		d.Challenged = true
	}
	return d
}

type gameFriend struct {
	ID   int64
	Name string
}

type gameHub struct {
	Lang    string
	Tabs    []deckTab
	Best    int
	Max     int
	Admin   bool
	TooFew  bool
	Turns   []gameDuel
	Waiting []gameDuel
	Done    []gameDuel
	Friends []gameFriend
}

func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if !s.canPlay(user) {
		http.NotFound(w, r)
		return
	}
	hub := gameHub{Max: gameMaxScore, Admin: s.isAdmin(user), TooFew: r.URL.Query().Get("few") != ""}
	langs, err := s.store.VocabLangs(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if len(langs) > 0 {
		hub.Lang = langs[0]
		if l := r.URL.Query().Get("lang"); slices.Contains(langs, l) {
			hub.Lang = l
		}
		if len(langs) > 1 {
			for _, l := range langs {
				hub.Tabs = append(hub.Tabs, deckTab{Lang: l, Name: langName(l), Active: l == hub.Lang})
			}
		}
		if hub.Best, err = s.store.GameBest(user.ID, hub.Lang); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	duels, err := s.store.GameDuels(user.ID, 30)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	for _, g := range duels {
		d := duelFor(g, user.ID)
		switch {
		case d.Mine == nil:
			hub.Turns = append(hub.Turns, d)
		case d.Theirs == nil:
			hub.Waiting = append(hub.Waiting, d)
		default:
			hub.Done = append(hub.Done, d)
		}
	}
	friends, err := s.store.Friends(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	for _, f := range friends {
		hub.Friends = append(hub.Friends, gameFriend{ID: f.ID, Name: publicName(f.ID, f.Username, f.DisplayName)})
	}
	s.render(w, r, "game.html", s.dictFor(r)["GameTitle"], hub)
}

// handleGameNew starts a round: solo (no friend) or a duel with a friend,
// which the challenger plays first.
func (s *Server) handleGameNew(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if !s.canPlay(user) || (!s.isAdmin(user) && r.FormValue("friend") == "") {
		http.NotFound(w, r)
		return
	}
	lang := r.FormValue("lang")
	langs, err := s.store.VocabLangs(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if !slices.Contains(langs, lang) && len(langs) > 0 {
		lang = langs[0]
	}
	owners := []store.User{*user}
	var opponentID int64
	if fid, _ := strconv.ParseInt(r.FormValue("friend"), 10, 64); fid != 0 {
		ok, err := s.store.AreFriends(user.ID, fid)
		if err == nil && ok && !s.isAdmin(user) {
			// Not public yet: a non-admin can only take revenge.
			ok, err = s.store.GameDueled(user.ID, fid)
		}
		if err != nil || !ok {
			http.NotFound(w, r)
			return
		}
		friend, err := s.store.GetUserByID(fid)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		opponentID = fid
		// The friend's words too, when they read in this language and get
		// their translations in the same one (the answers are shown as is).
		theirs, err := s.store.VocabLangs(fid)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		if !slices.Contains(langs, lang) || !slices.Contains(theirs, lang) {
			for _, l := range langs {
				if slices.Contains(theirs, l) {
					lang = l
					break
				}
			}
		}
		if slices.Contains(theirs, lang) && strings.EqualFold(friend.NativeLang, user.NativeLang) {
			owners = append(owners, friend)
		}
	}
	qs, err := s.gameQuestions(lang, owners...)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if len(qs) < gameRoundLen {
		http.Redirect(w, r, "/game?few=1", http.StatusSeeOther)
		return
	}
	raw, _ := json.Marshal(qs)
	id, err := s.store.CreateGameRound(user.ID, opponentID, lang, string(raw), time.Now())
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/game/rounds/%d", id), http.StatusSeeOther)
}

type gamePlay struct {
	ID        int64
	Lang      string
	Questions []gameQuestion
	Duel      *gameDuel // nil when solo
	Played    bool      // already played: show the result instead
	Best      int
	Max       int
	SecLimit  int
}

func (s *Server) gameRound(r *http.Request) (store.GameRound, error) {
	user := userFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return store.GameRound{}, err
	}
	return s.store.GetGameRound(id, user.ID)
}

func (s *Server) handleGameRound(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	g, err := s.gameRound(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	view := gamePlay{ID: g.ID, Lang: g.Lang, Max: gameMaxScore, SecLimit: int(gameTimeLimit / time.Second)}
	if err := json.Unmarshal([]byte(g.Questions), &view.Questions); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if g.OpponentID != 0 {
		d := duelFor(g, user.ID)
		view.Duel = &d
		view.Played = d.Mine != nil
	} else {
		view.Played = g.ChallengerScore != nil
		if view.Best, err = s.store.GameBest(user.ID, g.Lang); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	s.render(w, r, "game_play.html", s.dictFor(r)["GameTitle"], view)
}

// handleGameScore checks a played round's answers against its questions,
// so the score is the server's, and records it.
func (s *Server) handleGameScore(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	g, err := s.gameRound(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var qs []gameQuestion
	if err := json.Unmarshal([]byte(g.Questions), &qs); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	var answers []struct {
		Choice int   `json:"choice"`
		MS     int64 `json:"ms"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&answers); err != nil || len(answers) > len(qs) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	score := 0
	for i, a := range answers {
		score += gamePoints(a.Choice == qs[i].Answer, a.MS)
	}
	now := time.Now()
	best := 0
	if g.OpponentID == 0 {
		if best, err = s.store.GameBest(user.ID, g.Lang); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	if err := s.store.SetGameScore(g.ID, user.ID, score, now); err != nil {
		if errors.Is(err, store.ErrAlreadyPlayed) {
			http.Error(w, "already played", http.StatusConflict)
			return
		}
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if g.OpponentID == 0 {
		if _, err := s.store.SaveGameScore(user.ID, g.Lang, score, now); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"score": score, "best": max(best, score), "record": g.OpponentID == 0 && score > best && score > 0})
}
