package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"fearlix/qrcode_generator/server/internal/auth"
	"fearlix/qrcode_generator/server/internal/config"
	"fearlix/qrcode_generator/server/internal/store"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	iPhoneUA   = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1"
	androidUA  = "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36"
	whatsappUA = "WhatsApp/2.23.20.0 A"
)

type env struct {
	t   *testing.T
	ts  *httptest.Server
	st  *store.Store
	srv *Server
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st := openTestStore(t)

	loc, _ := time.LoadLocation("Europe/Rome")
	cfg := config.Config{BaseURL: "http://qr.test", Location: loc}
	app := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>dashboard")}}
	public := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>generatore")}}
	srv := New(st, cfg, app, public, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	e := &env{t: t, ts: ts, st: st, srv: srv}
	e.createUser("admin", true)
	return e
}

// openTestStore usa SQLite in una cartella temporanea oppure, se c'è
// TEST_DATABASE_URL, un Postgres di prova che viene svuotato prima di ogni test.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		return st
	}
	st, err := store.OpenPostgres(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	raw, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec("TRUNCATE scans, target_history, qr_codes, sessions, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatal(err)
	}
	return st
}

func (e *env) createUser(name string, admin bool) {
	e.t.Helper()
	hash, err := auth.HashPassword(name + "-password")
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.st.CreateUser(context.Background(), name, hash, admin, time.Now()); err != nil {
		e.t.Fatal(err)
	}
}

// client restituisce un client che non segue i redirect, con sessione già aperta se user != "".
func (e *env) client(user string) *http.Client {
	e.t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	if user != "" {
		res := e.do(c, "POST", "/api/login", map[string]string{"username": user, "password": user + "-password"})
		expectStatus(e.t, res, http.StatusOK)
	}
	return c
}

func (e *env) do(c *http.Client, method, path string, body any) *http.Response {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, r)
	if method != "GET" && method != "HEAD" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { res.Body.Close() })
	return res
}

// scan simula un telefono che apre il link del QR.
func (e *env) scan(method, code, ua string) *http.Response {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.ts.URL+"/"+code, nil)
	req.Header.Set("User-Agent", ua)
	res, err := e.client("").Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	res.Body.Close()
	return res
}

func (e *env) createQR(c *http.Client, target string) qrJSON {
	e.t.Helper()
	res := e.do(c, "POST", "/api/qrcodes", map[string]any{
		"name": "Test", "target_url": target, "style": map[string]any{"fgColor": "#112233"},
	})
	expectStatus(e.t, res, http.StatusCreated)
	var qr qrJSON
	decode(e.t, res, &qr)
	return qr
}

func expectStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("%s %s: status %d, atteso %d (%s)", res.Request.Method, res.Request.URL.Path, res.StatusCode, want, body)
	}
}

func decode(t *testing.T, res *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func TestDynamicQRFlow(t *testing.T) {
	e := newEnv(t)
	c := e.client("admin")

	qr := e.createQR(c, "https://test.com")
	if !auth.ValidCode(qr.Code) || qr.ShortURL != "http://qr.test/"+qr.Code {
		t.Fatalf("codice o link corto non validi: %+v", qr)
	}

	res := e.scan("GET", qr.Code, iPhoneUA)
	expectStatus(t, res, http.StatusFound)
	if loc := res.Header.Get("Location"); loc != "https://test.com" {
		t.Fatalf("Location = %q", loc)
	}
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control = %q: il redirect non deve essere memorizzato", cc)
	}

	// Cambio destinazione: stesso codice, stesso link corto, nuova Location
	res = e.do(c, "PATCH", "/api/qrcodes/"+itoa(qr.ID), map[string]any{"target_url": "https://prova.com"})
	expectStatus(t, res, http.StatusOK)
	var updated qrJSON
	decode(t, res, &updated)
	if updated.Code != qr.Code || updated.ShortURL != qr.ShortURL {
		t.Fatalf("il codice è cambiato: %q → %q", qr.Code, updated.Code)
	}
	if string(updated.Style) != `{"fgColor":"#112233"}` {
		t.Fatalf("lo stile è cambiato: %s", updated.Style)
	}
	res = e.scan("GET", qr.Code, androidUA)
	if loc := res.Header.Get("Location"); loc != "https://prova.com" {
		t.Fatalf("dopo il cambio Location = %q", loc)
	}

	// HEAD, bot e prefetch vengono redirezionati ma non contano
	expectStatus(t, e.scan("HEAD", qr.Code, iPhoneUA), http.StatusFound)
	expectStatus(t, e.scan("GET", qr.Code, whatsappUA), http.StatusFound)

	res = e.do(c, "GET", "/api/qrcodes/"+itoa(qr.ID)+"/stats?days=7", nil)
	expectStatus(t, res, http.StatusOK)
	var st statsJSON
	decode(t, res, &st)
	if st.Total != 2 || st.Period != 2 || st.BotsIgnored != 1 {
		t.Fatalf("statistiche: total=%d period=%d bots=%d, attesi 2/2/1", st.Total, st.Period, st.BotsIgnored)
	}
	if len(st.Daily) != 7 || st.Daily[6].Count != 2 {
		t.Fatalf("serie giornaliera: %+v", st.Daily)
	}
	if len(st.History) != 1 || st.History[0].OldURL != "https://test.com" || st.History[0].NewURL != "https://prova.com" || st.History[0].ChangedBy != "admin" {
		t.Fatalf("storico: %+v", st.History)
	}
	if len(st.OS) != 2 {
		t.Fatalf("sistemi operativi: %+v", st.OS)
	}

	res = e.do(c, "GET", "/api/qrcodes", nil)
	var list []qrListItem
	decode(t, res, &list)
	if len(list) != 1 || list[0].ScansTotal != 2 || list[0].Scans7d != 2 || list[0].LastScanAt == nil {
		t.Fatalf("lista: %+v", list)
	}
}

func TestStaticQR(t *testing.T) {
	e := newEnv(t)
	c := e.client("admin")

	// Un QR statico accetta anche testo libero e non ha link corto
	res := e.do(c, "POST", "/api/qrcodes", map[string]any{"kind": "static", "target_url": "WIFI:T:WPA;S:Casa;P:segreta;;"})
	expectStatus(t, res, http.StatusCreated)
	var qr qrJSON
	decode(t, res, &qr)
	if qr.Kind != "static" || qr.ShortURL != "" || qr.TargetURL != "WIFI:T:WPA;S:Casa;P:segreta;;" {
		t.Fatalf("QR statico: %+v", qr)
	}

	// Il suo codice non fa redirect: il servizio non entra mai in gioco
	expectStatus(t, e.scan("GET", qr.Code, iPhoneUA), http.StatusNotFound)

	// Il contenuto si può modificare (cambierà il disegno), il tipo no
	expectStatus(t, e.do(c, "PATCH", "/api/qrcodes/"+itoa(qr.ID), map[string]any{"target_url": "https://altro.it"}), http.StatusOK)
	expectStatus(t, e.do(c, "PATCH", "/api/qrcodes/"+itoa(qr.ID), map[string]any{"kind": "dynamic"}), http.StatusBadRequest)
	expectStatus(t, e.do(c, "PATCH", "/api/qrcodes/"+itoa(qr.ID), map[string]any{"kind": "static", "name": "ok"}), http.StatusOK)

	// I dinamici restano il default e vogliono un link vero
	dyn := e.createQR(c, "https://test.com")
	if dyn.Kind != "dynamic" || dyn.ShortURL == "" {
		t.Fatalf("QR dinamico: %+v", dyn)
	}
	expectStatus(t, e.do(c, "POST", "/api/qrcodes", map[string]any{"target_url": "solo testo"}), http.StatusBadRequest)
	expectStatus(t, e.do(c, "POST", "/api/qrcodes", map[string]any{"kind": "boh", "target_url": "https://x.it"}), http.StatusBadRequest)
}

func TestInactiveAndDeleted(t *testing.T) {
	e := newEnv(t)
	c := e.client("admin")
	qr := e.createQR(c, "https://test.com")

	expectStatus(t, e.do(c, "PATCH", "/api/qrcodes/"+itoa(qr.ID), map[string]any{"active": false}), http.StatusOK)
	expectStatus(t, e.scan("GET", qr.Code, iPhoneUA), http.StatusGone)

	expectStatus(t, e.do(c, "DELETE", "/api/qrcodes/"+itoa(qr.ID), nil), http.StatusNoContent)
	expectStatus(t, e.scan("GET", qr.Code, iPhoneUA), http.StatusNotFound)
	expectStatus(t, e.do(c, "GET", "/api/qrcodes/"+itoa(qr.ID), nil), http.StatusNotFound)

	// Codici inesistenti o malformati
	expectStatus(t, e.scan("GET", "ZZZZZZ", iPhoneUA), http.StatusNotFound)
	expectStatus(t, e.scan("GET", "favicon.ico", iPhoneUA), http.StatusNotFound)
}

func TestUserIsolation(t *testing.T) {
	e := newEnv(t)
	e.createUser("mario", false)
	admin, mario := e.client("admin"), e.client("mario")
	qr := e.createQR(admin, "https://test.com")

	id := itoa(qr.ID)
	expectStatus(t, e.do(mario, "GET", "/api/qrcodes/"+id, nil), http.StatusNotFound)
	expectStatus(t, e.do(mario, "PATCH", "/api/qrcodes/"+id, map[string]any{"target_url": "https://evil.com"}), http.StatusNotFound)
	expectStatus(t, e.do(mario, "DELETE", "/api/qrcodes/"+id, nil), http.StatusNotFound)
	expectStatus(t, e.do(mario, "GET", "/api/qrcodes/"+id+"/stats", nil), http.StatusNotFound)

	var list []qrListItem
	decode(t, e.do(mario, "GET", "/api/qrcodes", nil), &list)
	if len(list) != 0 {
		t.Fatalf("mario vede QR non suoi: %+v", list)
	}

	// Gestione utenti riservata agli admin
	expectStatus(t, e.do(mario, "GET", "/api/users", nil), http.StatusForbidden)
	expectStatus(t, e.do(admin, "GET", "/api/users", nil), http.StatusOK)
}

func TestAdminSeesEveryoneReadOnly(t *testing.T) {
	e := newEnv(t)
	e.createUser("mario", false)
	e.createUser("lucia", false)
	admin, mario, lucia := e.client("admin"), e.client("mario"), e.client("lucia")

	mq := e.createQR(mario, "https://mario.it")
	e.createQR(lucia, "https://lucia.it")
	own := e.createQR(admin, "https://admin.it")
	e.scan("GET", mq.Code, iPhoneUA)
	id := itoa(mq.ID)

	// L'elenco "miei" dell'admin contiene solo i suoi
	var mine []qrListItem
	decode(t, e.do(admin, "GET", "/api/qrcodes", nil), &mine)
	if len(mine) != 1 || mine[0].ID != own.ID || !mine[0].Editable {
		t.Fatalf("QR dell'admin: %+v", mine)
	}

	// La vista di tutti contiene i QR di ognuno, con il proprietario
	var all []qrListItem
	decode(t, e.do(admin, "GET", "/api/admin/qrcodes", nil), &all)
	if len(all) != 3 {
		t.Fatalf("tutti i QR: %d, attesi 3", len(all))
	}
	for _, q := range all {
		if q.ID == mq.ID && (q.Owner.Username != "mario" || q.Editable || q.ScansTotal != 1) {
			t.Fatalf("QR di mario visto dall'admin: %+v", q)
		}
	}
	var marioOnly []qrListItem
	decode(t, e.do(admin, "GET", "/api/admin/qrcodes?user_id="+itoa(mq.Owner.ID), nil), &marioOnly)
	if len(marioOnly) != 1 || marioOnly[0].ID != mq.ID {
		t.Fatalf("filtro per utente: %+v", marioOnly)
	}

	// Dettaglio e statistiche sì...
	var got qrJSON
	res := e.do(admin, "GET", "/api/qrcodes/"+id, nil)
	expectStatus(t, res, http.StatusOK)
	decode(t, res, &got)
	if got.Editable || got.Owner.Username != "mario" {
		t.Fatalf("dettaglio visto dall'admin: %+v", got)
	}
	var st statsJSON
	res = e.do(admin, "GET", "/api/qrcodes/"+id+"/stats", nil)
	expectStatus(t, res, http.StatusOK)
	decode(t, res, &st)
	if st.Total != 1 {
		t.Fatalf("statistiche viste dall'admin: %+v", st)
	}

	// ...modifiche no: il QR resta di mario
	expectStatus(t, e.do(admin, "PATCH", "/api/qrcodes/"+id, map[string]any{"target_url": "https://altro.it"}), http.StatusForbidden)
	expectStatus(t, e.do(admin, "DELETE", "/api/qrcodes/"+id, nil), http.StatusForbidden)
	res = e.scan("GET", mq.Code, iPhoneUA)
	if loc := res.Header.Get("Location"); loc != "https://mario.it" {
		t.Fatalf("la destinazione di mario è cambiata: %q", loc)
	}

	// Gli utenti normali restano chiusi nei propri QR
	expectStatus(t, e.do(mario, "GET", "/api/admin/qrcodes", nil), http.StatusForbidden)
	expectStatus(t, e.do(lucia, "GET", "/api/qrcodes/"+id, nil), http.StatusNotFound)
	expectStatus(t, e.do(lucia, "PATCH", "/api/qrcodes/"+id, map[string]any{"name": "x"}), http.StatusNotFound)

	// Pagina utenti: conteggi per utente (mario è stato scansionato due volte)
	var users []userJSON
	decode(t, e.do(admin, "GET", "/api/users", nil), &users)
	for _, u := range users {
		if u.Username == "mario" && (*u.QRCount != 1 || *u.ScanCount != 2) {
			t.Fatalf("conteggi di mario: qr=%d scansioni=%d", *u.QRCount, *u.ScanCount)
		}
	}
}

func TestAuthAndCSRF(t *testing.T) {
	e := newEnv(t)
	anon := e.client("")
	expectStatus(t, e.do(anon, "GET", "/api/qrcodes", nil), http.StatusUnauthorized)

	res := e.do(anon, "POST", "/api/login", map[string]string{"username": "admin", "password": "sbagliata"})
	expectStatus(t, res, http.StatusUnauthorized)

	c := e.client("admin")

	// Un form di un altro sito non può mandare JSON...
	req, _ := http.NewRequest("POST", e.ts.URL+"/api/qrcodes", strings.NewReader("target_url=https://x.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, _ = c.Do(req)
	expectStatus(t, res, http.StatusUnsupportedMediaType)

	// ...né arrivare con un'origine diversa
	req, _ = http.NewRequest("POST", e.ts.URL+"/api/qrcodes", strings.NewReader(`{"target_url":"https://x.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	res, _ = c.Do(req)
	expectStatus(t, res, http.StatusForbidden)

	// Validazione destinazione
	for _, target := range []string{"javascript:alert(1)", "test.com", "ftp://x.com", "http://qr.test/abc123"} {
		res := e.do(c, "POST", "/api/qrcodes", map[string]any{"target_url": target})
		expectStatus(t, res, http.StatusBadRequest)
	}

	// Logout: la sessione non vale più
	expectStatus(t, e.do(c, "POST", "/api/logout", nil), http.StatusNoContent)
	expectStatus(t, e.do(c, "GET", "/api/me", nil), http.StatusUnauthorized)
}

func TestUserManagement(t *testing.T) {
	e := newEnv(t)
	admin := e.client("admin")

	res := e.do(admin, "POST", "/api/users", map[string]any{"username": "lucia", "password": "lucia-password"})
	expectStatus(t, res, http.StatusCreated)
	var lucia userJSON
	decode(t, res, &lucia)
	expectStatus(t, e.do(admin, "POST", "/api/users", map[string]any{"username": "LUCIA", "password": "altra-password"}), http.StatusConflict)

	lc := e.client("lucia")
	expectStatus(t, e.do(lc, "GET", "/api/me", nil), http.StatusOK)

	// Disattivata: la sessione aperta smette di funzionare e non può più entrare
	expectStatus(t, e.do(admin, "PATCH", "/api/users/"+itoa(lucia.ID), map[string]any{"disabled": true}), http.StatusOK)
	expectStatus(t, e.do(lc, "GET", "/api/me", nil), http.StatusUnauthorized)
	res = e.do(e.client(""), "POST", "/api/login", map[string]string{"username": "lucia", "password": "lucia-password"})
	expectStatus(t, res, http.StatusUnauthorized)

	// L'admin non può chiudersi fuori da solo
	var me userJSON
	decode(t, e.do(admin, "GET", "/api/me", nil), &me)
	expectStatus(t, e.do(admin, "PATCH", "/api/users/"+itoa(me.ID), map[string]any{"is_admin": false}), http.StatusBadRequest)

	// Cambio password
	expectStatus(t, e.do(admin, "PUT", "/api/me/password", map[string]any{"current_password": "no", "new_password": "nuova-password"}), http.StatusBadRequest)
	expectStatus(t, e.do(admin, "PUT", "/api/me/password", map[string]any{"current_password": "admin-password", "new_password": "nuova-password"}), http.StatusNoContent)
	res = e.do(e.client(""), "POST", "/api/login", map[string]string{"username": "admin", "password": "nuova-password"})
	expectStatus(t, res, http.StatusOK)
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	anon := e.client("")
	for range 10 {
		e.do(anon, "POST", "/api/login", map[string]string{"username": "admin", "password": "sbagliata"})
	}
	res := e.do(anon, "POST", "/api/login", map[string]string{"username": "admin", "password": "admin-password"})
	expectStatus(t, res, http.StatusTooManyRequests)
}

func TestDashboardRoutes(t *testing.T) {
	e := newEnv(t)
	c := e.client("")
	res := e.do(c, "GET", "/", nil)
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/app/" {
		t.Fatalf("/ → %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	res = e.do(c, "GET", "/app/", nil)
	expectStatus(t, res, http.StatusOK)
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("manca la Content-Security-Policy sulla dashboard")
	}
	// Generatore di QR statici, senza accesso
	res = e.do(c, "GET", "/crea/", nil)
	expectStatus(t, res, http.StatusOK)
	if body, _ := io.ReadAll(res.Body); !strings.Contains(string(body), "generatore") {
		t.Fatalf("/crea/ non serve il generatore: %s", body)
	}
	res = e.do(c, "GET", "/crea", nil)
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/crea/" {
		t.Fatalf("/crea → %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	expectStatus(t, e.do(c, "GET", "/healthz", nil), http.StatusOK)
	expectStatus(t, e.do(c, "GET", "/health", nil), http.StatusOK)
	expectStatus(t, e.do(c, "GET", "/api/nope", nil), http.StatusNotFound)
}

func TestAggregateDaysAcrossDST(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Rome")
	// Il 26 ottobre 2025 in Italia si torna all'ora solare
	now := time.Date(2025, 10, 27, 10, 0, 0, 0, loc)
	from := startOfPeriod(now, 3, loc)
	if want := time.Date(2025, 10, 25, 0, 0, 0, 0, loc); !from.Equal(want) {
		t.Fatalf("inizio periodo %v, atteso %v", from, want)
	}
	scans := []store.Scan{
		{At: time.Date(2025, 10, 25, 23, 30, 0, 0, loc), Device: "Smartphone", OS: "iOS", Browser: "Safari"},
		{At: time.Date(2025, 10, 26, 0, 30, 0, 0, loc), Device: "Smartphone", OS: "Android", Browser: "Chrome"},
		{At: time.Date(2025, 10, 26, 23, 59, 0, 0, loc), Device: "Desktop", OS: "iOS", Browser: "Safari"},
		{At: time.Date(2025, 10, 27, 9, 0, 0, 0, loc), Device: "Smartphone", OS: "iOS", Browser: "Safari"},
	}
	got := aggregate(scans, from, 3, loc)
	want := []dayCount{{"2025-10-25", 1}, {"2025-10-26", 2}, {"2025-10-27", 1}}
	for i := range want {
		if got.Daily[i] != want[i] {
			t.Fatalf("giorni: %+v, attesi %+v", got.Daily, want)
		}
	}
	if got.OS[0] != (labelCount{"iOS", 3}) || got.Devices[0] != (labelCount{"Smartphone", 3}) {
		t.Fatalf("ripartizioni: %+v %+v", got.OS, got.Devices)
	}
}

func itoa(id int64) string {
	b, _ := json.Marshal(id)
	return string(b)
}
