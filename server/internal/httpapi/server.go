// Package httpapi espone il redirect dei QR, le API della dashboard e la dashboard stessa.
package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fearlix/qrcode_generator/server/internal/auth"
	"fearlix/qrcode_generator/server/internal/config"
	"fearlix/qrcode_generator/server/internal/store"
)

const sessionCookie = "qr_session"

func init() {
	// Go non conosce questa estensione: senza, il manifest della web app arriva come text/plain
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

type Server struct {
	store   *store.Store
	cfg     config.Config
	app     fs.FS
	public  fs.FS
	limiter *auth.Limiter
	log     *slog.Logger
	// Sostituibile nei test
	now func() time.Time
}

// New crea il server: app è la dashboard (servita su /app/), public il
// generatore di QR statici senza accesso (servito su /crea/).
func New(st *store.Store, cfg config.Config, app, public fs.FS, log *slog.Logger) *Server {
	return &Server{
		store:   st,
		cfg:     cfg,
		app:     app,
		public:  public,
		limiter: auth.NewLimiter(10, 15*time.Minute),
		log:     log,
		now:     time.Now,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Pubblici
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})
	mux.HandleFunc("GET /{$}", redirectTo("/app/"))
	mux.HandleFunc("GET /app", redirectTo("/app/"))
	mux.Handle("GET /app/", staticFiles("/app/", s.app))
	// In una cartella sua: il service worker della web app mette in cache tutto
	// quello che sta nel suo percorso e non deve mai toccare /app/, /api/ o i link corti
	mux.HandleFunc("GET /crea", redirectTo("/crea/"))
	mux.Handle("GET /crea/", staticFiles("/crea/", s.public))
	mux.HandleFunc("GET /{code}", s.handleRedirect)

	// Sessione
	mux.HandleFunc("POST /api/login", s.mutating(s.handleLogin))
	mux.HandleFunc("POST /api/logout", s.mutating(s.withUser(s.handleLogout)))
	mux.HandleFunc("GET /api/me", s.withUser(s.handleMe))
	mux.HandleFunc("PUT /api/me/password", s.mutating(s.withUser(s.handleChangePassword)))

	// QR dell'utente
	mux.HandleFunc("GET /api/qrcodes", s.withUser(s.handleListQRs))
	mux.HandleFunc("POST /api/qrcodes", s.mutating(s.withUser(s.handleCreateQR)))
	mux.HandleFunc("GET /api/qrcodes/{id}", s.withUser(s.handleGetQR))
	mux.HandleFunc("PATCH /api/qrcodes/{id}", s.mutating(s.withUser(s.handleUpdateQR)))
	mux.HandleFunc("DELETE /api/qrcodes/{id}", s.mutating(s.withUser(s.handleDeleteQR)))
	mux.HandleFunc("GET /api/qrcodes/{id}/logo", s.withUser(s.handleLogo))
	mux.HandleFunc("GET /api/qrcodes/{id}/stats", s.withUser(s.handleStats))

	// Vista dell'admin: QR e statistiche di tutti gli utenti (in sola lettura)
	mux.HandleFunc("GET /api/admin/qrcodes", s.withAdmin(s.handleListAllQRs))

	// Gestione utenti (solo admin)
	mux.HandleFunc("GET /api/users", s.withAdmin(s.handleListUsers))
	mux.HandleFunc("POST /api/users", s.mutating(s.withAdmin(s.handleCreateUser)))
	mux.HandleFunc("PATCH /api/users/{id}", s.mutating(s.withAdmin(s.handleUpdateUser)))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "endpoint inesistente")
	})

	return securityHeaders(mux)
}

func redirectTo(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path, http.StatusFound)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// staticFiles serve le pagine incluse nell'eseguibile sotto prefix.
func staticFiles(prefix string, files fs.FS) http.Handler {
	fileServer := http.StripPrefix(prefix, http.FileServerFS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-cache")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: blob:; style-src 'self'; script-src 'self'; "+
				"connect-src 'self'; worker-src 'self'; manifest-src 'self'; object-src 'none'; "+
				"base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		fileServer.ServeHTTP(w, r)
	})
}

// ---- Sessione e protezioni ----------------------------------------------------

type session struct {
	user      store.User
	tokenHash string
}

type authedHandler func(http.ResponseWriter, *http.Request, session)

func (s *Server) withUser(h authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "accesso richiesto")
			return
		}
		hash := auth.HashToken(c.Value)
		user, err := s.store.SessionUser(r.Context(), hash, s.now())
		if errors.Is(err, store.ErrNotFound) {
			s.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "sessione scaduta, accedi di nuovo")
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		h(w, r, session{user: user, tokenHash: hash})
	}
}

func (s *Server) withAdmin(h authedHandler) http.HandlerFunc {
	return s.withUser(func(w http.ResponseWriter, r *http.Request, sess session) {
		if !sess.user.IsAdmin {
			writeError(w, http.StatusForbidden, "riservato agli amministratori")
			return
		}
		h(w, r, sess)
	})
}

// mutating protegge le richieste che modificano dati dal CSRF: un form di un
// altro sito non può inviare JSON né un'intestazione Origin diversa dalla nostra.
func (s *Server) mutating(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				writeError(w, http.StatusForbidden, "origine della richiesta non consentita")
				return
			}
		}
		mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mt != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "Content-Type deve essere application/json")
			return
		}
		next(w, r)
	}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip, _, _ := strings.Cut(fwd, ",")
			return strings.TrimSpace(ip)
		}
		if ip := r.Header.Get("X-Real-IP"); ip != "" {
			return strings.TrimSpace(ip)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---- JSON ----------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("errore interno", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "errore interno del server")
}

// decodeJSON legge il corpo come JSON; in caso di errore ha già risposto al client.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any, maxBytes int64) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "richiesta troppo grande")
		} else {
			writeError(w, http.StatusBadRequest, "JSON non valido: "+err.Error())
		}
		return false
	}
	return true
}
