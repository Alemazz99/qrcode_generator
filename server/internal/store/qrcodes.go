package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	KindDynamic = "dynamic"
	KindStatic  = "static"
)

// AnyOwner, passato al posto dell'id utente nelle letture, include i QR di tutti
// gli utenti (vista dell'amministratore). Le modifiche richiedono sempre il proprietario.
const AnyOwner int64 = 0

type QR struct {
	ID        int64
	UserID    int64
	OwnerName string
	// Anche i QR statici hanno un codice (identificativo interno), ma per loro
	// il redirect non esiste: il contenuto è scritto direttamente nel QR
	Code      string
	Kind      string
	Name      string
	TargetURL string
	StyleJSON string
	HasLogo   bool
	LogoName  string
	Active    bool
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time

	// Valorizzati solo da ListQRs
	ScansTotal  int
	ScansRecent int
	LastScanAt  *time.Time
}

const qrColumns = `id, user_id, code, name, target_url, style_json, logo IS NOT NULL, logo_name,
	active, deleted_at, created_at, updated_at, kind,
	COALESCE((SELECT username FROM users WHERE users.id = qr_codes.user_id), '')`

// ownerFilter limita la query al proprietario, o a nessuno se è AnyOwner.
const ownerFilter = "(? = 0 OR user_id = ?)"

func scanQR(row interface{ Scan(...any) error }, extra ...any) (QR, error) {
	var q QR
	var deleted sql.NullInt64
	var created, updated int64
	dest := append([]any{&q.ID, &q.UserID, &q.Code, &q.Name, &q.TargetURL, &q.StyleJSON, &q.HasLogo,
		&q.LogoName, &q.Active, &deleted, &created, &updated, &q.Kind, &q.OwnerName}, extra...)
	err := row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return QR{}, ErrNotFound
	}
	if err != nil {
		return QR{}, err
	}
	q.DeletedAt = fromNullUnix(deleted)
	q.CreatedAt = fromUnix(created)
	q.UpdatedAt = fromUnix(updated)
	return q, nil
}

type NewQR struct {
	UserID    int64
	Kind      string
	Name      string
	TargetURL string
	StyleJSON string
	Logo      []byte
	LogoName  string
}

// CreateQR inserisce un nuovo QR con un codice generato da newCode, riprovando
// se il codice è già in uso.
func (s *Store) CreateQR(ctx context.Context, n NewQR, newCode func() (string, error), now time.Time) (QR, error) {
	var logo any
	if len(n.Logo) > 0 {
		logo = n.Logo
	}
	for range 10 {
		code, err := newCode()
		if err != nil {
			return QR{}, err
		}
		var id int64
		err = s.db.QueryRowContext(ctx, `
			INSERT INTO qr_codes (user_id, code, kind, name, target_url, style_json, logo, logo_name, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			n.UserID, code, n.Kind, n.Name, n.TargetURL, n.StyleJSON, logo, n.LogoName, unix(now), unix(now)).Scan(&id)
		if isUniqueViolation(err) {
			continue
		}
		if err != nil {
			return QR{}, err
		}
		return s.GetQR(ctx, n.UserID, id)
	}
	return QR{}, fmt.Errorf("impossibile generare un codice libero")
}

// ListQRs restituisce i QR dell'utente (o di tutti, con AnyOwner) con il
// conteggio delle scansioni (totali e da recentSince in poi), bot esclusi.
func (s *Store) ListQRs(ctx context.Context, userID int64, recentSince time.Time) ([]QR, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+qrColumns+`,
		       (SELECT COUNT(*) FROM scans s WHERE s.qr_id = qr_codes.id AND s.is_bot = 0),
		       (SELECT COUNT(*) FROM scans s WHERE s.qr_id = qr_codes.id AND s.is_bot = 0 AND s.scanned_at >= ?),
		       (SELECT MAX(scanned_at) FROM scans s WHERE s.qr_id = qr_codes.id AND s.is_bot = 0)
		FROM qr_codes
		WHERE `+ownerFilter+` AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC`,
		unix(recentSince), userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []QR{}
	for rows.Next() {
		var last sql.NullInt64
		var total, recent int
		q, err := scanQR(rows, &total, &recent, &last)
		if err != nil {
			return nil, err
		}
		q.ScansTotal, q.ScansRecent, q.LastScanAt = total, recent, fromNullUnix(last)
		list = append(list, q)
	}
	return list, rows.Err()
}

// GetQR restituisce un QR dell'utente (o di chiunque, con AnyOwner); quelli
// eliminati o di altri utenti risultano ErrNotFound.
func (s *Store) GetQR(ctx context.Context, userID, id int64) (QR, error) {
	return scanQR(s.db.QueryRowContext(ctx,
		"SELECT "+qrColumns+" FROM qr_codes WHERE id = ? AND "+ownerFilter+" AND deleted_at IS NULL",
		id, userID, userID))
}

// QRByCode cerca un QR dal codice, anche se eliminato: decide il chiamante cosa fare.
func (s *Store) QRByCode(ctx context.Context, code string) (QR, error) {
	return scanQR(s.db.QueryRowContext(ctx, "SELECT "+qrColumns+" FROM qr_codes WHERE code = ?", code))
}

// QRPatch contiene solo i campi da modificare (nil = invariato).
type QRPatch struct {
	Name      *string
	TargetURL *string
	StyleJSON *string
	// Logo non nil e vuoto = rimuove il logo
	Logo     *[]byte
	LogoName *string
	Active   *bool
}

// UpdateQR modifica un QR dell'utente; se cambia la destinazione la registra nello storico.
func (s *Store) UpdateQR(ctx context.Context, userID, id int64, p QRPatch, now time.Time) (QR, error) {
	err := s.tx(ctx, func(tx *tx) error {
		var oldTarget string
		err := tx.QueryRowContext(ctx,
			"SELECT target_url FROM qr_codes WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
			id, userID).Scan(&oldTarget)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		set := func(column string, value any) error {
			_, err := tx.ExecContext(ctx, "UPDATE qr_codes SET "+column+" = ? WHERE id = ?", value, id)
			return err
		}
		if p.TargetURL != nil && *p.TargetURL != oldTarget {
			if err := set("target_url", *p.TargetURL); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO target_history (qr_id, old_url, new_url, changed_at, changed_by)
				VALUES (?, ?, ?, ?, ?)`, id, oldTarget, *p.TargetURL, unix(now), userID); err != nil {
				return err
			}
		}
		if p.Name != nil {
			if err := set("name", *p.Name); err != nil {
				return err
			}
		}
		if p.StyleJSON != nil {
			if err := set("style_json", *p.StyleJSON); err != nil {
				return err
			}
		}
		if p.Logo != nil {
			var logo any
			name := ""
			if len(*p.Logo) > 0 {
				logo = *p.Logo
				if p.LogoName != nil {
					name = *p.LogoName
				}
			}
			if err := set("logo", logo); err != nil {
				return err
			}
			if err := set("logo_name", name); err != nil {
				return err
			}
		}
		if p.Active != nil {
			if err := set("active", *p.Active); err != nil {
				return err
			}
		}
		return set("updated_at", unix(now))
	})
	if err != nil {
		return QR{}, err
	}
	return s.GetQR(ctx, userID, id)
}

// DeleteQR elimina il QR in modo "soft": il codice resta occupato per sempre,
// così un QR già stampato non potrà mai puntare al contenuto di qualcun altro.
func (s *Store) DeleteQR(ctx context.Context, userID, id int64, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE qr_codes SET deleted_at = ?, active = 0, logo = NULL, updated_at = ?
		WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		unix(now), unix(now), id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Logo(ctx context.Context, userID, id int64) ([]byte, error) {
	var logo []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT logo FROM qr_codes WHERE id = ? AND "+ownerFilter+" AND deleted_at IS NULL AND logo IS NOT NULL",
		id, userID, userID).Scan(&logo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return logo, err
}

type HistoryEntry struct {
	OldURL    string
	NewURL    string
	ChangedAt time.Time
	ChangedBy string
}

func (s *Store) TargetHistory(ctx context.Context, qrID int64) ([]HistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.old_url, h.new_url, h.changed_at, COALESCE(u.username, '')
		FROM target_history h LEFT JOIN users u ON u.id = h.changed_by
		WHERE h.qr_id = ? ORDER BY h.changed_at DESC, h.id DESC`, qrID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []HistoryEntry{}
	for rows.Next() {
		var h HistoryEntry
		var at int64
		if err := rows.Scan(&h.OldURL, &h.NewURL, &at, &h.ChangedBy); err != nil {
			return nil, err
		}
		h.ChangedAt = fromUnix(at)
		list = append(list, h)
	}
	return list, rows.Err()
}
