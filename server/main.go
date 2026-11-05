// Servizio dei QR code dinamici: redirect con conteggio delle scansioni,
// API e dashboard per gestire i QR.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // fusi orari anche nelle immagini Docker minimali

	"fearlix/qrcode_generator/server/internal/auth"
	"fearlix/qrcode_generator/server/internal/config"
	"fearlix/qrcode_generator/server/internal/httpapi"
	"fearlix/qrcode_generator/server/internal/store"
	"fearlix/qrcode_generator/server/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("avvio fallito", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, dbName, err := openStore(cfg)
	if err != nil {
		return fmt.Errorf("database %s: %w", dbName, err)
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// La porta si apre subito: se è occupata si esce prima di toccare il database
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("impossibile ascoltare su %s (porta già usata da un altro servizio? cambia PORT e BASE_URL nel .env): %w", cfg.Addr, err)
	}

	if err := bootstrapAdmin(ctx, st, cfg, log); err != nil {
		return err
	}
	go cleanSessions(ctx, st, log)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(st, cfg, web.App(), web.Public(), log).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.Info("servizio avviato", "addr", cfg.Addr, "base_url", cfg.BaseURL, "dashboard", cfg.BaseURL+"/app/", "qr_statici", cfg.BaseURL+"/crea/", "db", dbName)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("arresto in corso")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// openStore usa Postgres se c'è DATABASE_URL, altrimenti il file SQLite.
// dbName è una descrizione del database da mostrare nei log, senza credenziali.
func openStore(cfg config.Config) (*store.Store, string, error) {
	if cfg.DatabaseURL == "" {
		st, err := store.Open(cfg.DBPath)
		return st, cfg.DBPath, err
	}
	name := "Postgres"
	if u, err := url.Parse(cfg.DatabaseURL); err == nil {
		name = "Postgres " + u.Host + u.Path
	}
	st, err := store.OpenPostgres(cfg.DatabaseURL)
	return st, name, err
}

// bootstrapAdmin crea il primo amministratore quando il database è vuoto.
func bootstrapAdmin(ctx context.Context, st *store.Store, cfg config.Config, log *slog.Logger) error {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
		return errors.New("nessun utente nel database: imposta ADMIN_USERNAME e ADMIN_PASSWORD (nel .env) per creare il primo amministratore")
	}
	if err := auth.ValidateUsername(cfg.AdminUsername); err != nil {
		return fmt.Errorf("ADMIN_USERNAME: %w", err)
	}
	if err := auth.ValidatePassword(cfg.AdminPassword); err != nil {
		return fmt.Errorf("ADMIN_PASSWORD: %w", err)
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	if _, err := st.CreateUser(ctx, cfg.AdminUsername, hash, true, time.Now()); err != nil {
		return err
	}
	log.Info("creato il primo amministratore", "username", cfg.AdminUsername)
	return nil
}

func cleanSessions(ctx context.Context, st *store.Store, log *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if err := st.DeleteExpiredSessions(ctx, time.Now()); err != nil && ctx.Err() == nil {
			log.Error("pulizia sessioni", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
