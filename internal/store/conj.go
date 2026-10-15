package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/romeo4934/ai-reader/internal/srs"
)

// ConjTable gives a verb's kept table (tense ID -> forms); ok is false when
// it was never worked out.
func (s *Store) ConjTable(lang, verb string) (forms map[string][]string, ok bool, err error) {
	var raw string
	err = s.db.QueryRow(`SELECT forms FROM conj_tables WHERE lang_key = ? AND verb = ?`, lang, verb).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal([]byte(raw), &forms); err != nil {
		return nil, false, err
	}
	return forms, true, nil
}

// ConjVerbs lists the verbs of a language worked out: true for a table,
// false for a word found not to be a verb.
func (s *Store) ConjVerbs(lang string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT verb, forms != 'null' FROM conj_tables WHERE lang_key = ?`, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		var isVerb bool
		if err := rows.Scan(&v, &isVerb); err != nil {
			return nil, err
		}
		out[v] = isVerb
	}
	return out, rows.Err()
}

// SetConjTable keeps a verb's table; nil forms mark a word that isn't a
// verb, so it isn't asked about again.
func (s *Store) SetConjTable(lang, verb string, forms map[string][]string, now time.Time) error {
	raw := "null"
	if forms != nil {
		b, err := json.Marshal(forms)
		if err != nil {
			return err
		}
		raw = string(b)
	}
	_, err := s.db.Exec(`INSERT INTO conj_tables (lang_key, verb, forms, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (lang_key, verb) DO UPDATE SET forms = excluded.forms, created_at = excluded.created_at`,
		lang, verb, raw, now.UTC().Format(timeLayout))
	return err
}

// OwnVerb is a verb a reader looked up in one of their books.
type OwnVerb struct {
	Infinitive, Translation string
	Phrase, Context, Book   string
}

// OwnVerbs lists the verbs a reader looked up in a language: the words
// whose forms were worked out ("ride · rode · ridden", "poder · passé
// simple" — the infinitive first), the latest look-up of each.
func (s *Store) OwnVerbs(userID int64, lang string) ([]OwnVerb, error) {
	rows, err := s.db.Query(`SELECT v.forms, v.translation, v.phrase, v.context, b.title
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND b.lang_key = ? AND v.forms IS NOT NULL AND v.forms != ''
		ORDER BY v.created_at DESC`, userID, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OwnVerb
	seen := map[string]bool{}
	for rows.Next() {
		var forms string
		var v OwnVerb
		if err := rows.Scan(&forms, &v.Translation, &v.Phrase, &v.Context, &v.Book); err != nil {
			return nil, err
		}
		inf, _, _ := strings.Cut(forms, "·")
		inf = strings.TrimSpace(inf)
		if inf == "" || len(strings.Fields(inf)) > 3 || seen[strings.ToLower(inf)] {
			continue
		}
		seen[strings.ToLower(inf)] = true
		v.Infinitive = inf
		out = append(out, v)
	}
	return out, rows.Err()
}

// ConjItem is a form to drill again.
type ConjItem struct {
	Verb, Tense string
	Person      int
}

// ConjDue lists a reader's missed forms due again, the oldest first.
func (s *Store) ConjDue(userID int64, lang string, now time.Time, limit int) ([]ConjItem, error) {
	rows, err := s.db.Query(`SELECT verb, tense, person FROM conj_items
		WHERE user_id = ? AND lang_key = ? AND due_at <= ? ORDER BY due_at LIMIT ?`,
		userID, lang, now.UTC().Format(timeLayout), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConjItem
	for rows.Next() {
		var it ConjItem
		if err := rows.Scan(&it.Verb, &it.Tense, &it.Person); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ConjAnswered records an answer: a miss puts the form back in box 1, due
// right away (the next series); a right answer on a form being drilled
// moves it up a box, and out once known.
func (s *Store) ConjAnswered(userID int64, lang string, it ConjItem, ok bool, now time.Time) error {
	ts := now.UTC().Format(timeLayout)
	if _, err := s.db.Exec(`INSERT INTO conj_answers (user_id, lang_key, tense, ok, answered_at) VALUES (?, ?, ?, ?, ?)`,
		userID, lang, it.Tense, ok, ts); err != nil {
		return err
	}
	if !ok {
		_, err := s.db.Exec(`INSERT INTO conj_items (user_id, lang_key, verb, tense, person, box, due_at) VALUES (?, ?, ?, ?, ?, 1, ?)
			ON CONFLICT (user_id, lang_key, verb, tense, person) DO UPDATE SET box = 1, due_at = excluded.due_at`,
			userID, lang, it.Verb, it.Tense, it.Person, ts)
		return err
	}
	var box int
	err := s.db.QueryRow(`SELECT box FROM conj_items WHERE user_id = ? AND lang_key = ? AND verb = ? AND tense = ? AND person = ?`,
		userID, lang, it.Verb, it.Tense, it.Person).Scan(&box)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if box >= srs.MaxBox {
		_, err = s.db.Exec(`DELETE FROM conj_items WHERE user_id = ? AND lang_key = ? AND verb = ? AND tense = ? AND person = ?`,
			userID, lang, it.Verb, it.Tense, it.Person)
		return err
	}
	next, due := srs.Next(box, srs.Good, now)
	_, err = s.db.Exec(`UPDATE conj_items SET box = ?, due_at = ? WHERE user_id = ? AND lang_key = ? AND verb = ? AND tense = ? AND person = ?`,
		next, due.UTC().Format(timeLayout), userID, lang, it.Verb, it.Tense, it.Person)
	return err
}

// ConjScore is how a reader does in a tense, over their last answers.
type ConjScore struct {
	Right, Total int
}

const conjScoreOver = 30

// ConjScores gives a reader's score per tense in a language, over their
// last conjScoreOver answers in each.
func (s *Store) ConjScores(userID int64, lang string) (map[string]ConjScore, error) {
	rows, err := s.db.Query(`SELECT tense, SUM(ok), COUNT(*) FROM (
			SELECT tense, ok, ROW_NUMBER() OVER (PARTITION BY tense ORDER BY answered_at DESC) AS n
			FROM conj_answers WHERE user_id = ? AND lang_key = ?)
		WHERE n <= ? GROUP BY tense`, userID, lang, conjScoreOver)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ConjScore{}
	for rows.Next() {
		var t string
		var sc ConjScore
		if err := rows.Scan(&t, &sc.Right, &sc.Total); err != nil {
			return nil, err
		}
		out[t] = sc
	}
	return out, rows.Err()
}

// ConjTenses gives the tenses a reader drills in a language (nil: never
// chosen).
func (s *Store) ConjTenses(userID int64, lang string) ([]string, error) {
	var raw string
	err := s.db.QueryRow(`SELECT tenses FROM conj_prefs WHERE user_id = ? AND lang_key = ?`, userID, lang).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Split(raw, ","), nil
}

func (s *Store) SetConjTenses(userID int64, lang string, tenses []string) error {
	_, err := s.db.Exec(`INSERT INTO conj_prefs (user_id, lang_key, tenses) VALUES (?, ?, ?)
		ON CONFLICT (user_id, lang_key) DO UPDATE SET tenses = excluded.tenses`,
		userID, lang, strings.Join(tenses, ","))
	return err
}
