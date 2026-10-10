CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    native_lang   TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS books (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id  INTEGER NOT NULL DEFAULT 0 REFERENCES users(id),
    title    TEXT NOT NULL,
    author   TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    added_at TEXT NOT NULL
);
-- idx_books_user is created in Go, after the user_id backfill migration —
-- an index on a column that doesn't exist yet on an upgraded database would
-- fail before ensureColumn gets a chance to add it.

CREATE TABLE IF NOT EXISTS chapters (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER NOT NULL REFERENCES books(id),
    idx     INTEGER NOT NULL,
    title   TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chapters_book ON chapters(book_id, idx);

CREATE TABLE IF NOT EXISTS reading_progress (
    book_id     INTEGER PRIMARY KEY REFERENCES books(id),
    chapter_idx INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS vocab (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id          INTEGER NOT NULL DEFAULT 0 REFERENCES users(id),
    book_id          INTEGER NOT NULL REFERENCES books(id),
    chapter_id       INTEGER NOT NULL REFERENCES chapters(id),
    phrase           TEXT NOT NULL,
    lemma            TEXT NOT NULL DEFAULT '',
    context          TEXT NOT NULL,
    translation      TEXT NOT NULL,
    note             TEXT NOT NULL DEFAULT '',
    frequency        INTEGER NOT NULL DEFAULT 3,
    box              INTEGER NOT NULL DEFAULT 1,
    next_review_at   TEXT NOT NULL,
    created_at       TEXT NOT NULL,
    last_reviewed_at TEXT,
    archived         INTEGER NOT NULL DEFAULT 0
);
-- idx_vocab_due: same reason as idx_books_user above — created in Go.

-- One finished round of the word game (solo for now).
CREATE TABLE IF NOT EXISTS game_scores (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id   INTEGER NOT NULL REFERENCES users(id),
    lang_key  TEXT NOT NULL,
    score     INTEGER NOT NULL,
    played_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_game_scores_user ON game_scores(user_id, lang_key, score);

-- A round of the word game. Solo rounds have opponent_id 0; in a duel the
-- challenger plays first, then the friend plays the same questions (JSON).
CREATE TABLE IF NOT EXISTS game_rounds (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    challenger_id    INTEGER NOT NULL REFERENCES users(id),
    opponent_id      INTEGER NOT NULL DEFAULT 0,
    lang_key         TEXT NOT NULL,
    questions        TEXT NOT NULL,
    challenger_score INTEGER,
    opponent_score   INTEGER,
    created_at       TEXT NOT NULL,
    finished_at      TEXT
);
CREATE INDEX IF NOT EXISTS idx_game_rounds_challenger ON game_rounds(challenger_id, created_at);
CREATE INDEX IF NOT EXISTS idx_game_rounds_opponent ON game_rounds(opponent_id, created_at);

-- Words of a reader's own missed in the game, until answered right again.
CREATE TABLE IF NOT EXISTS game_misses (
    user_id   INTEGER NOT NULL REFERENCES users(id),
    vocab_id  INTEGER NOT NULL REFERENCES vocab(id) ON DELETE CASCADE,
    missed_at TEXT NOT NULL,
    PRIMARY KEY (user_id, vocab_id)
);
