// Package store is the single SQLite-backed persistence layer: books,
// chapters, reading progress, settings, and the vocab deck.
package store

import (
	"database/sql"
	_ "embed"
	"errors"
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
	if err := ensureColumn(db, "reading_progress", "section_idx", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration reading_progress.section_idx : %w", err)
	}
	if err := migrateUsersForEmail(db); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS usage (
		user_id      INTEGER NOT NULL REFERENCES users(id),
		month        TEXT NOT NULL,
		translations INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (user_id, month)
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("table usage : %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS daily_activity (
		user_id   INTEGER NOT NULL REFERENCES users(id),
		day       TEXT NOT NULL,
		reviews   INTEGER NOT NULL DEFAULT 0,
		new_cards INTEGER NOT NULL DEFAULT 0,
		extra_new INTEGER NOT NULL DEFAULT 0,
		completed INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (user_id, day)
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("table daily_activity : %w", err)
	}
	if err := ensureColumn(db, "daily_activity", "points", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration daily_activity.points : %w", err)
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

// migrateUsersForEmail adds what open, email-based signup needs. Accounts
// created before it (username + invite code, no email) keep logging in by
// username, and are put on the unlimited plan: they're the friends the app
// was first built for, not the free tier the quota is meant to bound.
func migrateUsersForEmail(db *sql.DB) error {
	hadPlan, err := hasColumn(db, "users", "plan")
	if err != nil {
		return fmt.Errorf("migration users.plan : %w", err)
	}
	for _, c := range []struct{ name, decl string }{
		{"email", "TEXT"},
		{"email_verified_at", "TEXT NOT NULL DEFAULT ''"},
		{"plan", "TEXT NOT NULL DEFAULT 'free'"},
		{"pending_email", "TEXT NOT NULL DEFAULT ''"},
		{"daily_new_limit", "INTEGER NOT NULL DEFAULT 10"},
		{"display_name", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := ensureColumn(db, "users", c.name, c.decl); err != nil {
			return fmt.Errorf("migration users.%s : %w", c.name, err)
		}
	}
	if !hadPlan {
		if _, err := db.Exec(`UPDATE users SET plan = ?`, PlanUnlimited); err != nil {
			return fmt.Errorf("migration users.plan : %w", err)
		}
	}
	// NULL emails (legacy accounts) don't collide in a UNIQUE index.
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email COLLATE NOCASE)`); err != nil {
		return fmt.Errorf("index idx_users_email : %w", err)
	}
	return nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notnull, pk int
			name, ctype      string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// ensureColumn adds a column to a table already created by an earlier version
// of schema.sql — CREATE TABLE IF NOT EXISTS doesn't alter existing tables.
func ensureColumn(db *sql.DB, table, column, decl string) error {
	has, err := hasColumn(db, table, column)
	if err != nil || has {
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
	// Email is empty for accounts created before email signup.
	Email         string
	EmailVerified bool
	Plan          string
	// PendingEmail is an address an older account asked to add from the
	// settings, waiting for its confirmation link to be clicked. Kept apart
	// from Email so an unconfirmed address never locks the account's login.
	PendingEmail string
	// DailyNewLimit caps how many never-reviewed cards enter the daily
	// review session; the rest wait for the following days.
	DailyNewLimit int
	// DisplayName is the pseudo shown to others on the leaderboard; empty
	// means anonymous for an email account.
	DisplayName string
}

// Plans: "free" is bounded by the monthly translation quota; "unlimited"
// isn't (pre-signup accounts, and later whoever pays).
const (
	PlanFree      = "free"
	PlanUnlimited = "unlimited"
)

const userColumns = `id, username, password_hash, native_lang, created_at, COALESCE(email, ''), email_verified_at != '', plan, pending_email, daily_new_limit, display_name`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var createdAt string
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.NativeLang, &createdAt, &u.Email, &u.EmailVerified, &u.Plan, &u.PendingEmail, &u.DailyNewLimit, &u.DisplayName); err != nil {
		return User{}, err
	}
	u.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return u, nil
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
	return scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE username = ? COLLATE NOCASE`, username))
}

// GetUserByLogin finds the account for what someone typed in the login form:
// an email for accounts made with email signup, a username for older ones.
func (s *Store) GetUserByLogin(login string) (User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE email = ? COLLATE NOCASE OR username = ? COLLATE NOCASE ORDER BY email IS NULL LIMIT 1`, login, login))
}

func (s *Store) GetUserByEmail(email string) (User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE email = ? COLLATE NOCASE`, email))
}

func (s *Store) GetUserByID(id int64) (User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// CreateEmailUser inserts an account from email signup, not yet verified.
// The email doubles as the username, which keeps that column's NOT NULL
// UNIQUE constraint meaningful without asking for a second identifier.
func (s *Store) CreateEmailUser(email, passwordHash, nativeLang string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO users (username, email, password_hash, native_lang, plan, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		email, email, passwordHash, nativeLang, PlanFree, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) MarkEmailVerified(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET email_verified_at = ? WHERE id = ? AND email_verified_at = ''`,
		time.Now().UTC().Format(timeLayout), userID)
	return err
}

func (s *Store) SetPendingEmail(userID int64, email string) error {
	_, err := s.db.Exec(`UPDATE users SET pending_email = ? WHERE id = ?`, email, userID)
	return err
}

// ConfirmPendingEmail makes the pending address the account's verified
// email. Fails on the unique index if someone else took it meanwhile.
func (s *Store) ConfirmPendingEmail(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET email = pending_email, pending_email = '', email_verified_at = ? WHERE id = ? AND pending_email != ''`,
		time.Now().UTC().Format(timeLayout), userID)
	return err
}

func (s *Store) SetPasswordHash(userID int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

func (s *Store) SetUserNativeLang(userID int64, lang string) error {
	_, err := s.db.Exec(`UPDATE users SET native_lang = ? WHERE id = ?`, lang, userID)
	return err
}

func (s *Store) SetDisplayName(userID int64, name string) error {
	_, err := s.db.Exec(`UPDATE users SET display_name = ? WHERE id = ?`, name, userID)
	return err
}

func (s *Store) SetDailyNewLimit(userID int64, n int) error {
	_, err := s.db.Exec(`UPDATE users SET daily_new_limit = ? WHERE id = ?`, n, userID)
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

func (s *Store) SetProgress(bookID int64, chapterIdx, sectionIdx int) error {
	_, err := s.db.Exec(`
		INSERT INTO reading_progress (book_id, chapter_idx, section_idx, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(book_id) DO UPDATE SET chapter_idx = excluded.chapter_idx, section_idx = excluded.section_idx, updated_at = excluded.updated_at`,
		bookID, chapterIdx, sectionIdx, time.Now().UTC().Format(timeLayout))
	return err
}

func (s *Store) GetProgress(bookID int64) (chapterIdx, sectionIdx int, err error) {
	err = s.db.QueryRow(`SELECT chapter_idx, section_idx FROM reading_progress WHERE book_id = ?`, bookID).Scan(&chapterIdx, &sectionIdx)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	return chapterIdx, sectionIdx, err
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
//
// Deliberately NOT filtered by archived: "je le connais" hides a card from
// the browsing list (ListVocab), but box 5's 30-day cycle is still real
// spaced repetition — a mastered word should keep resurfacing occasionally
// to catch it being forgotten, not vanish from review forever.
func (s *Store) DueVocab(userID int64, now time.Time, limit int) ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.user_id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.frequency, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND v.next_review_at <= ?
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
	err := s.db.QueryRow(`SELECT COUNT(*) FROM vocab WHERE user_id = ? AND next_review_at <= ?`,
		userID, now.Format(timeLayout)).Scan(&n)
	return n, err
}

// ListVocab returns the browsing deck — a card marked "je le connais" via
// ArchiveVocab drops out of this list (and starts a 30-day box-5 cycle) but
// keeps coming back in DueVocab and keeps its history in RecentlyReviewed.
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

// DeleteVocab removes a card for good — for a word saved by mistake or a
// function word (his, with, and) that shouldn't come back in review at all,
// unlike ArchiveVocab's 30-day box-5 cycle.
func (s *Store) DeleteVocab(id, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM vocab WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// ArchiveVocab marks a card "already known": it drops out of the /words
// browsing list, but still comes back in /review on its box-5, 30-day cycle
// — "known" declutters the list, it doesn't opt a word out of ever being
// checked again.
func (s *Store) ArchiveVocab(id, userID int64, box int, nextReview, now time.Time) error {
	_, err := s.db.Exec(`UPDATE vocab SET archived = 1, box = ?, next_review_at = ?, last_reviewed_at = ? WHERE id = ? AND user_id = ?`,
		box, nextReview.Format(timeLayout), now.Format(timeLayout), id, userID)
	return err
}

// --- usage (free-plan quota) ---

func usageMonth(now time.Time) string { return now.UTC().Format("2006-01") }

// TranslationsThisMonth is how many AI translations the user has used in the
// current calendar month (UTC).
func (s *Store) TranslationsThisMonth(userID int64, now time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT translations FROM usage WHERE user_id = ? AND month = ?`, userID, usageMonth(now)).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

func (s *Store) CountTranslation(userID int64, now time.Time) error {
	_, err := s.db.Exec(`INSERT INTO usage (user_id, month, translations) VALUES (?, ?, 1)
		ON CONFLICT (user_id, month) DO UPDATE SET translations = translations + 1`, userID, usageMonth(now))
	return err
}

// --- daily challenge ---
//
// A card is "new" until its first review (last_reviewed_at IS NULL). Due
// reviews always make it into the day's session; new cards only up to the
// user's daily limit, so a long reading session doesn't turn into an
// 80-card review the next morning. Days are the user's local calendar days
// ("2006-01-02"), computed by the caller.

type DailyActivity struct {
	Reviews   int
	NewCards  int
	ExtraNew  int
	Completed bool
	Points    int
}

func (s *Store) GetDailyActivity(userID int64, day string) (DailyActivity, error) {
	var a DailyActivity
	err := s.db.QueryRow(`SELECT reviews, new_cards, extra_new, completed, points FROM daily_activity WHERE user_id = ? AND day = ?`,
		userID, day).Scan(&a.Reviews, &a.NewCards, &a.ExtraNew, &a.Completed, &a.Points)
	if errors.Is(err, sql.ErrNoRows) {
		return DailyActivity{}, nil
	}
	return a, err
}

// RecordReview counts one answered card for the day, and the points it
// earned; wasNew when it was the card's first review, which is what the
// daily new-card limit counts.
func (s *Store) RecordReview(userID int64, day string, wasNew bool, points int) error {
	n := 0
	if wasNew {
		n = 1
	}
	_, err := s.db.Exec(`INSERT INTO daily_activity (user_id, day, reviews, new_cards, points) VALUES (?, ?, 1, ?, ?)
		ON CONFLICT (user_id, day) DO UPDATE SET reviews = reviews + 1, new_cards = new_cards + excluded.new_cards,
			points = points + excluded.points`,
		userID, day, n, points)
	return err
}

func (s *Store) TotalPoints(userID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COALESCE(SUM(points), 0) FROM daily_activity WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}

// AddExtraNew raises today's new-card allowance, for someone who finished
// the challenge and wants to keep going.
func (s *Store) AddExtraNew(userID int64, day string, n int) error {
	_, err := s.db.Exec(`INSERT INTO daily_activity (user_id, day, extra_new) VALUES (?, ?, ?)
		ON CONFLICT (user_id, day) DO UPDATE SET extra_new = extra_new + excluded.extra_new`,
		userID, day, n)
	return err
}

func (s *Store) MarkDayCompleted(userID int64, day string) error {
	_, err := s.db.Exec(`INSERT INTO daily_activity (user_id, day, completed) VALUES (?, ?, 1)
		ON CONFLICT (user_id, day) DO UPDATE SET completed = 1`, userID, day)
	return err
}

// CompletedDays returns the days since `since` (inclusive) on which the
// user finished the daily challenge — what the streak is counted from.
func (s *Store) CompletedDays(userID int64, since string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT day FROM daily_activity WHERE user_id = ? AND day >= ? AND completed = 1`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d] = true
	}
	return out, rows.Err()
}

// DeckCounts is what the daily session is built from: due cards already
// reviewed at least once, never-reviewed cards, and the whole deck.
func (s *Store) DeckCounts(userID int64, now time.Time) (dueReviews, newCards, total int, err error) {
	err = s.db.QueryRow(`SELECT
			COALESCE(SUM(last_reviewed_at IS NOT NULL AND next_review_at <= ?), 0),
			COALESCE(SUM(last_reviewed_at IS NULL), 0),
			COUNT(*)
		FROM vocab WHERE user_id = ?`, now.Format(timeLayout), userID).Scan(&dueReviews, &newCards, &total)
	return
}

// NextDailyCards returns up to `limit` upcoming cards of the daily session,
// in order: due reviews first, then (if allowNew) never-reviewed cards, most
// frequent words first within each group. The first is the one to show; the
// rest are what the review page prepares ahead of time.
func (s *Store) NextDailyCards(userID int64, now time.Time, allowNew bool, limit int) ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.user_id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.frequency, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND (
			(v.last_reviewed_at IS NOT NULL AND v.next_review_at <= ?) OR (? AND v.last_reviewed_at IS NULL))
		ORDER BY v.last_reviewed_at IS NULL, v.frequency ASC, v.next_review_at ASC
		LIMIT ?`, userID, now.Format(timeLayout), allowNew, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabRows(rows)
}

type LeaderboardRow struct {
	UserID      int64
	Username    string
	DisplayName string
	Points      int
}

// Leaderboard ranks everyone who earned points between two days (inclusive),
// best first.
func (s *Store) Leaderboard(fromDay, toDay string, limit int) ([]LeaderboardRow, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.display_name, SUM(d.points) AS p
		FROM daily_activity d JOIN users u ON u.id = d.user_id
		WHERE d.day >= ? AND d.day <= ?
		GROUP BY u.id HAVING p > 0
		ORDER BY p DESC, u.id ASC
		LIMIT ?`, fromDay, toDay, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaderboardRow
	for rows.Next() {
		var r LeaderboardRow
		if err := rows.Scan(&r.UserID, &r.Username, &r.DisplayName, &r.Points); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
