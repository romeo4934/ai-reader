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
// words, four answers each, and the faster the right answer the more points
// — doubled from the third right answer in a row. Its heart is the duel: a
// reader plays a round against a friend, who then plays the same questions
// — half the challenger's words, half their own, unless the challenger
// narrowed them (a book, missed words, a hand-picked five). Solo rounds are
// practice against one's record. Words of one's own missed in a round come
// back into review.
//
// While it's tried out, only admins can start rounds; anyone challenged to
// a duel can play it and ask for a rematch.

const (
	gameRoundLen  = 5
	gameChoices   = 4
	gameMaxPoints = 10
	gameTimeLimit = 10 * time.Second
	gameComboFrom = 3 // right answers in a row before points double
)

// gameMaxScore: every answer right and instant, combo included.
var gameMaxScore = func() int {
	n := 0
	for i := 1; i <= gameRoundLen; i++ {
		n += gameMaxPoints * comboFactor(i)
	}
	return n
}()

func comboFactor(streak int) int {
	if streak >= gameComboFrom {
		return 2
	}
	return 1
}

// Game modes: the word shown, translations to pick from; the translation
// shown, words to pick from; or the word only heard.
const (
	gameClassic = "classic"
	gameReverse = "reverse"
	gameListen  = "listen"
)

var gameModes = []string{gameClassic, gameReverse, gameListen}

// gameBands: common, middling and rare words. The language's ~100 most
// common words ("the", "a", "was") are too easy to be worth a question.
var gameBands = []store.GameBand{{Min: 101, Max: 2000}, {Min: 2001, Max: 10000}, {Min: 10001, Max: 1 << 30}}

var gameAnyBand = store.GameBand{Min: 1, Max: 1 << 30}

type gameQuestion struct {
	Phrase  string   `json:"phrase"`           // the word in the book's language, spoken aloud
	Prompt  string   `json:"prompt,omitempty"` // what's shown, when not Phrase (reverse mode)
	Choices []string `json:"choices"`
	Answer  int      `json:"answer"` // index in Choices
	VocabID int64    `json:"vocab"`
	OwnerID int64    `json:"owner"`
}

// gamePoints scores one answer before the combo: full points for an instant
// right answer, one less per second taken, nothing once time is up.
func gamePoints(right bool, ms int64) int {
	if !right || ms < 0 || ms >= gameTimeLimit.Milliseconds() {
		return 0
	}
	return max(1, gameMaxPoints-int(ms/1000))
}

type gameAnswer struct {
	Choice int   `json:"choice"`
	MS     int64 `json:"ms"`
}

// gameScore totals a round, combo included, and reports each answer.
func gameScore(qs []gameQuestion, answers []gameAnswer) (score int, right []bool) {
	streak := 0
	right = make([]bool, len(answers))
	for i, a := range answers {
		p := gamePoints(a.Choice == qs[i].Answer, a.MS)
		if p == 0 {
			streak = 0
			continue
		}
		right[i] = true
		streak++
		score += p * comboFactor(streak)
	}
	return score, right
}

// gameRank names a reader's rank from their duel wins.
var gameRanks = []struct {
	Wins int
	Key  string
}{{0, "GameRankBronze"}, {5, "GameRankSilver"}, {15, "GameRankGold"}, {30, "GameRankPlatinum"}, {50, "GameRankOwl"}}

type gameRank struct {
	Name, Next string
	Wins, Need int // Need: wins left to the next rank, 0 at the top
}

func rankFor(T map[string]string, wins int) gameRank {
	r := gameRank{Wins: wins}
	for i, g := range gameRanks {
		if wins >= g.Wins {
			r.Name = T[g.Key]
			r.Next, r.Need = "", 0
			if i+1 < len(gameRanks) {
				r.Next, r.Need = T[gameRanks[i+1].Key], gameRanks[i+1].Wins-wins
			}
		}
	}
	return r
}

// canPlay: the game isn't public yet.
func (s *Server) canPlay(user *store.User) bool {
	if s.isAdmin(user) {
		return true
	}
	_, involved, err := s.store.GameTurns(user.ID)
	return err == nil && involved
}

// gameOptions are a round's settings; the zero value is the one-click
// default: classic mode, auto level, words from both players.
type gameOptions struct {
	Mode   string
	Level  int // 0 auto, else 1-3 = gameBands[Level-1]
	BookID int64
	Missed bool
	IDs    []int64
}

func gameOptionsFrom(r *http.Request) gameOptions {
	o := gameOptions{Mode: r.FormValue("mode")}
	if !slices.Contains(gameModes, o.Mode) {
		o.Mode = gameClassic
	}
	if l, _ := strconv.Atoi(r.FormValue("level")); l >= 1 && l <= len(gameBands) {
		o.Level = l
	}
	switch src := r.FormValue("source"); {
	case src == "missed":
		o.Missed = true
	case strings.HasPrefix(src, "book:"):
		o.BookID, _ = strconv.ParseInt(strings.TrimPrefix(src, "book:"), 10, 64)
	}
	for _, v := range r.Form["word"] {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && len(o.IDs) < gameRoundLen {
			o.IDs = append(o.IDs, id)
		}
	}
	return o
}

// narrowed: the challenger chose where the words come from, so they're all
// theirs.
func (o gameOptions) narrowed() bool { return o.BookID != 0 || o.Missed || len(o.IDs) > 0 }

// gameQuestions builds a round in lang from the words of owners, taking
// turns between them, each word's decoys drawn from its owner's list. All
// words come from one frequency band — the one asked for, or one picked at
// random among those with enough words, from both players in a duel.
func (s *Server) gameQuestions(lang string, o gameOptions, owners ...store.User) ([]gameQuestion, error) {
	if o.narrowed() {
		owners = owners[:1]
	}
	filters := make([]store.GameFilter, len(owners))
	for i := range owners {
		filters[i] = store.GameFilter{Band: gameAnyBand}
	}
	filters[0].BookID, filters[0].Missed, filters[0].IDs = o.BookID, o.Missed, o.IDs
	if len(o.IDs) == 0 {
		band, err := s.gameBand(lang, o, owners, filters)
		if err != nil {
			return nil, err
		}
		for i := range filters {
			filters[i].Band = band
		}
	}
	pools := make([][]store.GameWord, len(owners))
	for i, ow := range owners {
		words, err := s.store.GameWords(ow.ID, lang, filters[i], gameRoundLen*2)
		if err != nil {
			return nil, err
		}
		pools[i] = words
	}
	// A book or the missed words may not fill a round: top up with others.
	if o.BookID != 0 || o.Missed {
		more, err := s.store.GameWords(owners[0].ID, lang, store.GameFilter{Band: gameAnyBand}, gameRoundLen*2)
		if err != nil {
			return nil, err
		}
		pools[0] = append(pools[0], more...)
	}
	reverse := o.Mode == gameReverse
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
		decoys, err := s.store.GameDecoys(owners[i].ID, lang, owners[i].NativeLang, wd, reverse, gameChoices-1)
		if err != nil {
			return nil, err
		}
		if len(decoys) < gameChoices-1 {
			continue
		}
		seen[key] = true
		q := gameQuestion{Phrase: wd.Phrase, VocabID: wd.ID, OwnerID: owners[i].ID}
		answer := wd.Translation
		if reverse {
			q.Prompt, answer = wd.Translation, wd.Phrase
		}
		q.Choices = append(decoys, answer)
		rand.Shuffle(len(q.Choices), func(a, b int) { q.Choices[a], q.Choices[b] = q.Choices[b], q.Choices[a] })
		q.Answer = slices.Index(q.Choices, answer)
		out = append(out, q)
	}
	return out, nil
}

// gameBand picks the round's frequency band: the level asked for if it has
// enough words, else one where the owners have enough between them, each
// owner at least a couple when possible; failing all, any word.
func (s *Server) gameBand(lang string, o gameOptions, owners []store.User, filters []store.GameFilter) (store.GameBand, error) {
	var fair, enough []store.GameBand
	for bi, b := range gameBands {
		total, least := 0, gameRoundLen
		for i, ow := range owners {
			f := filters[i]
			f.Band = b
			n, err := s.store.GameCount(ow.ID, lang, f)
			if err != nil {
				return store.GameBand{}, err
			}
			total += n
			least = min(least, n)
		}
		// A little slack: some words get skipped for want of decoys.
		ok := total >= gameRoundLen+2
		if o.Level == bi+1 && ok {
			return b, nil
		}
		if ok {
			enough = append(enough, b)
			if len(owners) == 1 || least >= 2 {
				fair = append(fair, b)
			}
		}
	}
	switch {
	case o.Level == 0 && len(fair) > 0:
		return fair[rand.IntN(len(fair))], nil
	case o.Level == 0 && len(enough) > 0:
		return enough[rand.IntN(len(enough))], nil
	}
	return gameAnyBand, nil
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
	Rank    gameRank
	Books   []store.GameBook // playlists
	Missed  int              // words in the "missed" playlist
	Levels  []int
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
		if hub.Books, err = s.store.GameBooks(user.ID, hub.Lang, gameRoundLen+2); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		if hub.Missed, err = s.store.GameCount(user.ID, hub.Lang, store.GameFilter{Band: gameAnyBand, Missed: true}); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	for i := range gameBands {
		hub.Levels = append(hub.Levels, i+1)
	}
	wins, err := s.store.GameWins(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	hub.Rank = rankFor(s.dictFor(r), wins)
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
	opts := gameOptionsFrom(r)
	qs, err := s.gameQuestions(lang, opts, owners...)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if len(qs) < gameRoundLen {
		http.Redirect(w, r, "/game?few=1", http.StatusSeeOther)
		return
	}
	raw, _ := json.Marshal(qs)
	id, err := s.store.CreateGameRound(user.ID, opponentID, lang, opts.Mode, string(raw), time.Now())
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/game/rounds/%d", id), http.StatusSeeOther)
}

type gamePlay struct {
	ID        int64
	Lang      string
	Mode      string
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
	view := gamePlay{ID: g.ID, Lang: g.Lang, Mode: g.Mode, Max: gameMaxScore, SecLimit: int(gameTimeLimit / time.Second)}
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
	var answers []gameAnswer
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&answers); err != nil || len(answers) > len(qs) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	score, right := gameScore(qs, answers)
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
	// The player's own words: missed ones come back into review.
	for i, ok := range right {
		if qs[i].OwnerID != user.ID || qs[i].VocabID == 0 {
			continue
		}
		mark := s.store.GameMissed
		if ok {
			mark = func(userID, vocabID int64, _ time.Time) error { return s.store.GameRight(userID, vocabID) }
		}
		if err := mark(user.ID, qs[i].VocabID, now); err != nil {
			s.log.Error("game: word result", "err", err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"score": score, "best": max(best, score), "record": g.OpponentID == 0 && score > best && score > 0})
}

type gamePick struct {
	Lang   string
	Mode   string
	Friend int64
	Words  []store.GameWord
	Need   int
}

// handleGamePick lists the challenger's words to hand-pick a round's five.
func (s *Server) handleGamePick(w http.ResponseWriter, r *http.Request) {
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
	if len(langs) == 0 {
		http.Redirect(w, r, "/game?few=1", http.StatusSeeOther)
		return
	}
	view := gamePick{Lang: langs[0], Mode: gameOptionsFrom(r).Mode, Need: gameRoundLen}
	if l := r.FormValue("lang"); slices.Contains(langs, l) {
		view.Lang = l
	}
	view.Friend, _ = strconv.ParseInt(r.FormValue("friend"), 10, 64)
	if view.Words, err = s.store.GamePickable(user.ID, view.Lang, 300); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.render(w, r, "game_pick.html", s.dictFor(r)["GameTitle"], view)
}
