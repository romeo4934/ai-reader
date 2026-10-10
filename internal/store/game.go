package store

import (
	"strings"
	"time"
)

// GameWord is one of the user's looked-up words, as the word game asks it.
type GameWord struct {
	ID          int64
	Phrase      string
	Translation string
}

// GameWords picks n random words from the user's list in one language.
func (s *Store) GameWords(userID int64, lang string, n int) ([]GameWord, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.phrase, v.translation FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND b.lang_key = ? AND v.translation != ''
		ORDER BY RANDOM() LIMIT ?`, userID, lang, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameWord
	for rows.Next() {
		var w GameWord
		if err := rows.Scan(&w.ID, &w.Phrase, &w.Translation); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GameDecoys returns up to n wrong answers for a word: translations of the
// user's other words in that language first, then — for a short list —
// other readers' with the same native language. None equals exclude.
func (s *Store) GameDecoys(userID int64, lang, nativeLang, exclude string, n int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT v.translation, v.user_id = ? AS mine FROM vocab v
		JOIN books b ON b.id = v.book_id JOIN users u ON u.id = v.user_id
		WHERE b.lang_key = ? AND v.translation != '' AND (v.user_id = ? OR u.native_lang = ?)
		GROUP BY LOWER(v.translation)
		ORDER BY mine DESC, RANDOM() LIMIT ?`, userID, lang, userID, nativeLang, n+5)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		var mine bool
		if err := rows.Scan(&t, &mine); err != nil {
			return nil, err
		}
		if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(exclude)) || len(out) == n {
			continue
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GameTranslations maps the given vocab ids, among the user's own, to their
// translations — to check a finished round's answers.
func (s *Store) GameTranslations(userID int64, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	for _, id := range ids {
		var t string
		err := s.db.QueryRow(`SELECT translation FROM vocab WHERE id = ? AND user_id = ?`, id, userID).Scan(&t)
		if err == nil {
			out[id] = t
		}
	}
	return out, nil
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
