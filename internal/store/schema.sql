CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS books (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    title    TEXT NOT NULL,
    author   TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    added_at TEXT NOT NULL
);

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
    book_id          INTEGER NOT NULL REFERENCES books(id),
    chapter_id       INTEGER NOT NULL REFERENCES chapters(id),
    phrase           TEXT NOT NULL,
    lemma            TEXT NOT NULL DEFAULT '',
    context          TEXT NOT NULL,
    translation      TEXT NOT NULL,
    note             TEXT NOT NULL DEFAULT '',
    box              INTEGER NOT NULL DEFAULT 1,
    next_review_at   TEXT NOT NULL,
    created_at       TEXT NOT NULL,
    last_reviewed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_vocab_due ON vocab(next_review_at);
