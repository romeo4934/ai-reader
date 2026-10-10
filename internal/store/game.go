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

// GameBandCount counts the user's words in a language within a band.
func (s *Store) GameBandCount(userID int64, lang string, b GameBand) (int, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND b.lang_key = ? AND v.translation != '' AND v.frequency BETWEEN ? AND ?`,
		userID, lang, b.Min, b.Max).Scan(&n)
	return n, err
}

// GameWords picks n random words from the user's list in one language and
// frequency band.
func (s *Store) GameWords(userID int64, lang string, b GameBand, n int) ([]GameWord, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.phrase, v.translation, v.frequency FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND b.lang_key = ? AND v.translation != '' AND v.frequency BETWEEN ? AND ?
		ORDER BY RANDOM() LIMIT ?`, userID, lang, b.Min, b.Max, n)
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

// GameDecoys returns up to n wrong answers for a word: translations of
// words about as common as it — the hardest to rule out — from the user's
// list in that language, or other readers' with the same native language.
// None equals the right answer.
func (s *Store) GameDecoys(userID int64, lang, nativeLang string, w GameWord, n int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT translation FROM (
			SELECT v.translation, v.frequency FROM vocab v
			JOIN books b ON b.id = v.book_id JOIN users u ON u.id = v.user_id
			WHERE b.lang_key = ? AND v.translation != '' AND v.id != ? AND (v.user_id = ? OR u.native_lang = ?)
			GROUP BY LOWER(v.translation)
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
		if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(w.Translation)) || len(out) == n {
			continue
		}
		out = append(out, t)
	}
	return out, rows.Err()
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
	Questions       string // JSON, with the right answers
	ChallengerScore *int
	OpponentScore   *int
	CreatedAt       time.Time
}

// ErrAlreadyPlayed: this player's score for the round is already in.
var ErrAlreadyPlayed = errors.New("manche déjà jouée")

func (s *Store) CreateGameRound(challengerID, opponentID int64, lang, questions string, now time.Time) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO game_rounds (challenger_id, opponent_id, lang_key, questions, created_at) VALUES (?, ?, ?, ?, ?)`,
		challengerID, opponentID, lang, questions, now.UTC().Format(timeLayout))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const gameRoundColumns = `r.id, r.challenger_id, cu.username, cu.display_name, r.opponent_id,
	COALESCE(ou.username, ''), COALESCE(ou.display_name, ''), r.lang_key, r.questions,
	r.challenger_score, r.opponent_score, r.created_at
	FROM game_rounds r JOIN users cu ON cu.id = r.challenger_id LEFT JOIN users ou ON ou.id = r.opponent_id`

func scanGameRound(sc interface{ Scan(...any) error }) (GameRound, error) {
	var g GameRound
	var cs, os sql.NullInt64
	var created string
	if err := sc.Scan(&g.ID, &g.ChallengerID, &g.ChallengerUser, &g.ChallengerDisp, &g.OpponentID,
		&g.OpponentUser, &g.OpponentDisp, &g.Lang, &g.Questions, &cs, &os, &created); err != nil {
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
