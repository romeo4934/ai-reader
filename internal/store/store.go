// Package store is the single SQLite-backed persistence layer: books,
// chapters, reading progress, settings, and the vocab deck.
package store

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
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
	if err := ensureColumn(db, "books", "source", "TEXT NOT NULL DEFAULT ''"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration books.source : %w", err)
	}
	if err := migrateBookLangKeys(db); err != nil {
		db.Close()
		return nil, err
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
	if err := migratePointsByLang(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := mergeDuplicateLemmas(db); err != nil {
		db.Close()
		return nil, err
	}
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS friends (
			user_id    INTEGER NOT NULL REFERENCES users(id),
			friend_id  INTEGER NOT NULL REFERENCES users(id),
			created_at TEXT NOT NULL,
			PRIMARY KEY (user_id, friend_id)
		)`,
		`CREATE TABLE IF NOT EXISTS invites (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			inviter_id  INTEGER NOT NULL REFERENCES users(id),
			email       TEXT NOT NULL,
			created_at  TEXT NOT NULL,
			accepted_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_invites_inviter ON invites(inviter_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_invites_email ON invites(email COLLATE NOCASE)`,
		`CREATE TABLE IF NOT EXISTS ai_usage (
			day         TEXT NOT NULL,
			user_id     INTEGER NOT NULL,
			kind        TEXT NOT NULL,
			calls       INTEGER NOT NULL DEFAULT 0,
			input       INTEGER NOT NULL DEFAULT 0,
			cache_write INTEGER NOT NULL DEFAULT 0,
			cache_read  INTEGER NOT NULL DEFAULT 0,
			output      INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, user_id, kind)
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			db.Close()
			return nil, fmt.Errorf("amis / invitations : %w", err)
		}
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
		{"invited_by", "INTEGER NOT NULL DEFAULT 0"},
		{"referral_credited", "INTEGER NOT NULL DEFAULT 0"},
		{"bonus_quota", "INTEGER NOT NULL DEFAULT 0"},
		{"timezone", "TEXT NOT NULL DEFAULT ''"},
		{"reminders", "INTEGER NOT NULL DEFAULT 1"},
		{"last_reminder_day", "TEXT NOT NULL DEFAULT ''"},
		{"theme", "TEXT NOT NULL DEFAULT ''"},
		{"reading_mode", "TEXT NOT NULL DEFAULT ''"},
		{"eink", "INTEGER NOT NULL DEFAULT 0"},
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

// migratePointsByLang adds the per-language split of review points that the
// leaderboard ranks on. Points earned before it existed are credited to the
// language of the user's first book (everyone had books in a single
// language when this shipped).
func migratePointsByLang(db *sql.DB) error {
	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'points_lang'`).Scan(&exists); err != nil {
		return fmt.Errorf("table points_lang : %w", err)
	}
	if exists == 1 {
		return nil
	}
	if _, err := db.Exec(`CREATE TABLE points_lang (
		user_id INTEGER NOT NULL REFERENCES users(id),
		day     TEXT NOT NULL,
		lang    TEXT NOT NULL,
		points  INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (user_id, day, lang)
	)`); err != nil {
		return fmt.Errorf("table points_lang : %w", err)
	}
	rows, err := db.Query(`SELECT d.user_id, d.day, d.points,
			COALESCE((SELECT b.language FROM books b WHERE b.user_id = d.user_id ORDER BY b.id LIMIT 1), '')
		FROM daily_activity d WHERE d.points > 0`)
	if err != nil {
		return fmt.Errorf("migration points_lang : %w", err)
	}
	type row struct {
		userID    int64
		day, lang string
		points    int
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.userID, &r.day, &r.points, &r.lang); err != nil {
			rows.Close()
			return fmt.Errorf("migration points_lang : %w", err)
		}
		all = append(all, r)
	}
	rows.Close()
	for _, r := range all {
		if _, err := db.Exec(`INSERT INTO points_lang (user_id, day, lang, points) VALUES (?, ?, ?, ?)`,
			r.userID, r.day, LangKey(r.lang), r.points); err != nil {
			return fmt.Errorf("migration points_lang : %w", err)
		}
	}
	return nil
}

// mergeDuplicateLemmas folds cards saved twice for the same word in
// different forms (before FindVocab matched on the dictionary form) into
// one: the most advanced card is kept — highest box, then most recently
// reviewed, then oldest — and the others are deleted. A no-op once merged.
func mergeDuplicateLemmas(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM vocab WHERE lemma != '' AND id NOT IN (
		SELECT id FROM (
			SELECT id, ROW_NUMBER() OVER (
				PARTITION BY user_id, lemma COLLATE NOCASE
				ORDER BY box DESC, last_reviewed_at IS NULL, last_reviewed_at DESC, id ASC) AS rn
			FROM vocab WHERE lemma != ''
		) WHERE rn = 1)`)
	if err != nil {
		return fmt.Errorf("fusion des doublons : %w", err)
	}
	return nil
}

// LangKey normalizes a book's language tag ("en-US", "EN", "fr_FR",
// "eng") to the two-letter code words, reviews and the leaderboard are
// grouped by; "" when unknown.
func LangKey(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_"); i >= 0 {
		tag = tag[:i]
	}
	if two, ok := iso639_2[tag]; ok {
		return two
	}
	return tag
}

// Three-letter codes some epubs use, for the languages Lydi has readers in.
var iso639_2 = map[string]string{
	"eng": "en", "fra": "fr", "fre": "fr", "spa": "es", "deu": "de", "ger": "de",
	"ita": "it", "por": "pt", "nld": "nl", "dut": "nl", "rus": "ru", "jpn": "ja",
	"zho": "zh", "chi": "zh", "pol": "pl", "swe": "sv", "dan": "da", "nor": "no",
}

// migrateBookLangKeys adds books.lang_key (the normalized language) and
// fills it for books imported before it existed.
func migrateBookLangKeys(db *sql.DB) error {
	if err := ensureColumn(db, "books", "lang_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migration books.lang_key : %w", err)
	}
	rows, err := db.Query(`SELECT id, language FROM books WHERE lang_key = '' AND language != ''`)
	if err != nil {
		return fmt.Errorf("migration books.lang_key : %w", err)
	}
	keys := map[int64]string{}
	for rows.Next() {
		var id int64
		var lang string
		if err := rows.Scan(&id, &lang); err != nil {
			rows.Close()
			return fmt.Errorf("migration books.lang_key : %w", err)
		}
		keys[id] = LangKey(lang)
	}
	rows.Close()
	for id, key := range keys {
		if _, err := db.Exec(`UPDATE books SET lang_key = ? WHERE id = ?`, key, id); err != nil {
			return fmt.Errorf("migration books.lang_key : %w", err)
		}
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
	// BonusQuota is added to the free plan's monthly translations (earned
	// by inviting friends).
	BonusQuota int
	// Timezone is the browser's IANA zone, last seen; "" if never.
	Timezone string
	// Reminders: the evening email when the daily challenge isn't done.
	Reminders       bool
	LastReminderDay string
	// Theme: "light" or "dark" when chosen in the settings, "" to follow
	// the system.
	Theme string
	// ReadingMode: "scroll", "pages", or "" (auto: scroll on a phone,
	// pages on a tablet).
	ReadingMode string
	// Eink: e-reader display mode (pure black on white, no animation,
	// pages).
	Eink bool
}

// Plans: "free" is bounded by the monthly translation quota; "unlimited"
// isn't (pre-signup accounts, and later whoever pays).
const (
	PlanFree      = "free"
	PlanUnlimited = "unlimited"
)

const userColumns = `id, username, password_hash, native_lang, created_at, COALESCE(email, ''), email_verified_at != '', plan, pending_email, daily_new_limit, display_name, bonus_quota, timezone, reminders, last_reminder_day, theme, reading_mode, eink`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var createdAt string
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.NativeLang, &createdAt, &u.Email, &u.EmailVerified, &u.Plan, &u.PendingEmail, &u.DailyNewLimit, &u.DisplayName, &u.BonusQuota, &u.Timezone, &u.Reminders, &u.LastReminderDay, &u.Theme, &u.ReadingMode, &u.Eink); err != nil {
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

func (s *Store) SetReadingPrefs(userID int64, mode string, eink bool) error {
	_, err := s.db.Exec(`UPDATE users SET reading_mode = ?, eink = ? WHERE id = ?`, mode, eink, userID)
	return err
}

func (s *Store) SetTheme(userID int64, theme string) error {
	_, err := s.db.Exec(`UPDATE users SET theme = ? WHERE id = ?`, theme, userID)
	return err
}

func (s *Store) SetTimezone(userID int64, tz string) error {
	_, err := s.db.Exec(`UPDATE users SET timezone = ? WHERE id = ?`, tz, userID)
	return err
}

func (s *Store) SetReminders(userID int64, on bool) error {
	_, err := s.db.Exec(`UPDATE users SET reminders = ? WHERE id = ?`, on, userID)
	return err
}

func (s *Store) SetLastReminderDay(userID int64, day string) error {
	_, err := s.db.Exec(`UPDATE users SET last_reminder_day = ? WHERE id = ?`, day, userID)
	return err
}

// ReminderCandidates lists confirmed-email accounts with reminders on that
// reviewed at least once since `sinceDay` — the evening reminder is for
// people building a habit, not for waking up long-gone accounts.
func (s *Store) ReminderCandidates(sinceDay string) ([]User, error) {
	rows, err := s.db.Query(`SELECT `+userColumns+` FROM users
		WHERE reminders = 1 AND COALESCE(email, '') != '' AND email_verified_at != ''
		AND id IN (SELECT user_id FROM daily_activity WHERE day >= ? AND reviews > 0)`, sinceDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
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
// InsertBook stores an imported book. source is "" for an uploaded epub, or
// where it came from (a catalog book's Source).
func (s *Store) InsertBook(userID int64, title, author, language, source string, chapters []struct{ Title, Content string }) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO books (user_id, title, author, language, lang_key, source, added_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, title, author, language, LangKey(language), source, time.Now().UTC().Format(timeLayout))
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

// BookSources maps the sources of this user's catalog books to their ids.
func (s *Store) BookSources(userID int64) (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT source, id FROM books WHERE user_id = ? AND source != ''`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var src string
		var id int64
		if err := rows.Scan(&src, &id); err != nil {
			return nil, err
		}
		out[src] = id
	}
	return out, rows.Err()
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

// FindVocab returns the id of this user's existing card for this word —
// same phrase, or same dictionary form (case-insensitive) — so looking a
// word up twice, or in another form ("weirwood", then "weirwoods"),
// doesn't create two cards. Scoped per user so two people don't collide on
// a common word.
func (s *Store) FindVocab(userID int64, phrase, lemma string) (id int64, found bool, err error) {
	err = s.db.QueryRow(`SELECT id FROM vocab WHERE user_id = ?
		AND (phrase = ? COLLATE NOCASE OR (? != '' AND lemma = ? COLLATE NOCASE)) LIMIT 1`,
		userID, phrase, lemma, lemma).Scan(&id)
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
// lang ("" for all) keeps only words from books in that language.
func (s *Store) ListVocab(userID int64, lang string) ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.user_id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.frequency, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND v.archived = 0 AND (? = '' OR b.lang_key = ?)
		ORDER BY v.frequency ASC`, userID, lang, lang)
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
// earned — also credited to the language being learned (the card's book's),
// for the per-language leaderboard. wasNew when it was the card's first
// review, which is what the daily new-card limit counts.
func (s *Store) RecordReview(userID int64, day string, wasNew bool, points int, lang string) error {
	n := 0
	if wasNew {
		n = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO daily_activity (user_id, day, reviews, new_cards, points) VALUES (?, ?, 1, ?, ?)
		ON CONFLICT (user_id, day) DO UPDATE SET reviews = reviews + 1, new_cards = new_cards + excluded.new_cards,
			points = points + excluded.points`,
		userID, day, n, points); err != nil {
		return err
	}
	if points > 0 {
		if _, err := tx.Exec(`INSERT INTO points_lang (user_id, day, lang, points) VALUES (?, ?, ?, ?)
			ON CONFLICT (user_id, day, lang) DO UPDATE SET points = points + excluded.points`,
			userID, day, LangKey(lang), points); err != nil {
			return err
		}
	}
	return tx.Commit()
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

// DeckCounts is what the daily session is built from.
type DeckCounts struct {
	DueReviews int // due cards already reviewed at least once
	New        int // never-reviewed cards
	Fragile    int // reviewed but still in box 1-2: the words being learned
	Total      int
}

// DeckCounts counts the deck, or only one language's words (lang != "").
func (s *Store) DeckCounts(userID int64, now time.Time, lang string) (DeckCounts, error) {
	var c DeckCounts
	err := s.db.QueryRow(`SELECT
			COALESCE(SUM(v.last_reviewed_at IS NOT NULL AND v.next_review_at <= ?), 0),
			COALESCE(SUM(v.last_reviewed_at IS NULL), 0),
			COALESCE(SUM(v.last_reviewed_at IS NOT NULL AND v.box <= 2), 0),
			COUNT(*)
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND (? = '' OR b.lang_key = ?)`, now.Format(timeLayout), userID, lang, lang).
		Scan(&c.DueReviews, &c.New, &c.Fragile, &c.Total)
	return c, err
}

// VocabLangs lists the languages of the user's words, most words first.
func (s *Store) VocabLangs(userID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT b.lang_key FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? GROUP BY b.lang_key ORDER BY COUNT(*) DESC, b.lang_key`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// VocabProgress splits the deck for the "my words" page: waiting (never
// reviewed), learning (box 1-3) and known (box 4-5, or marked known).
func (s *Store) VocabProgress(userID int64, lang string) (waiting, learning, known int, err error) {
	err = s.db.QueryRow(`SELECT
			COALESCE(SUM(v.last_reviewed_at IS NULL), 0),
			COALESCE(SUM(v.last_reviewed_at IS NOT NULL AND v.box <= 3 AND v.archived = 0), 0),
			COALESCE(SUM(v.last_reviewed_at IS NOT NULL AND (v.box >= 4 OR v.archived = 1)), 0)
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND (? = '' OR b.lang_key = ?)`, userID, lang, lang).Scan(&waiting, &learning, &known)
	return
}

// NextDailyCards returns up to `limit` upcoming cards of the daily session,
// in order: due reviews first, then (if allowNew) never-reviewed cards, then
// cards missed earlier today (reviewed since dayStart and due again), most
// frequent words first within each group. The first is the one to show; the
// rest are what the review page prepares ahead of time.
func (s *Store) NextDailyCards(userID int64, now, dayStart time.Time, lang string, allowNew bool, limit int) ([]Vocab, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.user_id, v.book_id, b.title, v.chapter_id, v.phrase, v.lemma, v.context,
		       v.translation, v.note, v.frequency, v.box, v.next_review_at, v.created_at, v.last_reviewed_at
		FROM vocab v JOIN books b ON b.id = v.book_id
		WHERE v.user_id = ? AND (? = '' OR b.lang_key = ?) AND (
			(v.last_reviewed_at IS NOT NULL AND v.next_review_at <= ?) OR (? AND v.last_reviewed_at IS NULL))
		ORDER BY CASE WHEN v.last_reviewed_at IS NULL THEN 1 WHEN v.last_reviewed_at >= ? THEN 2 ELSE 0 END,
			v.frequency ASC, v.next_review_at ASC
		LIMIT ?`, userID, lang, lang, now.Format(timeLayout), allowNew, dayStart.UTC().Format(timeLayout), limit)
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

// onlyUsers restricts a points_lang query to some users (a friends league);
// nil means everyone.
func onlyUsers(ids []int64, args []any) (string, []any) {
	if ids == nil {
		return "", args
	}
	if len(ids) == 0 {
		return " AND 0", args
	}
	clause := " AND p.user_id IN (?" + strings.Repeat(", ?", len(ids)-1) + ")"
	for _, id := range ids {
		args = append(args, id)
	}
	return clause, args
}

// Leaderboard ranks everyone (or only the given users) who earned points in
// one language between two days (inclusive), best first.
func (s *Store) Leaderboard(fromDay, toDay, lang string, only []int64, limit int) ([]LeaderboardRow, error) {
	filter, args := onlyUsers(only, []any{fromDay, toDay, lang})
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.display_name, SUM(p.points) AS total
		FROM points_lang p JOIN users u ON u.id = p.user_id
		WHERE p.day >= ? AND p.day <= ? AND p.lang = ?`+filter+`
		GROUP BY u.id HAVING total > 0
		ORDER BY total DESC, u.id ASC
		LIMIT ?`, append(args, limit)...)
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

type LeaderboardLang struct {
	Lang    string
	Players int
}

// LeaderboardLangs lists the languages someone earned points in between two
// days, most players first.
func (s *Store) LeaderboardLangs(fromDay, toDay string, only []int64) ([]LeaderboardLang, error) {
	filter, args := onlyUsers(only, []any{fromDay, toDay})
	rows, err := s.db.Query(`
		SELECT p.lang, COUNT(DISTINCT p.user_id) AS players FROM points_lang p
		WHERE p.day >= ? AND p.day <= ? AND p.points > 0`+filter+`
		GROUP BY p.lang ORDER BY players DESC, p.lang ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaderboardLang
	for rows.Next() {
		var l LeaderboardLang
		if err := rows.Scan(&l.Lang, &l.Players); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// UserBookLang is the language of the user's most recently added book, the
// default leaderboard tab for them; "" without books.
func (s *Store) UserBookLang(userID int64) (string, error) {
	var lang string
	err := s.db.QueryRow(`SELECT language FROM books WHERE user_id = ? ORDER BY id DESC LIMIT 1`, userID).Scan(&lang)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return LangKey(lang), err
}

// --- friends & invitations ---
//
// Friendship is symmetric (stored both ways) and makes up a reader's
// private league on the leaderboard. Someone who signs up through a
// friend's invitation becomes their friend once their email is confirmed,
// and both get ReferralBonus more free translations a month.

const (
	ReferralBonus    = 100
	MaxReferralBonus = 500
)

type Friend struct {
	ID          int64
	Username    string
	DisplayName string
}

func (s *Store) AddFriendship(a, b int64) error {
	if a == b {
		return nil
	}
	now := time.Now().UTC().Format(timeLayout)
	_, err := s.db.Exec(`INSERT OR IGNORE INTO friends (user_id, friend_id, created_at) VALUES (?, ?, ?), (?, ?, ?)`,
		a, b, now, b, a, now)
	return err
}

func (s *Store) RemoveFriendship(a, b int64) error {
	_, err := s.db.Exec(`DELETE FROM friends WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)`, a, b, b, a)
	return err
}

func (s *Store) Friends(userID int64) ([]Friend, error) {
	rows, err := s.db.Query(`SELECT u.id, u.username, u.display_name FROM friends f JOIN users u ON u.id = f.friend_id
		WHERE f.user_id = ? ORDER BY f.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Friend
	for rows.Next() {
		var f Friend
		if err := rows.Scan(&f.ID, &f.Username, &f.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) AreFriends(a, b int64) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM friends WHERE user_id = ? AND friend_id = ?`, a, b).Scan(&n)
	return n > 0, err
}

// SetInvitedBy remembers who invited a just-created account, credited when
// its email gets confirmed (CompleteReferral).
func (s *Store) SetInvitedBy(userID, inviterID int64) error {
	_, err := s.db.Exec(`UPDATE users SET invited_by = ? WHERE id = ? AND invited_by = 0`, inviterID, userID)
	return err
}

// CompleteReferral turns a confirmed invited account into its inviter's
// friend and gives both the referral bonus, once. Returns the inviter's id
// (0 when there was nothing to credit).
func (s *Store) CompleteReferral(userID int64, email string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var inviter int64
	var credited bool
	if err := tx.QueryRow(`SELECT invited_by, referral_credited FROM users WHERE id = ?`, userID).Scan(&inviter, &credited); err != nil {
		return 0, err
	}
	if inviter == 0 || credited {
		return 0, nil
	}
	now := time.Now().UTC().Format(timeLayout)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE users SET referral_credited = 1, bonus_quota = MIN(bonus_quota + ?, ?) WHERE id = ?`, []any{ReferralBonus, MaxReferralBonus, userID}},
		{`UPDATE users SET bonus_quota = MIN(bonus_quota + ?, ?) WHERE id = ?`, []any{ReferralBonus, MaxReferralBonus, inviter}},
		{`INSERT OR IGNORE INTO friends (user_id, friend_id, created_at) VALUES (?, ?, ?), (?, ?, ?)`, []any{userID, inviter, now, inviter, userID, now}},
		{`UPDATE invites SET accepted_at = ? WHERE inviter_id = ? AND email = ? COLLATE NOCASE AND accepted_at = ''`, []any{now, inviter, email}},
	} {
		if _, err := tx.Exec(q.sql, q.args...); err != nil {
			return 0, err
		}
	}
	return inviter, tx.Commit()
}

type Invite struct {
	Email     string
	CreatedAt time.Time
	Accepted  bool
}

func (s *Store) CreateInvite(inviterID int64, email string) error {
	_, err := s.db.Exec(`INSERT INTO invites (inviter_id, email, created_at) VALUES (?, ?, ?)`,
		inviterID, email, time.Now().UTC().Format(timeLayout))
	return err
}

func (s *Store) CountInvitesSince(inviterID int64, since time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM invites WHERE inviter_id = ? AND created_at >= ?`,
		inviterID, since.UTC().Format(timeLayout)).Scan(&n)
	return n, err
}

// InvitedSince reports whether anyone invited this address since `since` —
// so one address can't be flooded by several inviters, or the same one.
func (s *Store) InvitedSince(email string, since time.Time) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM invites WHERE email = ? COLLATE NOCASE AND created_at >= ?`,
		email, since.UTC().Format(timeLayout)).Scan(&n)
	return n > 0, err
}

func (s *Store) ListInvites(inviterID int64, limit int) ([]Invite, error) {
	rows, err := s.db.Query(`SELECT email, created_at, accepted_at != '' FROM invites WHERE inviter_id = ? ORDER BY id DESC LIMIT ?`, inviterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invite
	for rows.Next() {
		var inv Invite
		var created string
		if err := rows.Scan(&inv.Email, &created, &inv.Accepted); err != nil {
			return nil, err
		}
		inv.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, inv)
	}
	return out, rows.Err()
}

// --- AI usage & admin dashboard ---

type TokenUsage struct {
	Calls, Input, CacheWrite, CacheRead, Output int64
}

func (s *Store) RecordAIUsage(day string, userID int64, kind string, u TokenUsage) error {
	_, err := s.db.Exec(`INSERT INTO ai_usage (day, user_id, kind, calls, input, cache_write, cache_read, output)
		VALUES (?, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT (day, user_id, kind) DO UPDATE SET calls = calls + 1, input = input + excluded.input,
			cache_write = cache_write + excluded.cache_write, cache_read = cache_read + excluded.cache_read,
			output = output + excluded.output`,
		day, userID, kind, u.Input, u.CacheWrite, u.CacheRead, u.Output)
	return err
}

// AIUsageByKind sums token usage since a day, per kind of call.
func (s *Store) AIUsageByKind(sinceDay string) (map[string]TokenUsage, error) {
	rows, err := s.db.Query(`SELECT kind, SUM(calls), SUM(input), SUM(cache_write), SUM(cache_read), SUM(output)
		FROM ai_usage WHERE day >= ? GROUP BY kind`, sinceDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]TokenUsage{}
	for rows.Next() {
		var k string
		var u TokenUsage
		if err := rows.Scan(&k, &u.Calls, &u.Input, &u.CacheWrite, &u.CacheRead, &u.Output); err != nil {
			return nil, err
		}
		out[k] = u
	}
	return out, rows.Err()
}

type AdminUser struct {
	ID                 int64
	Username, Email    string
	DisplayName, Plan  string
	CreatedAt          time.Time
	Verified           bool
	InvitedBy          int64
	Books, Words       int
	Reviews, Reviews7d int
	LastActive         string // last day with a review, "" if never
	TranslationsMonth  int
	AI                 TokenUsage // this month
}

// AdminUsers is one row per account with what the dashboard needs.
func (s *Store) AdminUsers(sevenDaysAgo, monthStart, month string) ([]AdminUser, error) {
	rows, err := s.db.Query(`SELECT u.id, u.username, COALESCE(u.email, ''), u.display_name, u.plan, u.created_at,
			u.email_verified_at != '', u.invited_by,
			(SELECT COUNT(*) FROM books b WHERE b.user_id = u.id),
			(SELECT COUNT(*) FROM vocab v WHERE v.user_id = u.id),
			(SELECT COALESCE(SUM(reviews), 0) FROM daily_activity d WHERE d.user_id = u.id),
			(SELECT COALESCE(SUM(reviews), 0) FROM daily_activity d WHERE d.user_id = u.id AND d.day >= ?),
			(SELECT COALESCE(MAX(day), '') FROM daily_activity d WHERE d.user_id = u.id AND d.reviews > 0),
			(SELECT COALESCE(SUM(translations), 0) FROM usage g WHERE g.user_id = u.id AND g.month = ?),
			(SELECT COALESCE(SUM(calls), 0) FROM ai_usage a WHERE a.user_id = u.id AND a.day >= ?),
			(SELECT COALESCE(SUM(input), 0) FROM ai_usage a WHERE a.user_id = u.id AND a.day >= ?),
			(SELECT COALESCE(SUM(cache_write), 0) FROM ai_usage a WHERE a.user_id = u.id AND a.day >= ?),
			(SELECT COALESCE(SUM(cache_read), 0) FROM ai_usage a WHERE a.user_id = u.id AND a.day >= ?),
			(SELECT COALESCE(SUM(output), 0) FROM ai_usage a WHERE a.user_id = u.id AND a.day >= ?)
		FROM users u ORDER BY u.id`,
		sevenDaysAgo, month, monthStart, monthStart, monthStart, monthStart, monthStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUser
	for rows.Next() {
		var a AdminUser
		var created string
		if err := rows.Scan(&a.ID, &a.Username, &a.Email, &a.DisplayName, &a.Plan, &created, &a.Verified, &a.InvitedBy,
			&a.Books, &a.Words, &a.Reviews, &a.Reviews7d, &a.LastActive, &a.TranslationsMonth,
			&a.AI.Calls, &a.AI.Input, &a.AI.CacheWrite, &a.AI.CacheRead, &a.AI.Output); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, a)
	}
	return out, rows.Err()
}

type DayStat struct {
	Day                      string
	Signups, Active, Reviews int
}

// DayStats gives signups, active readers (at least one review) and reviews
// per day since a day.
func (s *Store) DayStats(sinceDay string) (map[string]DayStat, error) {
	out := map[string]DayStat{}
	rows, err := s.db.Query(`SELECT day, COUNT(DISTINCT user_id), SUM(reviews) FROM daily_activity
		WHERE day >= ? AND reviews > 0 GROUP BY day`, sinceDay)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d DayStat
		if err := rows.Scan(&d.Day, &d.Active, &d.Reviews); err != nil {
			rows.Close()
			return nil, err
		}
		out[d.Day] = d
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT substr(created_at, 1, 10), COUNT(*) FROM users WHERE created_at >= ? GROUP BY 1`, sinceDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var n int
		if err := rows.Scan(&day, &n); err != nil {
			return nil, err
		}
		d := out[day]
		d.Day, d.Signups = day, n
		out[day] = d
	}
	return out, rows.Err()
}
