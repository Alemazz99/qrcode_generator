// Package config legge la configurazione del servizio dalle variabili d'ambiente
// (e dal file .env, se presente).
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Indirizzo su cui ascoltare, es. ":8080"
	Addr string
	// URL pubblico del servizio, senza "/" finale: finisce dentro ogni QR dinamico
	BaseURL string
	// Se impostato (postgres://…) si usa Postgres, altrimenti SQLite in DBPath
	DatabaseURL string
	DBPath      string
	// Usati solo al primo avvio, quando non esiste ancora nessun utente
	AdminUsername string
	AdminPassword string
	// Se il servizio sta dietro un reverse proxy, l'IP del client arriva da X-Forwarded-For
	TrustProxy bool
	// Cookie solo su HTTPS: attivo automaticamente se BASE_URL è https
	SecureCookies bool
	// Fuso orario con cui raggruppare le scansioni per giorno
	Location *time.Location
}

// Load legge .env (se c'è) e poi le variabili d'ambiente, che hanno la precedenza.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("lettura .env: %w", err)
	}

	cfg := Config{
		Addr:          ":" + env("PORT", "8080"),
		BaseURL:       strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		DBPath:        env("DB_PATH", "data/qr.db"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		TrustProxy:    env("TRUST_PROXY", "false") == "true",
	}

	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("BASE_URL non valido: %q (atteso es. https://q.esempio.it)", cfg.BaseURL)
	}
	if u.Path != "" || u.RawQuery != "" {
		return Config{}, fmt.Errorf("BASE_URL deve essere solo schema e dominio, senza percorso: %q", cfg.BaseURL)
	}
	cfg.SecureCookies = u.Scheme == "https"

	if cfg.DatabaseURL != "" && !strings.HasPrefix(cfg.DatabaseURL, "postgres://") &&
		!strings.HasPrefix(cfg.DatabaseURL, "postgresql://") {
		return Config{}, errors.New("DATABASE_URL deve iniziare con postgres:// (lascialo vuoto per usare SQLite)")
	}

	cfg.Location, err = time.LoadLocation(env("TIMEZONE", "Europe/Rome"))
	if err != nil {
		return Config{}, fmt.Errorf("TIMEZONE non valido: %w", err)
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
