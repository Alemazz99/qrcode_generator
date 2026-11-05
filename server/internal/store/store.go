// Package store salva utenti, sessioni, QR code e scansioni su SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound = errors.New("non trovato")
	ErrConflict = errors.New("esiste già")
)

type Store struct {
	db *sql.DB
}

// Open apre (o crea) il database e applica le migrazioni mancanti.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("cartella del database: %w", err)
		}
	}
	dsn := "file:" + path +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Una sola connessione: SQLite serializza comunque le scritture e così non
	// ci sono mai errori "database is locked" tra connessioni concorrenti
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Ogni voce è una migrazione; la versione applicata è in PRAGMA user_version.
// Le migrazioni già rilasciate non vanno mai modificate: se ne aggiunge una nuova.
var migrations = []string{
	`
CREATE TABLE users (
	id            INTEGER PRIMARY KEY,
	username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT    NOT NULL,
	is_admin      INTEGER NOT NULL DEFAULT 0,
	disabled      INTEGER NOT NULL DEFAULT 0,
	created_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
	token_hash TEXT    PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE qr_codes (
	id         INTEGER PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id),
	code       TEXT    NOT NULL UNIQUE,
	name       TEXT    NOT NULL DEFAULT '',
	target_url TEXT    NOT NULL,
	style_json TEXT    NOT NULL DEFAULT '{}',
	logo       BLOB,
	logo_name  TEXT    NOT NULL DEFAULT '',
	active     INTEGER NOT NULL DEFAULT 1,
	deleted_at INTEGER,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX qr_codes_user ON qr_codes(user_id);

CREATE TABLE target_history (
	id         INTEGER PRIMARY KEY,
	qr_id      INTEGER NOT NULL REFERENCES qr_codes(id),
	old_url    TEXT    NOT NULL,
	new_url    TEXT    NOT NULL,
	changed_at INTEGER NOT NULL,
	changed_by INTEGER REFERENCES users(id)
);
CREATE INDEX target_history_qr ON target_history(qr_id, changed_at);

CREATE TABLE scans (
	id         INTEGER PRIMARY KEY,
	qr_id      INTEGER NOT NULL REFERENCES qr_codes(id),
	scanned_at INTEGER NOT NULL,
	device     TEXT    NOT NULL,
	os         TEXT    NOT NULL,
	browser    TEXT    NOT NULL,
	is_bot     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX scans_qr_time ON scans(qr_id, scanned_at);
`,
	// QR statici: il contenuto è scritto nel QR, il servizio non fa redirect né statistiche
	`ALTER TABLE qr_codes ADD COLUMN kind TEXT NOT NULL DEFAULT 'dynamic' CHECK (kind IN ('dynamic', 'static'));`,
}

func (s *Store) migrate(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("lettura versione schema: %w", err)
	}
	if version > len(migrations) {
		return fmt.Errorf("il database è alla versione %d, più recente di questo eseguibile (%d)", version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		err := s.tx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1))
			return err
		})
		if err != nil {
			return fmt.Errorf("migrazione %d: %w", i+1, err)
		}
	}
	return nil
}

func (s *Store) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func unix(t time.Time) int64 { return t.Unix() }

func fromUnix(v int64) time.Time { return time.Unix(v, 0).UTC() }

func fromNullUnix(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := fromUnix(v.Int64)
	return &t
}
