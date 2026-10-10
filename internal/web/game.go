package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/romeo4934/ai-reader/internal/auth"
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
// A challenge can also go out by link, to anyone: each one who takes it up
// gets their own duel against the challenger, and someone without an
// account plays first and signs up (through the challenger's invite, so
// they become friends) to see who won. And every day brings a daily
// challenge: the same questions for all, and a ranking.

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
		mrand.Shuffle(len(q.Choices), func(a, b int) { q.Choices[a], q.Choices[b] = q.Choices[b], q.Choices[a] })
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
		return fair[mrand.IntN(len(fair))], nil
	case o.Level == 0 && len(enough) > 0:
		return enough[mrand.IntN(len(enough))], nil
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
	TooFew  bool
	Turns   []gameDuel
	Waiting []gameDuel
	Done    []gameDuel
	Friends []gameFriend
	Rank    gameRank
	Daily   gameDailyView
	Books   []store.GameBook // playlists
	Missed  int              // words in the "missed" playlist
	Levels  []int
}

func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	hub := gameHub{Max: gameMaxScore, TooFew: r.URL.Query().Get("few") != ""}
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
	if hub.Lang != "" {
		if hub.Daily, err = s.gameDaily(user, hub.Lang); err != nil {
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
	open := r.FormValue("friend") == "link"
	if fid, _ := strconv.ParseInt(r.FormValue("friend"), 10, 64); fid != 0 {
		ok, err := s.store.AreFriends(user.ID, fid)
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
	if open {
		if err := s.store.SetGameOpenToken(id, newGameToken()); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
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
	ScoreURL  string
	ShareURL  string // shared with the result: the game, or the link challenge
	Link      bool   // a link challenge, ShareURL being its link
	Takers    int    // how many took it up
	Daily     bool
	MyScore   int
	Guest     string // without an account: the challenger's name
}

func gamePlayFor(g store.GameRound) (gamePlay, error) {
	view := gamePlay{ID: g.ID, Lang: g.Lang, Mode: g.Mode, Max: gameMaxScore, SecLimit: int(gameTimeLimit / time.Second),
		ScoreURL: fmt.Sprintf("/game/rounds/%d/score", g.ID)}
	return view, json.Unmarshal([]byte(g.Questions), &view.Questions)
}

func newGameToken() string {
	b := make([]byte, 9)
	rand.Read(b)
	return hex.EncodeToString(b)
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
	view, err := gamePlayFor(g)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	view.ShareURL = s.cfg.BaseURL + "/game"
	if g.ChallengerScore != nil {
		view.MyScore = *g.ChallengerScore
	}
	switch {
	case g.OpponentID != 0:
		d := duelFor(g, user.ID)
		view.Duel = &d
		view.Played = d.Mine != nil
	case g.OpenToken != "":
		view.Played = g.ChallengerScore != nil
		view.ShareURL, view.Link = s.cfg.BaseURL+"/game/c/"+g.OpenToken, true
		if view.Takers, err = s.store.GameLinkTakers(g.ID); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	default:
		view.Played = g.ChallengerScore != nil
		view.Daily = g.Daily != ""
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
	if g.Solo() {
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
	if g.Solo() {
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
	json.NewEncoder(w).Encode(map[string]any{"score": score, "best": max(best, score), "record": g.Solo() && score > best && score > 0})
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

// --- link challenges ---

// gameGuestCookie keeps the score of someone who played a link challenge
// before having an account, until they sign up or log in.
const gameGuestCookie = "game_guest"

// handleGameLink opens a link challenge: the challenger sees their own
// round, a reader gets their own duel (and the challenger as a friend, for
// rematches), and someone without an account plays it straight away.
func (s *Server) handleGameLink(w http.ResponseWriter, r *http.Request) {
	src, err := s.store.GameOpenRound(r.PathValue("token"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r, ok := s.withSessionUser(r); ok {
		user := userFromContext(r)
		if user.ID == src.ChallengerID {
			http.Redirect(w, r, fmt.Sprintf("/game/rounds/%d", src.ID), http.StatusSeeOther)
			return
		}
		id, err := s.takeGameChallenge(src, user.ID, -1)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/game/rounds/%d", id), http.StatusSeeOther)
		return
	}
	view, err := gamePlayFor(src)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	view.ScoreURL = "/game/c/" + src.OpenToken + "/score"
	view.ShareURL = s.cfg.BaseURL + "/game"
	view.Guest = publicName(src.ChallengerID, src.ChallengerUser, src.ChallengerDisp)
	T := s.visitorDict(w, r)
	s.renderDict(w, r, T, "game_play.html", T["GameTitle"], view)
}

func (s *Server) takeGameChallenge(src store.GameRound, userID int64, score int) (int64, error) {
	id, err := s.store.TakeGameChallenge(src, userID, score, time.Now())
	if err != nil {
		return 0, err
	}
	return id, s.store.AddFriendship(userID, src.ChallengerID)
}

// handleGameLinkScore scores a link challenge played without an account and
// keeps the score in a signed cookie; the answer points to the challenger's
// invite to sign up and see who won.
func (s *Server) handleGameLinkScore(w http.ResponseWriter, r *http.Request) {
	src, err := s.store.GameOpenRound(r.PathValue("token"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var qs []gameQuestion
	if err := json.Unmarshal([]byte(src.Questions), &qs); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	var answers []gameAnswer
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&answers); err != nil || len(answers) > len(qs) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	score, _ := gameScore(qs, answers)
	token := auth.SignLink(s.secret, auth.PurposeGameGuest, src.ID, strconv.Itoa(score), 30*24*time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name: gameGuestCookie, Value: strconv.Itoa(score) + ":" + token, Path: "/", MaxAge: 30 * 24 * 3600,
		HttpOnly: true, Secure: r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"score": score, "signup": s.inviteLink(src.ChallengerID)})
}

// claimGameGuest turns the score of a link challenge played before signing
// up into the reader's duel, once they're logged in. Returns the duel's id.
func (s *Server) claimGameGuest(w http.ResponseWriter, r *http.Request, user *store.User) (int64, bool) {
	c, err := r.Cookie(gameGuestCookie)
	if err != nil {
		return 0, false
	}
	http.SetCookie(w, &http.Cookie{Name: gameGuestCookie, Value: "", Path: "/", MaxAge: -1})
	scoreStr, token, ok := strings.Cut(c.Value, ":")
	score, err := strconv.Atoi(scoreStr)
	if !ok || err != nil || !auth.VerifyLink(s.secret, auth.PurposeGameGuest, token, scoreStr) {
		return 0, false
	}
	roundID, _ := auth.LinkUserID(token)
	src, err := s.store.GameRoundByID(roundID)
	if err != nil || src.OpenToken == "" || src.ChallengerID == user.ID {
		return 0, false
	}
	id, err := s.takeGameChallenge(src, user.ID, score)
	if err != nil {
		s.log.Error("game: claim guest score", "err", err)
		return 0, false
	}
	return id, true
}

// --- daily challenge ---

type gameDailyEntry struct {
	Rank  int
	Name  string
	Score int
	Me    bool
}

type gameDailyView struct {
	Lang    string
	Played  bool
	Score   int
	Ranking []gameDailyEntry
	Players int
}

func gameDay(now time.Time) string { return now.UTC().Format("2006-01-02") }

func (s *Server) gameDaily(user *store.User, lang string) (gameDailyView, error) {
	v := gameDailyView{Lang: lang}
	day := gameDay(time.Now())
	if g, err := s.store.GameDailyRound(user.ID, day, lang); err == nil && g.ChallengerScore != nil {
		v.Played, v.Score = true, *g.ChallengerScore
	}
	entries, err := s.store.GameDailyRanking(day, lang, user.NativeLang, 100)
	if err != nil {
		return v, err
	}
	v.Players = len(entries)
	for i, e := range entries {
		if i < 10 || e.UserID == user.ID {
			v.Ranking = append(v.Ranking, gameDailyEntry{Rank: i + 1, Name: publicName(e.UserID, e.Username, e.Display), Score: e.Score, Me: e.UserID == user.ID})
		}
	}
	return v, nil
}

// gameDailyQuestions builds — once per day, language and native language —
// the daily challenge, from every such reader's words.
func (s *Server) gameDailyQuestions(day, lang, nativeLang string) (string, error) {
	if q, err := s.store.GameDailyQuestions(day, lang, nativeLang); err != nil || q != "" {
		return q, err
	}
	words, owners, err := s.store.GameDailyWords(lang, nativeLang, store.GameBand{Min: gameBands[0].Min, Max: gameAnyBand.Max}, gameRoundLen*3)
	if err != nil {
		return "", err
	}
	var qs []gameQuestion
	for i, wd := range words {
		if len(qs) == gameRoundLen {
			break
		}
		decoys, err := s.store.GameDecoys(owners[i], lang, nativeLang, wd, false, gameChoices-1)
		if err != nil {
			return "", err
		}
		if len(decoys) < gameChoices-1 {
			continue
		}
		q := gameQuestion{Phrase: wd.Phrase, VocabID: wd.ID, OwnerID: owners[i], Choices: append(decoys, wd.Translation)}
		mrand.Shuffle(len(q.Choices), func(a, b int) { q.Choices[a], q.Choices[b] = q.Choices[b], q.Choices[a] })
		q.Answer = slices.Index(q.Choices, wd.Translation)
		qs = append(qs, q)
	}
	if len(qs) < gameRoundLen {
		return "", nil
	}
	raw, _ := json.Marshal(qs)
	return s.store.SetGameDailyQuestions(day, lang, nativeLang, string(raw))
}

// handleGameDaily starts (or resumes) the reader's daily challenge.
func (s *Server) handleGameDaily(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	lang := r.FormValue("lang")
	if !validLangKey(lang) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	day := gameDay(time.Now())
	if g, err := s.store.GameDailyRound(user.ID, day, lang); err == nil {
		http.Redirect(w, r, fmt.Sprintf("/game/rounds/%d", g.ID), http.StatusSeeOther)
		return
	}
	qs, err := s.gameDailyQuestions(day, lang, user.NativeLang)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if qs == "" {
		http.Redirect(w, r, "/game?few=1&lang="+lang, http.StatusSeeOther)
		return
	}
	id, err := s.store.CreateGameDailyRound(user.ID, day, lang, qs, time.Now())
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/game/rounds/%d", id), http.StatusSeeOther)
}
