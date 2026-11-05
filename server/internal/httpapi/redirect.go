package httpapi

import (
	"errors"
	"html/template"
	"net/http"

	"fearlix/qrcode_generator/server/internal/auth"
	"fearlix/qrcode_generator/server/internal/store"
	"fearlix/qrcode_generator/server/internal/useragent"
)

// handleRedirect è il cuore dei QR dinamici: registra la scansione e manda
// il browser alla destinazione attuale.
func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if !auth.ValidCode(code) {
		s.errorPage(w, http.StatusNotFound)
		return
	}
	qr, err := s.store.QRByCode(r.Context(), code)
	// I QR statici non passano dal servizio: il loro codice è solo un identificativo interno
	if errors.Is(err, store.ErrNotFound) || (err == nil && (qr.DeletedAt != nil || qr.Kind != store.KindDynamic)) {
		s.errorPage(w, http.StatusNotFound)
		return
	}
	if err != nil {
		s.log.Error("lettura QR", "code", code, "err", err)
		s.errorPage(w, http.StatusInternalServerError)
		return
	}
	if !qr.Active {
		s.errorPage(w, http.StatusGone)
		return
	}

	if countable(r) {
		info := useragent.Parse(r.UserAgent())
		err := s.store.InsertScan(r.Context(), store.Scan{
			QRID: qr.ID, At: s.now(), Device: info.Device, OS: info.OS, Browser: info.Browser, Bot: info.Bot,
		})
		if err != nil {
			// La scansione persa non deve bloccare chi ha inquadrato il QR
			s.log.Error("registrazione scansione", "code", code, "err", err)
		}
	}

	// 302 e no-store: un 301 verrebbe memorizzato dal browser e un cambio di
	// destinazione non arriverebbe più a chi ha già scansionato il QR
	h := w.Header()
	h.Set("Cache-Control", "no-store, max-age=0")
	h.Set("X-Robots-Tag", "noindex")
	http.Redirect(w, r, qr.TargetURL, http.StatusFound)
}

// countable esclude le richieste che non sono una persona che apre il link:
// HEAD (controlli dei link) e precaricamenti del browser.
func countable(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	for _, h := range []string{"Purpose", "Sec-Purpose", "X-Purpose", "X-Moz"} {
		if v := r.Header.Get(h); v == "prefetch" || v == "preview" || v == "prerender" {
			return false
		}
	}
	return true
}

var errorPageTmpl = template.Must(template.New("err").Parse(`<!doctype html>
<html lang="it"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>{{.Title}}</title>
<style>
body{margin:0;min-height:100vh;display:grid;place-items:center;font:16px/1.5 system-ui,sans-serif;
background:#f6f7f9;color:#16181d;padding:24px;box-sizing:border-box;text-align:center}
@media (prefers-color-scheme:dark){body{background:#0f1115;color:#e8eaee}}
h1{font-size:1.4rem;margin:0 0 8px}p{margin:0;opacity:.75}
</style></head>
<body><main><h1>{{.Title}}</h1><p>{{.Text}}</p></main></body></html>`))

func (s *Server) errorPage(w http.ResponseWriter, status int) {
	data := struct{ Title, Text string }{"QR code non trovato", "Questo QR code non esiste o è stato eliminato."}
	switch status {
	case http.StatusGone:
		data = struct{ Title, Text string }{"QR code non più attivo", "Il proprietario ha disattivato questo QR code."}
	case http.StatusInternalServerError:
		data = struct{ Title, Text string }{"Errore temporaneo", "Riprova tra qualche istante."}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	errorPageTmpl.Execute(w, data)
}
