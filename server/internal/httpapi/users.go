package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fearlix/qrcode_generator/server/internal/auth"
	"fearlix/qrcode_generator/server/internal/store"
)

type userJSON struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
	QRCount   *int      `json:"qr_count,omitempty"`
	ScanCount *int      `json:"scan_count,omitempty"`
}

func toUserJSON(u store.User) userJSON {
	return userJSON{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, Disabled: u.Disabled, CreatedAt: u.CreatedAt}
}

// ---- Sessione ------------------------------------------------------------------

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	now := s.now()
	if !s.limiter.Allowed(ip, now) {
		writeError(w, http.StatusTooManyRequests, "troppi tentativi falliti, riprova tra qualche minuto")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req, 4<<10) {
		return
	}

	user, err := s.store.UserByUsername(r.Context(), strings.TrimSpace(req.Username))
	switch {
	case errors.Is(err, store.ErrNotFound):
		auth.CheckDummyPassword(req.Password)
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	if err != nil || user.Disabled || !auth.CheckPassword(user.PasswordHash, req.Password) {
		s.limiter.Fail(ip, now)
		s.log.Warn("login fallito", "username", req.Username, "ip", ip)
		writeError(w, http.StatusUnauthorized, "nome utente o password errati")
		return
	}
	s.limiter.Reset(ip)

	token, hash, err := auth.NewSessionToken()
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	expires := now.Add(auth.SessionDuration)
	if err := s.store.CreateSession(r.Context(), hash, user.ID, expires); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.setSessionCookie(w, token, expires)
	writeJSON(w, http.StatusOK, s.meJSON(user))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, sess session) {
	if err := s.store.DeleteSession(r.Context(), sess.tokenHash); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type meJSON struct {
	userJSON
	BaseURL string `json:"base_url"`
}

func (s *Server) meJSON(u store.User) meJSON {
	return meJSON{userJSON: toUserJSON(u), BaseURL: s.cfg.BaseURL}
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, sess session) {
	writeJSON(w, http.StatusOK, s.meJSON(sess.user))
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req, 4<<10) {
		return
	}
	if !auth.CheckPassword(sess.user.PasswordHash, req.Current) {
		writeError(w, http.StatusBadRequest, "la password attuale non è corretta")
		return
	}
	if err := auth.ValidatePassword(req.New); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.New)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if _, err := s.store.UpdateUser(r.Context(), sess.user.ID, store.UserPatch{PasswordHash: &hash}); err != nil {
		s.internalError(w, r, err)
		return
	}
	// Chiude le altre sessioni aperte (es. su un dispositivo perso), non questa
	if err := s.store.DeleteUserSessions(r.Context(), sess.user.ID, sess.tokenHash); err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Gestione utenti (admin) -----------------------------------------------------

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request, sess session) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]userJSON, len(users))
	for i, u := range users {
		out[i] = toUserJSON(u)
		out[i].QRCount = &users[i].QRCount
		out[i].ScanCount = &users[i].ScanCount
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if !decodeJSON(w, r, &req, 4<<10) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if err := auth.ValidateUsername(req.Username); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), req.Username, hash, req.IsAdmin, s.now())
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "nome utente già in uso")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.log.Info("utente creato", "username", user.Username, "admin", user.IsAdmin, "by", sess.user.Username)
	writeJSON(w, http.StatusCreated, toUserJSON(user))
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request, sess session) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "utente non trovato")
		return
	}
	var req struct {
		Password *string `json:"password"`
		IsAdmin  *bool   `json:"is_admin"`
		Disabled *bool   `json:"disabled"`
	}
	if !decodeJSON(w, r, &req, 4<<10) {
		return
	}
	// Evita di chiudersi fuori da soli
	if id == sess.user.ID && ((req.IsAdmin != nil && !*req.IsAdmin) || (req.Disabled != nil && *req.Disabled)) {
		writeError(w, http.StatusBadRequest, "non puoi disattivare o togliere i permessi di amministratore a te stesso")
		return
	}

	var patch store.UserPatch
	if req.Password != nil {
		if err := auth.ValidatePassword(*req.Password); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		patch.PasswordHash = &hash
	}
	patch.IsAdmin, patch.Disabled = req.IsAdmin, req.Disabled

	user, err := s.store.UpdateUser(r.Context(), id, patch)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "utente non trovato")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	// Password reimpostata o utente disattivato: le sue sessioni aperte non valgono più
	if req.Password != nil || user.Disabled {
		except := ""
		if id == sess.user.ID {
			except = sess.tokenHash
		}
		if err := s.store.DeleteUserSessions(r.Context(), id, except); err != nil {
			s.internalError(w, r, err)
			return
		}
	}
	s.log.Info("utente modificato", "username", user.Username, "by", sess.user.Username)
	writeJSON(w, http.StatusOK, toUserJSON(user))
}
