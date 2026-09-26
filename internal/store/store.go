// Package store is the single SQLite-backed persistence layer: books,
// chapters, reading progress, settings, and the vocab deck.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

const timeLayout = time.RFC3339

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // modernc.org/sqlite: one writer, avoids "database is locked"
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration : %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// --- settings ---

func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// --- books & chapters ---

type Book struct {
	ID           int64
	Title        string
	Author       string
	Language     string
	AddedAt      time.Time
	ChapterCount int
}

type Chapter struct {
	ID      int64
	BookID  int64
	Idx     int
	Title   string
	Content string
}

// InsertBook stores a book and all of its chapters (idx 0-based, in reading order).
func (s *Store) InsertBook(title, author, language string, chapters []struct{ Title, Content string }) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO books (title, author, language, added_at) VALUES (?, ?, ?, ?)`,
		title, author, language, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, err
	}
	bookID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for i, ch := range chapters {
		if _, err := tx.Exec(`INSERT INTO chapters (book_id, idx, title, content) VALUES (?, ?, ?, ?)`,
			bookID, i, ch.Title, ch.Content); err != nil {
			return 0, err
		}
	}
	return bookID, tx.Commit()
}

func (s *Store) ListBooks() ([]Book, error) {
	rows, err := s.db.Query(`
		SELECT b.id, b.title, b.author, b.language, b.added_at, COUNT(c.id)
		FROM books b LEFT JOIN chapters c ON c.book_id = b.id
		GROUP BY b.id ORDER BY b.added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Book
	for rows.Next() {
		var b Book
		var addedAt string
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Language, &addedAt, &b.ChapterCount); err != nil {
			return nil, err
		}
		b.AddedAt, _ = time.Parse(timeLayout, addedAt)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) GetBook(id int64) (Book, error) {
	var b Book
	var addedAt string
	err := s.db.QueryRow(`
		SELECT b.id, b.title, b.author, b.language, b.added_at, COUNT(c.id)
		FROM books b LEFT JOIN chapters c ON c.book_id = b.id
		WHERE b.id = ? GROUP BY b.id`, id).
		Scan(&b.ID, &b.Title, &b.Author, &b.Language, &addedAt, &b.ChapterCount)
	if err != nil {
		return Book{}, err
	}
	b.AddedAt, _ = time.Parse(timeLayout, addedAt)
	return b, nil
}

func (s *Store) GetChapterByIdx(bookID int64, idx int) (Chapter, error) {
	var c Chapter
	err := s.db.QueryRow(`SELECT id, book_id, idx, title, content FROM chapters WHERE book_id = ? AND idx = ?`,
		bookID, idx).Scan(&c.ID, &c.BookID, &c.Idx, &c.Title, &c.Content)
	return c, err
}

func (s *Store) GetChapter(id int64) (Chapter, error) {
	var c Chapter
	err := s.db.QueryRow(`SELECT id, book_id, idx, title, content FROM chapters WHERE id = ?`, id).
		Scan(&c.ID, &c.BookID, &c.Idx, &c.Title, &c.Content)
	return c, err
}

// --- reading progress ---

func (s *Store) SetProgress(bookID int64, chapterIdx int) error {
	_, err := s.db.Exec(`
		INSERT INTO reading_progress (book_id, chapter_idx, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(book_id) DO UPDATE SET chapter_idx = excluded.chapter_idx, updated_at = excluded.updated_at`,
		bookID, chapterIdx, time.Now().UTC().Format(timeLayout))
	return err
}

func (s *Store) GetProgress(bookID int64) (int, error) {
	var idx int
	err := s.db.QueryRow(`SELECT chapter_idx FROM reading_progress WHERE book_id = ?`, bookID).Scan(&idx)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return idx, err
}

// --- vocab / review deck ---

type Vocab struct {
	ID             int64
	BookID         int64
	BookTitle      string
	ChapterID      int64
	Phrase         string
	Lemma          string
	Context        string
	Translation    string
	Note           string
	Box            int
	NextReviewAt   time.Time
	CreatedAt      time.Time
	LastReviewedAt *time.Time
}

func (s *Store) InsertVocab(v Vocab) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.Exec(`
		INSERT INTO vocab (book_id, chapter_id, phrase, lemma, context, translation, note, box, next_review_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		v.BookID, v.ChapterID, v.Phrase, v.Lemma, v.Context, v.Translation, v.Note,
		now.Format(timeLayout), now.Format(timeLayout))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DueVocab returns up to `limit` cards whose next_review_at has passed, oldest first.
func (s *Store) DueVocab(now time.Time, limit int) ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.next_review_at <= ?
		ORDER BY v.next_review_at ASC
		LIMIT ?`, now.Format(timeLayout), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabRows(rows)
}

func (s *Store) CountDueVocab(now time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM vocab WHERE next_review_at <= ?`, now.Format(timeLayout)).Scan(&n)
	return n, err
}

func (s *Store) ListVocab() ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		ORDER BY v.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabRows(rows)
}

func scanVocabRows(rows *sql.Rows) ([]Vocab, error) {
	var out []Vocab
	for rows.Next() {
		var v Vocab
		var nextReview, created string
		var lastReviewed sql.NullString
		if err := rows.Scan(&v.ID, &v.BookID, &v.BookTitle, &v.ChapterID, &v.Phrase, &v.Lemma, &v.Context,
			&v.Translation, &v.Note, &v.Box, &nextReview, &created, &lastReviewed); err != nil {
			return nil, err
		}
		v.NextReviewAt, _ = time.Parse(timeLayout, nextReview)
		v.CreatedAt, _ = time.Parse(timeLayout, created)
		if lastReviewed.Valid {
			t, _ := time.Parse(timeLayout, lastReviewed.String)
			v.LastReviewedAt = &t
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) GetVocab(id int64) (Vocab, error) {
	var v Vocab
	var nextReview, created string
	var lastReviewed sql.NullString
	err := s.db.QueryRow(`
		SELECT v.id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id WHERE v.id = ?`, id).
		Scan(&v.ID, &v.BookID, &v.BookTitle, &v.ChapterID, &v.Phrase, &v.Lemma, &v.Context,
			&v.Translation, &v.Note, &v.Box, &nextReview, &created, &lastReviewed)
	if err != nil {
		return Vocab{}, err
	}
	v.NextReviewAt, _ = time.Parse(timeLayout, nextReview)
	v.CreatedAt, _ = time.Parse(timeLayout, created)
	if lastReviewed.Valid {
		t, _ := time.Parse(timeLayout, lastReviewed.String)
		v.LastReviewedAt = &t
	}
	return v, nil
}

func (s *Store) UpdateVocabReview(id int64, box int, nextReview, now time.Time) error {
	_, err := s.db.Exec(`UPDATE vocab SET box = ?, next_review_at = ?, last_reviewed_at = ? WHERE id = ?`,
		box, nextReview.Format(timeLayout), now.Format(timeLayout), id)
	return err
}
