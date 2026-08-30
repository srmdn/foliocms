package handler_test

import (
	"database/sql"
	"testing"

	"github.com/srmdn/foliocms/internal/config"
	"github.com/srmdn/foliocms/internal/db"
	"github.com/srmdn/foliocms/internal/handler"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    email       TEXT NOT NULL UNIQUE,
    passwd_hash TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS posts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    slug         TEXT NOT NULL UNIQUE,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    tags         TEXT NOT NULL DEFAULT '',
    draft        INTEGER NOT NULL DEFAULT 1,
    publish_date DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS subscribers (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    token         TEXT NOT NULL UNIQUE,
    subscribed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS media (
    key          TEXT PRIMARY KEY,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size         INTEGER NOT NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if _, err := conn.Exec(schema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return db.Wrap(conn)
}

func newTestHandler(t *testing.T, contentDir string) *handler.Handler {
	t.Helper()
	database := newTestDB(t)
	cfg := &config.Config{
		ContentDir: contentDir,
		JWTSecret:  "test-secret",
	}
	return handler.New(database, cfg)
}
