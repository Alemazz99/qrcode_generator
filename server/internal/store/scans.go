package store

import (
	"context"
	"database/sql"
	"time"
)

type Scan struct {
	QRID    int64
	At      time.Time
	Device  string
	OS      string
	Browser string
	Bot     bool
}

func (s *Store) InsertScan(ctx context.Context, sc Scan) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scans (qr_id, scanned_at, device, os, browser, is_bot) VALUES (?, ?, ?, ?, ?, ?)`,
		sc.QRID, unix(sc.At), sc.Device, sc.OS, sc.Browser, sc.Bot)
	return err
}

// ScanSummary sono i dati grezzi per le statistiche di un QR.
type ScanSummary struct {
	Total      int        // scansioni di sempre, bot esclusi
	LastScanAt *time.Time // ultima scansione, bot esclusi
	BotsSince  int        // anteprime e bot ignorati nel periodo
	Since      []Scan     // scansioni (non bot) dal periodo richiesto in poi
}

func (s *Store) ScanSummary(ctx context.Context, qrID int64, since time.Time) (ScanSummary, error) {
	var sum ScanSummary
	var last sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FILTER (WHERE is_bot = 0),
		       MAX(scanned_at) FILTER (WHERE is_bot = 0),
		       COUNT(*) FILTER (WHERE is_bot = 1 AND scanned_at >= ?)
		FROM scans WHERE qr_id = ?`, unix(since), qrID).Scan(&sum.Total, &last, &sum.BotsSince)
	if err != nil {
		return sum, err
	}
	sum.LastScanAt = fromNullUnix(last)

	rows, err := s.db.QueryContext(ctx, `
		SELECT scanned_at, device, os, browser FROM scans
		WHERE qr_id = ? AND is_bot = 0 AND scanned_at >= ? ORDER BY scanned_at`, qrID, unix(since))
	if err != nil {
		return sum, err
	}
	defer rows.Close()
	sum.Since = []Scan{}
	for rows.Next() {
		sc := Scan{QRID: qrID}
		var at int64
		if err := rows.Scan(&at, &sc.Device, &sc.OS, &sc.Browser); err != nil {
			return sum, err
		}
		sc.At = fromUnix(at)
		sum.Since = append(sum.Since, sc)
	}
	return sum, rows.Err()
}
