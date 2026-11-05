package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fearlix/qrcode_generator/server/internal/auth"
	"fearlix/qrcode_generator/server/internal/store"
)

const (
	maxNameLen    = 100
	maxTargetLen  = 2048
	maxStyleBytes = 4 << 10
	maxLogoBytes  = 2 << 20
	maxLogoSide   = 2048
	// Il logo arriva in base64 dentro il JSON: +33% e un po' di margine
	maxQRBody = maxLogoBytes*4/3 + 64<<10
)

type qrJSON struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Kind string `json:"kind"`
	// Vuoto per i QR statici, che non passano dal servizio
	ShortURL  string          `json:"short_url"`
	Name      string          `json:"name"`
	TargetURL string          `json:"target_url"`
	Style     json.RawMessage `json:"style"`
	HasLogo   bool            `json:"has_logo"`
	LogoName  string          `json:"logo_name"`
	Active    bool            `json:"active"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Owner     ownerJSON       `json:"owner"`
	// false quando l'admin guarda il QR di un altro utente: può vederlo, non modificarlo
	Editable bool `json:"editable"`
}

type ownerJSON struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type qrListItem struct {
	qrJSON
	ScansTotal int        `json:"scans_total"`
	Scans7d    int        `json:"scans_7d"`
	LastScanAt *time.Time `json:"last_scan_at"`
}

func (s *Server) toQRJSON(q store.QR, viewer store.User) qrJSON {
	short := ""
	if q.Kind == store.KindDynamic {
		short = s.cfg.BaseURL + "/" + q.Code
	}
	return qrJSON{
		ID: q.ID, Code: q.Code, Kind: q.Kind, ShortURL: short, Name: q.Name,
		TargetURL: q.TargetURL, Style: json.RawMessage(q.StyleJSON), HasLogo: q.HasLogo,
		LogoName: q.LogoName, Active: q.Active, CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt,
		Owner: ownerJSON{ID: q.UserID, Username: q.OwnerName}, Editable: q.UserID == viewer.ID,
	}
}

// readScope è il proprietario da usare nelle letture: l'admin vede i QR di tutti.
func readScope(sess session) int64 {
	if sess.user.IsAdmin {
		return store.AnyOwner
	}
	return sess.user.ID
}

// notOwnedError risponde per un QR che l'utente non può modificare: all'admin, che il QR lo
// vede, si spiega perché; per gli altri il QR semplicemente non esiste.
func (s *Server) notOwnedError(w http.ResponseWriter, r *http.Request, sess session, id int64) {
	if sess.user.IsAdmin {
		if _, err := s.store.GetQR(r.Context(), store.AnyOwner, id); err == nil {
			writeError(w, http.StatusForbidden, "solo il proprietario può modificare o eliminare questo QR")
			return
		}
	}
	writeError(w, http.StatusNotFound, "QR non trovato")
}

func (s *Server) handleListQRs(w http.ResponseWriter, r *http.Request, sess session) {
	s.writeQRList(w, r, sess, sess.user.ID)
}

// handleListAllQRs è la vista dell'admin: i QR di tutti, o di un solo utente con ?user_id=.
func (s *Server) handleListAllQRs(w http.ResponseWriter, r *http.Request, sess session) {
	owner := store.AnyOwner
	if v := r.URL.Query().Get("user_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "user_id non valido")
			return
		}
		owner = id
	}
	s.writeQRList(w, r, sess, owner)
}

func (s *Server) writeQRList(w http.ResponseWriter, r *http.Request, sess session, owner int64) {
	list, err := s.store.ListQRs(r.Context(), owner, s.now().Add(-7*24*time.Hour))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]qrListItem, len(list))
	for i, q := range list {
		out[i] = qrListItem{qrJSON: s.toQRJSON(q, sess.user), ScansTotal: q.ScansTotal, Scans7d: q.ScansRecent, LastScanAt: q.LastScanAt}
	}
	writeJSON(w, http.StatusOK, out)
}

// qrInput è il corpo di POST e PATCH: i campi assenti (nil) restano invariati.
type qrInput struct {
	// Solo in creazione: "dynamic" (predefinito) o "static"
	Kind      *string          `json:"kind"`
	Name      *string          `json:"name"`
	TargetURL *string          `json:"target_url"`
	Style     *json.RawMessage `json:"style"`
	// Data URL PNG (data:image/png;base64,...); stringa vuota = rimuovi il logo
	Logo     *string `json:"logo"`
	LogoName *string `json:"logo_name"`
	Active   *bool   `json:"active"`
}

// toPatch valida l'input per un QR del tipo kind; restituisce un messaggio d'errore per l'utente.
func (s *Server) toPatch(in qrInput, kind string) (store.QRPatch, string) {
	var p store.QRPatch
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if utf8.RuneCountInString(name) > maxNameLen {
			return p, fmt.Sprintf("il nome può avere al massimo %d caratteri", maxNameLen)
		}
		p.Name = &name
	}
	if in.TargetURL != nil {
		validate := s.validateTarget
		if kind == store.KindStatic {
			validate = validateStaticContent
		}
		target, msg := validate(*in.TargetURL)
		if msg != "" {
			return p, msg
		}
		p.TargetURL = &target
	}
	if in.Style != nil {
		style := bytes.TrimSpace(*in.Style)
		var obj map[string]any
		if len(style) > maxStyleBytes || json.Unmarshal(style, &obj) != nil || obj == nil {
			return p, "stile non valido"
		}
		str := string(style)
		p.StyleJSON = &str
	}
	if in.Logo != nil {
		logo, msg := decodeLogo(*in.Logo)
		if msg != "" {
			return p, msg
		}
		p.Logo = &logo
		name := ""
		if in.LogoName != nil {
			name = path.Base(strings.ReplaceAll(strings.TrimSpace(*in.LogoName), `\`, "/"))
			if name == "." || name == "/" {
				name = ""
			}
			if utf8.RuneCountInString(name) > 200 {
				name = string([]rune(name)[:200])
			}
		}
		p.LogoName = &name
	}
	p.Active = in.Active
	return p, ""
}

func (s *Server) validateTarget(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "inserisci il link di destinazione"
	}
	if len(raw) > maxTargetLen {
		return "", "link di destinazione troppo lungo"
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "il link di destinazione deve essere un indirizzo completo http:// o https://"
	}
	base, _ := url.Parse(s.cfg.BaseURL)
	if strings.EqualFold(u.Host, base.Host) {
		return "", "il link di destinazione non può puntare a questo stesso servizio"
	}
	return raw, ""
}

// Un QR statico contiene direttamente il testo: va bene anche qualcosa che non è un link
func validateStaticContent(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "inserisci il link o il testo da mettere nel QR"
	}
	if len(raw) > maxTargetLen {
		return "", "testo troppo lungo per un QR code"
	}
	return raw, ""
}

func decodeLogo(dataURL string) ([]byte, string) {
	if dataURL == "" {
		return []byte{}, ""
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		return nil, "il logo deve essere un'immagine PNG"
	}
	logo, err := base64.StdEncoding.DecodeString(dataURL[len(prefix):])
	if err != nil {
		return nil, "logo non leggibile"
	}
	if len(logo) > maxLogoBytes {
		return nil, "logo troppo grande (massimo 2 MB)"
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(logo))
	if err != nil || format != "png" {
		return nil, "logo non leggibile"
	}
	if cfg.Width > maxLogoSide || cfg.Height > maxLogoSide {
		return nil, fmt.Sprintf("logo troppo grande (massimo %d×%d px)", maxLogoSide, maxLogoSide)
	}
	return logo, ""
}

func (s *Server) handleCreateQR(w http.ResponseWriter, r *http.Request, sess session) {
	var in qrInput
	if !decodeJSON(w, r, &in, maxQRBody) {
		return
	}
	kind := store.KindDynamic
	if in.Kind != nil {
		kind = *in.Kind
	}
	if kind != store.KindDynamic && kind != store.KindStatic {
		writeError(w, http.StatusBadRequest, `tipo di QR non valido: usa "dynamic" o "static"`)
		return
	}
	if in.TargetURL == nil {
		writeError(w, http.StatusBadRequest, "inserisci il link di destinazione")
		return
	}
	p, msg := s.toPatch(in, kind)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	n := store.NewQR{UserID: sess.user.ID, Kind: kind, TargetURL: *p.TargetURL, StyleJSON: "{}"}
	if p.Name != nil {
		n.Name = *p.Name
	}
	if p.StyleJSON != nil {
		n.StyleJSON = *p.StyleJSON
	}
	if p.Logo != nil {
		n.Logo, n.LogoName = *p.Logo, *p.LogoName
	}
	qr, err := s.store.CreateQR(r.Context(), n, auth.NewCode, s.now())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.log.Info("QR creato", "code", qr.Code, "kind", qr.Kind, "user", sess.user.Username)
	writeJSON(w, http.StatusCreated, s.toQRJSON(qr, sess.user))
}

// qrID legge l'id dal percorso; se non è valido risponde 404.
func qrID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "QR non trovato")
		return 0, false
	}
	return id, true
}

// storeError traduce gli errori dello store in risposte HTTP.
func (s *Server) storeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "QR non trovato")
		return
	}
	s.internalError(w, r, err)
}

func (s *Server) handleGetQR(w http.ResponseWriter, r *http.Request, sess session) {
	id, ok := qrID(w, r)
	if !ok {
		return
	}
	qr, err := s.store.GetQR(r.Context(), readScope(sess), id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toQRJSON(qr, sess.user))
}

func (s *Server) handleUpdateQR(w http.ResponseWriter, r *http.Request, sess session) {
	id, ok := qrID(w, r)
	if !ok {
		return
	}
	var in qrInput
	if !decodeJSON(w, r, &in, maxQRBody) {
		return
	}
	current, err := s.store.GetQR(r.Context(), sess.user.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		s.notOwnedError(w, r, sess, id)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	// Passare da statico a dinamico (o viceversa) cambierebbe il contenuto e quindi il disegno
	if in.Kind != nil && *in.Kind != current.Kind {
		writeError(w, http.StatusBadRequest, "il tipo di un QR non si può cambiare dopo la creazione: creane uno nuovo")
		return
	}
	p, msg := s.toPatch(in, current.Kind)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	qr, err := s.store.UpdateQR(r.Context(), sess.user.ID, id, p, s.now())
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toQRJSON(qr, sess.user))
}

func (s *Server) handleDeleteQR(w http.ResponseWriter, r *http.Request, sess session) {
	id, ok := qrID(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteQR(r.Context(), sess.user.ID, id, s.now())
	if errors.Is(err, store.ErrNotFound) {
		s.notOwnedError(w, r, sess, id)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.log.Info("QR eliminato", "id", id, "user", sess.user.Username)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogo(w http.ResponseWriter, r *http.Request, sess session) {
	id, ok := qrID(w, r)
	if !ok {
		return
	}
	logo, err := s.store.Logo(r.Context(), readScope(sess), id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Write(logo)
}
