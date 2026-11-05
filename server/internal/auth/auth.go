// Package auth contiene password, token di sessione, codici dei QR e limitazione dei tentativi di login.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLen = 8
	// bcrypt ignora tutto oltre i 72 byte
	MaxPasswordBytes = 72
	SessionDuration  = 30 * 24 * time.Hour
	CodeLength       = 6
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

func ValidateUsername(u string) error {
	if !usernameRe.MatchString(u) {
		return errors.New("il nome utente deve avere 3-32 caratteri tra lettere, numeri, punto, trattino e underscore")
	}
	return nil
}

func ValidatePassword(p string) error {
	if utf8.RuneCountInString(p) < MinPasswordLen {
		return fmt.Errorf("la password deve avere almeno %d caratteri", MinPasswordLen)
	}
	if len(p) > MaxPasswordBytes {
		return fmt.Errorf("la password è troppo lunga (massimo %d byte)", MaxPasswordBytes)
	}
	return nil
}

func HashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

// dummyHash serve a impiegare lo stesso tempo anche quando l'utente non esiste,
// così dai tempi di risposta non si capisce quali nomi utente sono registrati.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("password-che-non-esiste"), bcrypt.DefaultCost)

func CheckDummyPassword(p string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(p))
}

// NewSessionToken restituisce il token da mettere nel cookie e l'hash da salvare nel DB.
func NewSessionToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

const codeAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var codeRe = regexp.MustCompile(`^[0-9A-Za-z]{` + fmt.Sprint(CodeLength) + `}$`)

// NewCode genera un codice casuale base62 (62^6 ≈ 57 miliardi di combinazioni).
func NewCode() (string, error) {
	b := make([]byte, CodeLength)
	for i := range b {
		// Rifiuta i byte >= 248 (= 4*62) per non favorire i primi caratteri
		for {
			var r [1]byte
			if _, err := rand.Read(r[:]); err != nil {
				return "", err
			}
			if r[0] < 248 {
				b[i] = codeAlphabet[r[0]%62]
				break
			}
		}
	}
	return string(b), nil
}

func ValidCode(code string) bool { return codeRe.MatchString(code) }

// Limiter conta i tentativi falliti per chiave (es. IP) in una finestra di tempo.
type Limiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	entries map[string]*limiterEntry
}

type limiterEntry struct {
	count int
	start time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, entries: map[string]*limiterEntry{}}
}

// Allowed dice se la chiave può ancora tentare.
func (l *Limiter) Allowed(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	return !ok || now.Sub(e.start) >= l.window || e.count < l.max
}

// Fail registra un tentativo fallito.
func (l *Limiter) Fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) > 10000 {
		for k, e := range l.entries {
			if now.Sub(e.start) >= l.window {
				delete(l.entries, k)
			}
		}
	}
	e, ok := l.entries[key]
	if !ok || now.Sub(e.start) >= l.window {
		l.entries[key] = &limiterEntry{count: 1, start: now}
		return
	}
	e.count++
}

// Reset azzera i tentativi della chiave (dopo un login riuscito).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}
