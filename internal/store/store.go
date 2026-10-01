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
	// WAL mode (below) allows concurrent readers alongside one writer, so cap
	// at a handful of connections rather than 1 — 1 serialized every request
	// in the app behind a single connection, including simple page reads, so
	// one slow request (an epub upload, a translate call) stalled everything
	// else. busy_timeout handles the rare write/write contention.
	db.SetMaxOpenConns(8)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration : %w", err)
	}
	if err := ensureColumn(db, "vocab", "frequency", "INTEGER NOT NULL DEFAULT 3"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration vocab.frequency : %w", err)
	}
	if err := ensureColumn(db, "books", "user_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration books.user_id : %w", err)
	}
	if err := ensureColumn(db, "vocab", "user_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration vocab.user_id : %w", err)
	}
	if err := ensureColumn(db, "vocab", "archived", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration vocab.archived : %w", err)
	}
	// These index the user_id columns just added above — created here rather
	// than in schema.sql so they never run before ensureColumn has had a
	// chance to add the column on an upgraded database.
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_books_user ON books(user_id)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("index idx_books_user : %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_vocab_due ON vocab(user_id, next_review_at)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("index idx_vocab_due : %w", err)
	}
	return &Store{db: db}, nil
}

// ensureColumn adds a column to a table already created by an earlier version
// of schema.sql — CREATE TABLE IF NOT EXISTS doesn't alter existing tables.
func ensureColumn(db *sql.DB, table, column, decl string) error {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notnull, pk int
			name, ctype      string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, decl))
	return err
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

// --- users ---

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	NativeLang   string
	CreatedAt    time.Time
}

// CreateUser inserts a new account. The caller has already hashed the
// password. If this is the very first user, everything created before
// accounts existed (user_id = 0) is claimed for them automatically —
// there's no separate migration step to run by hand.
func (s *Store) CreateUser(username, passwordHash, nativeLang string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, err
	}

	res, err := tx.Exec(`INSERT INTO users (username, password_hash, native_lang, created_at) VALUES (?, ?, ?, ?)`,
		username, passwordHash, nativeLang, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if n == 0 {
		if _, err := tx.Exec(`UPDATE books SET user_id = ? WHERE user_id = 0`, userID); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`UPDATE vocab SET user_id = ? WHERE user_id = 0`, userID); err != nil {
			return 0, err
		}
	}
	return userID, tx.Commit()
}

func (s *Store) GetUserByUsername(username string) (User, error) {
	var u User
	var createdAt string
	err := s.db.QueryRow(`SELECT id, username, password_hash, native_lang, created_at FROM users WHERE username = ? COLLATE NOCASE`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.NativeLang, &createdAt)
	if err != nil {
		return User{}, err
	}
	u.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return u, nil
}

func (s *Store) GetUserByID(id int64) (User, error) {
	var u User
	var createdAt string
	err := s.db.QueryRow(`SELECT id, username, password_hash, native_lang, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.NativeLang, &createdAt)
	if err != nil {
		return User{}, err
	}
	u.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return u, nil
}

func (s *Store) SetUserNativeLang(userID int64, lang string) error {
	_, err := s.db.Exec(`UPDATE users SET native_lang = ? WHERE id = ?`, lang, userID)
	return err
}

// --- books & chapters ---

type Book struct {
	ID           int64
	UserID       int64
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
func (s *Store) InsertBook(userID int64, title, author, language string, chapters []struct{ Title, Content string }) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO books (user_id, title, author, language, added_at) VALUES (?, ?, ?, ?, ?)`,
		userID, title, author, language, time.Now().UTC().Format(timeLayout))
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

func (s *Store) ListBooks(userID int64) ([]Book, error) {
	rows, err := s.db.Query(`
		SELECT b.id, b.user_id, b.title, b.author, b.language, b.added_at, COUNT(c.id)
		FROM books b LEFT JOIN chapters c ON c.book_id = b.id
		WHERE b.user_id = ?
		GROUP BY b.id ORDER BY b.added_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Book
	for rows.Next() {
		var b Book
		var addedAt string
		if err := rows.Scan(&b.ID, &b.UserID, &b.Title, &b.Author, &b.Language, &addedAt, &b.ChapterCount); err != nil {
			return nil, err
		}
		b.AddedAt, _ = time.Parse(timeLayout, addedAt)
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBook fetches a book, scoped to its owner — a wrong userID behaves like
// the book doesn't exist (sql.ErrNoRows), never leaking another user's data.
func (s *Store) GetBook(id, userID int64) (Book, error) {
	var b Book
	var addedAt string
	err := s.db.QueryRow(`
		SELECT b.id, b.user_id, b.title, b.author, b.language, b.added_at, COUNT(c.id)
		FROM books b LEFT JOIN chapters c ON c.book_id = b.id
		WHERE b.id = ? AND b.user_id = ? GROUP BY b.id`, id, userID).
		Scan(&b.ID, &b.UserID, &b.Title, &b.Author, &b.Language, &addedAt, &b.ChapterCount)
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
	UserID         int64
	BookID         int64
	BookTitle      string
	ChapterID      int64
	Phrase         string
	Lemma          string
	Context        string
	Translation    string
	Note           string
	Frequency      int
	Box            int
	NextReviewAt   time.Time
	CreatedAt      time.Time
	LastReviewedAt *time.Time
}

func (s *Store) InsertVocab(v Vocab) (int64, error) {
	now := time.Now().UTC()
	// Frequency is a corpus rank now (1 = most common; up to frequency.NotInList
	// for words outside the list) rather than the old 1-5 scale — just guard
	// against a missing/invalid value rather than clamp to a narrow range.
	freq := v.Frequency
	if freq < 1 {
		freq = 3000
	}
	res, err := s.db.Exec(`
		INSERT INTO vocab (user_id, book_id, chapter_id, phrase, lemma, context, translation, note, frequency, box, next_review_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		v.UserID, v.BookID, v.ChapterID, v.Phrase, v.Lemma, v.Context, v.Translation, v.Note, freq,
		now.Format(timeLayout), now.Format(timeLayout))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FindVocabByPhrase returns the id of an existing card for this phrase for
// this user (case-insensitive), so looking a word up twice doesn't create
// two cards — scoped per user so two people don't collide on a common word.
func (s *Store) FindVocabByPhrase(userID int64, phrase string) (id int64, found bool, err error) {
	err = s.db.QueryRow(`SELECT id FROM vocab WHERE user_id = ? AND phrase = ? COLLATE NOCASE LIMIT 1`, userID, phrase).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		return 0, false, nil
	case err != nil:
		return 0, false, err
	default:
		return id, true, nil
	}
}

// DueVocab returns up to `limit` due cards for this user, most frequent
// words first — the point of the deck is to spend review time where it pays
// off in reading, not on a word that showed up once.
func (s *Store) DueVocab(userID int64, now time.Time, limit int) ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.user_id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.frequency, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND v.archived = 0 AND v.next_review_at <= ?
		ORDER BY v.frequency ASC, v.next_review_at ASC
		LIMIT ?`, userID, now.Format(timeLayout), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabRows(rows)
}

func (s *Store) CountDueVocab(userID int64, now time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM vocab WHERE user_id = ? AND archived = 0 AND next_review_at <= ?`,
		userID, now.Format(timeLayout)).Scan(&n)
	return n, err
}

// ListVocab returns the active (non-archived) deck — a card marked "je le
// connais" via ArchiveVocab drops out of both this and DueVocab, but stays
// in the database so RecentlyReviewed keeps its history.
func (s *Store) ListVocab(userID int64) ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.user_id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.frequency, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND v.archived = 0
		ORDER BY v.frequency ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabRows(rows)
}

// RecentlyReviewed returns up to `limit` cards this user has actually
// reviewed at least once, most recent first.
func (s *Store) RecentlyReviewed(userID int64, limit int) ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.user_id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.frequency, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND v.last_reviewed_at IS NOT NULL
		ORDER BY v.last_reviewed_at DESC
		LIMIT ?`, userID, limit)
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
		if err := rows.Scan(&v.ID, &v.UserID, &v.BookID, &v.BookTitle, &v.ChapterID, &v.Phrase, &v.Lemma, &v.Context,
			&v.Translation, &v.Note, &v.Frequency, &v.Box, &nextReview, &created, &lastReviewed); err != nil {
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

// GetVocab fetches a card scoped to its owner — a wrong userID behaves like
// the card doesn't exist.
func (s *Store) GetVocab(id, userID int64) (Vocab, error) {
	var v Vocab
	var nextReview, created string
	var lastReviewed sql.NullString
	err := s.db.QueryRow(`
		SELECT v.id, v.user_id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.frequency, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id WHERE v.id = ? AND v.user_id = ?`, id, userID).
		Scan(&v.ID, &v.UserID, &v.BookID, &v.BookTitle, &v.ChapterID, &v.Phrase, &v.Lemma, &v.Context,
			&v.Translation, &v.Note, &v.Frequency, &v.Box, &nextReview, &created, &lastReviewed)
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

func (s *Store) UpdateVocabReview(id, userID int64, box int, nextReview, now time.Time) error {
	_, err := s.db.Exec(`UPDATE vocab SET box = ?, next_review_at = ?, last_reviewed_at = ? WHERE id = ? AND user_id = ?`,
		box, nextReview.Format(timeLayout), now.Format(timeLayout), id, userID)
	return err
}

// ArchiveVocab marks a card "already known" — it drops out of /words and
// /review but the row stays, so review history isn't lost. box/nextReview
// are set to the mastered (box 5) state too, so the data stays consistent
// with the Leitner system rather than introducing a separate notion of
// "known" the box number doesn't reflect.
func (s *Store) ArchiveVocab(id, userID int64, box int, nextReview, now time.Time) error {
	_, err := s.db.Exec(`UPDATE vocab SET archived = 1, box = ?, next_review_at = ?, last_reviewed_at = ? WHERE id = ? AND user_id = ?`,
		box, nextReview.Format(timeLayout), now.Format(timeLayout), id, userID)
	return err
}
