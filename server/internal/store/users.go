package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	IsAdmin      bool
	Disabled     bool
	CreatedAt    time.Time
	QRCount      int // valorizzati solo da ListUsers
	ScanCount    int
}

const userColumns = "id, username, password_hash, is_admin, disabled, created_at"

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var created int64
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.Disabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = fromUnix(created)
	return u, nil
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, isAdmin bool, now time.Time) (User, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO users (username, password_hash, is_admin, created_at) VALUES (?, ?, ?, ?)",
		username, passwordHash, isAdmin, unix(now))
	if isUniqueViolation(err) {
		return User{}, ErrConflict
	}
	if err != nil {
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return s.UserByID(ctx, id)
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE id = ?", id))
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE username = ?", username))
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.is_admin, u.disabled, u.created_at,
		       (SELECT COUNT(*) FROM qr_codes q WHERE q.user_id = u.id AND q.deleted_at IS NULL),
		       (SELECT COUNT(*) FROM scans s JOIN qr_codes q ON q.id = s.qr_id
		        WHERE q.user_id = u.id AND q.deleted_at IS NULL AND s.is_bot = 0)
		FROM users u ORDER BY u.username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		var created int64
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.Disabled, &created, &u.QRCount, &u.ScanCount); err != nil {
			return nil, err
		}
		u.CreatedAt = fromUnix(created)
		users = append(users, u)
	}
	return users, rows.Err()
}

// UserPatch contiene solo i campi da modificare (nil = invariato).
type UserPatch struct {
	PasswordHash *string
	IsAdmin      *bool
	Disabled     *bool
}

func (s *Store) UpdateUser(ctx context.Context, id int64, p UserPatch) (User, error) {
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM users WHERE id = ?", id).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if p.PasswordHash != nil {
			if _, err := tx.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", *p.PasswordHash, id); err != nil {
				return err
			}
		}
		if p.IsAdmin != nil {
			if _, err := tx.ExecContext(ctx, "UPDATE users SET is_admin = ? WHERE id = ?", *p.IsAdmin, id); err != nil {
				return err
			}
		}
		if p.Disabled != nil {
			if _, err := tx.ExecContext(ctx, "UPDATE users SET disabled = ? WHERE id = ?", *p.Disabled, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

// ---- Sessioni ----------------------------------------------------------------

func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, expires time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)",
		tokenHash, userID, unix(expires))
	return err
}

// SessionUser restituisce l'utente della sessione, se valida e se l'utente è attivo.
func (s *Store) SessionUser(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.is_admin, u.disabled, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ? AND u.disabled = 0`,
		tokenHash, unix(now)))
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}

// DeleteUserSessions chiude tutte le sessioni dell'utente tranne exceptHash (può essere vuoto).
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64, exceptHash string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE user_id = ? AND token_hash != ?", userID, exceptHash)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", unix(now))
	return err
}
