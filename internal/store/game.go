package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// GameWord is one of the user's looked-up words, as the word game asks it.
type GameWord struct {
	ID          int64
	Phrase      string
	Translation string
	Frequency   int
}

// GameBand is a range of word frequency ranks (1 = the language's most
// common word), so a round's words are about as hard as each other.
type GameBand struct{ Min, Max int }

// GameFilter narrows the words a round is drawn from: a frequency band, and
// optionally one book, the words missed in earlier rounds, or a hand-picked
// list.
type GameFilter struct {
	Band   GameBand
	BookID int64
	Missed bool
	IDs    []int64
}

func (f GameFilter) where(userID int64, lang string) (string, []any) {
	// Short enough to read at a glance: a word or an expression, not a clause.
	q := `v.user_id = ? AND b.lang_key = ? AND v.translation != '' AND v.frequency BETWEEN ? AND ?
		AND LENGTH(v.phrase) <= 30 AND LENGTH(v.translation) <= 60`
	args := []any{userID, lang, f.Band.Min, f.Band.Max}
	if f.BookID != 0 {
		q += ` AND v.book_id = ?`
		args = append(args, f.BookID)
	}
	if f.Missed {
		q += ` AND v.id IN (SELECT vocab_id FROM game_misses WHERE user_id = ?)`
		args = append(args, userID)
	}
	if len(f.IDs) > 0 {
		q += ` AND v.id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(f.IDs)), ",") + `)`
		for _, id := range f.IDs {
			args = append(args, id)
		}
	}
	return q, args
}

// GameCount counts the user's words in a language that pass the filter.
func (s *Store) GameCount(userID int64, lang string, f GameFilter) (int, error) {
	where, args := f.where(userID, lang)
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM vocab v JOIN books b ON b.id = v.book_id WHERE `+where, args...).Scan(&n)
	return n, err
}

// GameWords picks n random words from the user's list in one language that
// pass the filter.
func (s *Store) GameWords(userID int64, lang string, f GameFilter, n int) ([]GameWord, error) {
	where, args := f.where(userID, lang)
	rows, err := s.db.Query(`
		SELECT v.id, v.phrase, v.translation, v.frequency FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE `+where+` ORDER BY RANDOM() LIMIT ?`, append(args, n)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameWord
	for rows.Next() {
		var w GameWord
		if err := rows.Scan(&w.ID, &w.Phrase, &w.Translation, &w.Frequency); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GamePickable lists the user's words in a language, newest first, for
// hand-picking a duel's words.
func (s *Store) GamePickable(userID int64, lang string, limit int) ([]GameWord, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.phrase, v.translation, v.frequency FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND b.lang_key = ? AND v.translation != ''
		AND LENGTH(v.phrase) <= 30 AND LENGTH(v.translation) <= 60
		ORDER BY v.id DESC LIMIT ?`, userID, lang, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameWord
	for rows.Next() {
		var w GameWord
		if err := rows.Scan(&w.ID, &w.Phrase, &w.Translation, &w.Frequency); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GameBook is a book with enough of the user's words for a playlist.
type GameBook struct {
	ID    int64
	Title string
	Words int
}

func (s *Store) GameBooks(userID int64, lang string, minWords int) ([]GameBook, error) {
	rows, err := s.db.Query(`
		SELECT b.id, b.title, COUNT(*) FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND b.lang_key = ? AND v.translation != ''
		GROUP BY b.id HAVING COUNT(*) >= ? ORDER BY COUNT(*) DESC`, userID, lang, minWords)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameBook
	for rows.Next() {
		var b GameBook
		if err := rows.Scan(&b.ID, &b.Title, &b.Words); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GameDecoys returns up to n wrong answers for a word: words about as common
// as it — the hardest to rule out — from the user's list in that language,
// or other readers' with the same native language. They're translations,
// or with reverse the words themselves (the answer then being the word).
// None equals the right answer.
func (s *Store) GameDecoys(userID int64, lang, nativeLang string, w GameWord, reverse bool, n int) ([]string, error) {
	col, answer := "v.translation", w.Translation
	if reverse {
		col, answer = "v.phrase", w.Phrase
	}
	rows, err := s.db.Query(`
		SELECT answer FROM (
			SELECT `+col+` AS answer, v.frequency FROM vocab v
			JOIN books b ON b.id = v.book_id JOIN users u ON u.id = v.user_id
			WHERE b.lang_key = ? AND v.translation != '' AND v.id != ? AND (v.user_id = ? OR u.native_lang = ?)
			AND LENGTH(v.phrase) <= 30 AND LENGTH(v.translation) <= 60
			GROUP BY LOWER(`+col+`)
			ORDER BY ABS(v.frequency - ?), RANDOM() LIMIT ?)
		ORDER BY RANDOM()`, lang, w.ID, userID, nativeLang, w.Frequency, n*5)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(answer)) || len(out) == n {
			continue
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GameMissed records a word of the user's own missed in a round: it comes
// back into review now (even one marked known) and into the "missed words"
// playlist, until answered right in a round.
func (s *Store) GameMissed(userID, vocabID int64, now time.Time) error {
	ts := now.UTC().Format(timeLayout)
	res, err := s.db.Exec(`UPDATE vocab SET archived = 0,
		next_review_at = CASE WHEN next_review_at > ? THEN ? ELSE next_review_at END
		WHERE id = ? AND user_id = ?`, ts, ts, vocabID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil
	}
	_, err = s.db.Exec(`INSERT INTO game_misses (user_id, vocab_id, missed_at) VALUES (?, ?, ?)
		ON CONFLICT(user_id, vocab_id) DO UPDATE SET missed_at = excluded.missed_at`, userID, vocabID, ts)
	return err
}

// GameRight takes a word answered right off the user's missed words.
func (s *Store) GameRight(userID, vocabID int64) error {
	_, err := s.db.Exec(`DELETE FROM game_misses WHERE user_id = ? AND vocab_id = ?`, userID, vocabID)
	return err
}

// GameWins counts the duels the user won.
func (s *Store) GameWins(userID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM game_rounds WHERE opponent_id != 0
		AND challenger_score IS NOT NULL AND opponent_score IS NOT NULL
		AND ((challenger_id = ? AND challenger_score > opponent_score) OR (opponent_id = ? AND opponent_score > challenger_score))`,
		userID, userID).Scan(&n)
	return n, err
}

// SaveGameScore records a finished round and returns the user's best score
// in that language before it.
func (s *Store) SaveGameScore(userID int64, lang string, score int, now time.Time) (best int, err error) {
	if best, err = s.GameBest(userID, lang); err != nil {
		return 0, err
	}
	_, err = s.db.Exec(`INSERT INTO game_scores (user_id, lang_key, score, played_at) VALUES (?, ?, ?, ?)`,
		userID, lang, score, now.UTC().Format(timeLayout))
	return best, err
}

// GameBest is the user's best score in a language, 0 if never played.
func (s *Store) GameBest(userID int64, lang string) (int, error) {
	var best int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(score), 0) FROM game_scores WHERE user_id = ? AND lang_key = ?`,
		userID, lang).Scan(&best)
	return best, err
}

// GameRound is one round of the word game: solo (OpponentID 0) or a duel.
// Scores are nil until that player has played.
type GameRound struct {
	ID              int64
	ChallengerID    int64
	ChallengerUser  string // username and display_name, for publicName
	ChallengerDisp  string
	OpponentID      int64
	OpponentUser    string
	OpponentDisp    string
	Lang            string
	Mode            string // "classic", "reverse" or "listen"
	Questions       string // JSON, with the right answers
	ChallengerScore *int
	OpponentScore   *int
	CreatedAt       time.Time
}

// ErrAlreadyPlayed: this player's score for the round is already in.
var ErrAlreadyPlayed = errors.New("manche déjà jouée")

func (s *Store) CreateGameRound(challengerID, opponentID int64, lang, mode, questions string, now time.Time) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO game_rounds (challenger_id, opponent_id, lang_key, mode, questions, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		challengerID, opponentID, lang, mode, questions, now.UTC().Format(timeLayout))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const gameRoundColumns = `r.id, r.challenger_id, cu.username, cu.display_name, r.opponent_id,
	COALESCE(ou.username, ''), COALESCE(ou.display_name, ''), r.lang_key, r.mode, r.questions,
	r.challenger_score, r.opponent_score, r.created_at
	FROM game_rounds r JOIN users cu ON cu.id = r.challenger_id LEFT JOIN users ou ON ou.id = r.opponent_id`

func scanGameRound(sc interface{ Scan(...any) error }) (GameRound, error) {
	var g GameRound
	var cs, os sql.NullInt64
	var created string
	if err := sc.Scan(&g.ID, &g.ChallengerID, &g.ChallengerUser, &g.ChallengerDisp, &g.OpponentID,
		&g.OpponentUser, &g.OpponentDisp, &g.Lang, &g.Mode, &g.Questions, &cs, &os, &created); err != nil {
		return GameRound{}, err
	}
	if cs.Valid {
		v := int(cs.Int64)
		g.ChallengerScore = &v
	}
	if os.Valid {
		v := int(os.Int64)
		g.OpponentScore = &v
	}
	g.CreatedAt, _ = time.Parse(timeLayout, created)
	return g, nil
}

// GetGameRound fetches a round one of its two players is looking at; the
// friend only gets to see it once the challenger has played.
func (s *Store) GetGameRound(id, userID int64) (GameRound, error) {
	return scanGameRound(s.db.QueryRow(`SELECT `+gameRoundColumns+`
		WHERE r.id = ? AND (r.challenger_id = ? OR (r.opponent_id = ? AND r.challenger_score IS NOT NULL))`,
		id, userID, userID))
}

// SetGameScore records the user's score for a round, once.
func (s *Store) SetGameScore(id, userID int64, score int, now time.Time) error {
	res, err := s.db.Exec(`UPDATE game_rounds SET
		challenger_score = CASE WHEN challenger_id = ? THEN ? ELSE challenger_score END,
		opponent_score = CASE WHEN challenger_id != ? THEN ? ELSE opponent_score END,
		finished_at = CASE WHEN challenger_id != ? OR opponent_id = 0 THEN ? ELSE finished_at END
		WHERE id = ? AND ((challenger_id = ? AND challenger_score IS NULL)
		              OR (opponent_id = ? AND challenger_score IS NOT NULL AND opponent_score IS NULL))`,
		userID, score, userID, score, userID, now.UTC().Format(timeLayout), id, userID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAlreadyPlayed
	}
	return nil
}

// GameDuels lists the user's duels, newest first: those they challenged
// someone to and have played, and those they were challenged to.
func (s *Store) GameDuels(userID int64, limit int) ([]GameRound, error) {
	rows, err := s.db.Query(`SELECT `+gameRoundColumns+`
		WHERE r.opponent_id != 0 AND r.challenger_score IS NOT NULL AND (r.challenger_id = ? OR r.opponent_id = ?)
		ORDER BY r.id DESC LIMIT ?`, userID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameRound
	for rows.Next() {
		g, err := scanGameRound(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// GameTurns counts the duels waiting for the user to play, and whether they
// were ever in one — which opens the game to them before it's public.
func (s *Store) GameTurns(userID int64) (turns int, involved bool, err error) {
	var n int
	err = s.db.QueryRow(`SELECT
		COALESCE(SUM(opponent_id = ? AND opponent_score IS NULL), 0), COUNT(*)
		FROM game_rounds WHERE challenger_score IS NOT NULL AND opponent_id != 0 AND (opponent_id = ? OR challenger_id = ?)`,
		userID, userID, userID).Scan(&turns, &n)
	return turns, n > 0, err
}

// GameDueled: a and b have played a duel against each other.
func (s *Store) GameDueled(a, b int64) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM game_rounds
		WHERE (challenger_id = ? AND opponent_id = ?) OR (challenger_id = ? AND opponent_id = ?)`, a, b, b, a).Scan(&n)
	return n > 0, err
}
