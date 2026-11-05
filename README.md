<p align="center">
  <img src="assets/icon.png" width="96" alt="Icona QR Generator">
</p>

<h1 align="center">QR Generator</h1>

<p align="center">
  QR code personalizzati (colori, stile, logo) in pochi secondi.<br>
  Come <b>app desktop</b>, come <b>web app</b> anche da telefono e come <b>servizio</b>
  per QR <b>dinamici</b>: destinazione modificabile e statistiche delle scansioni.
</p>

<p align="center">
  <img alt="Python 3.10+" src="https://img.shields.io/badge/python-3.10%2B-3776AB?logo=python&logoColor=white">
  <img alt="PySide6" src="https://img.shields.io/badge/GUI-PySide6%20(Qt)-41CD52?logo=qt&logoColor=white">
  <img alt="Go 1.26" src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white">
  <img alt="SQLite o Postgres" src="https://img.shields.io/badge/db-SQLite%20%7C%20Postgres-336791?logo=postgresql&logoColor=white">
  <img alt="Licenza MIT" src="https://img.shields.io/badge/licenza-MIT-blue">
</p>

<p align="center">
  <img src="assets/screenshot.png" alt="Screenshot di QR Generator" width="760">
</p>

---

## Tre modi di usarlo

| | Cosa fa | Dove |
|---|---|---|
| 🖥️ **App desktop** | Genera e salva QR statici in PNG, JPG o WebP | `app.py`, su Linux/Windows/macOS |
| 🌐 **Web app** | Le stesse opzioni nel browser, anche da telefono e offline | [alemazz99.github.io/qrcode_generator](https://alemazz99.github.io/qrcode_generator/) |
| 📊 **Servizio QR dinamici** | Dashboard con login: QR con destinazione modificabile, statistiche, più utenti | `server/`, da installare su un tuo server |

Le prime due creano QR **statici** e non hanno bisogno di nessun server. Il
servizio aggiunge i QR **dinamici** e offre comunque anche quelli statici.

### Statico o dinamico?

```mermaid
flowchart LR
    subgraph S["QR statico"]
        A["📱 scansione"] --> B["https://esempio.it"]
    end
    subgraph D["QR dinamico"]
        C["📱 scansione"] --> E["q.tuodominio.it/a7Kx2Q"] --> F{"servizio:<br>conta la scansione"} --> G["destinazione attuale<br>(modificabile)"]
    end
```

- **Statico**: il link (o un testo qualsiasi) è scritto dentro il QR. Funziona
  per sempre e senza server, ma non si può cambiare e non sai quante volte viene
  scansionato.
- **Dinamico**: il QR contiene un link corto fisso che passa dal servizio. Puoi
  cambiare la destinazione quando vuoi, e **il disegno resta identico** perché
  il link corto non cambia: niente ristampe. In più vedi quante scansioni arrivano,
  quando e da quali dispositivi.

## Personalizzazione

Valida per tutte e tre le versioni, che producono lo stesso identico QR:

- **Anteprima in tempo reale** mentre scrivi il link o cambi un'opzione
- **Colori** del QR e dello sfondo, oppure **sfondo trasparente**
- **6 stili per i moduli**: quadrato, arrotondato, quadrato distanziato, cerchi, barre verticali, barre orizzontali
- **2 stili per gli angoli** (finder pattern): quadrato e arrotondato
- **Logo centrale** con dimensione regolabile e riquadro di sfondo allineato alla griglia
- **Dimensione dei moduli, margine e livello di correzione errori** regolabili
- **Impostazioni ricordate** tra un utilizzo e l'altro, con **Ripristina predefiniti** per tornare ai valori iniziali
- Nome file proposto in automatico (es. `qr_github.com.png`)

## 🌐 Versione web (anche da telefono)

👉 **[alemazz99.github.io/qrcode_generator](https://alemazz99.github.io/qrcode_generator/)**

La stessa app, con le stesse opzioni e gli stessi limiti di leggibilità, gira
direttamente nel browser: niente da installare, funziona su telefono, tablet e PC.
Il QR viene generato sul dispositivo, quindi link e logo non vengono inviati a nessun server.

Per averla come un'app sul telefono, aprila e aggiungila alla schermata Home:

- **Android (Chrome)**: menu <kbd>⋮</kbd> → *Aggiungi a schermata Home* (o *Installa app*)
- **iPhone (Safari)**: pulsante *Condividi* → *Aggiungi alla schermata Home*

Da lì si apre a schermo intero e funziona anche **offline**. Sul telefono il
pulsante **Condividi** permette di salvare il QR direttamente nelle foto o di
mandarlo in chat.

> La versione su GitHub Pages è solo statica: login, QR dinamici e statistiche
> richiedono il servizio descritto qui sotto, installato su un tuo server.

## 📊 Servizio QR dinamici

Un piccolo servizio Go (un solo eseguibile) con una dashboard web. Il database è
SQLite in locale (un file, niente da installare) o Postgres in produzione (es. Supabase).

### Cosa puoi fare

- **Creare QR dinamici** e **cambiarne la destinazione** quando vuoi: il disegno
  resta identico e i QR già stampati continuano a funzionare
- Vedere le **statistiche**: totale, andamento giorno per giorno, ultima scansione,
  dispositivo, sistema operativo e browser, storico dei cambi di destinazione
- **Disattivare** un QR (chi lo scansiona vede "QR non più attivo") o eliminarlo.
  I codici non vengono mai riassegnati, quindi un QR stampato non potrà mai finire
  su un contenuto di qualcun altro
- Creare anche **QR statici**, salvati nell'elenco per riscaricarli quando serve.
  Il tipo si sceglie alla creazione e non si cambia più, perché cambierebbe il disegno
- Creare QR statici **senza accedere**, su `/crea/`: è la stessa web app, il QR
  viene generato nel browser e non viene salvato
- Distinguere i due tipi a colpo d'occhio nella pagina *I miei QR*: **blu** per i
  dinamici e **ambra** per gli statici, con un filtro per tipo

### Utenti e ruoli

Ognuno vede e modifica solo i propri QR. L'amministratore in più gestisce gli
account e vede i QR di tutti, ma in sola lettura.

| | Utente | Amministratore |
|---|:---:|:---:|
| Creare, modificare ed eliminare i propri QR | ✅ | ✅ |
| Vedere le statistiche dei propri QR | ✅ | ✅ |
| Cambiare la propria password | ✅ | ✅ |
| Creare utenti, reimpostare password, disattivare, nominare altri admin | | ✅ |
| Vedere QR e statistiche di **tutti** gli utenti (*Tutti i QR*) | | ✅ in sola lettura |
| Modificare o eliminare i QR di altri utenti | | |

### Privacy e sicurezza

- **Nessun IP salvato**: di ogni scansione restano solo data, tipo di dispositivo,
  sistema e browser
- Le anteprime dei link nelle chat (WhatsApp, Telegram, Slack…), i crawler, i
  precaricamenti del browser e le richieste `HEAD` **non vengono contati**
- Password con bcrypt, sessioni in cookie `HttpOnly` (`Secure` su HTTPS), limite
  ai tentativi di login, protezione CSRF e Content Security Policy sulla dashboard
- Redirect con `302` e `no-store`: nessun browser memorizza la vecchia destinazione

### Avvio in locale

Serve Go 1.26:

```bash
cp server/.env.example server/.env   # imposta almeno ADMIN_PASSWORD
make run                             # dalla root della repo
```

| Indirizzo | Cosa c'è |
|---|---|
| <http://localhost:8080/app/> | Dashboard (accedi con `ADMIN_USERNAME` / `ADMIN_PASSWORD`) |
| <http://localhost:8080/crea/> | Generatore di QR statici, senza accesso |
| `http://localhost:8080/<codice>` | Link corto di un QR dinamico: conta la scansione e fa il redirect |
| <http://localhost:8080/health> | Risponde `ok` se servizio e database sono attivi (anche `/healthz`) |

Le credenziali admin del `.env` vengono usate **solo al primo avvio**, quando il
database (`server/data/qr.db`) è vuoto. Dopo, la password si cambia dalla
dashboard in *Account*.

Per provarlo **col telefono** metti in `.env` `BASE_URL=http://<IP-del-PC>:8080`
(telefono e PC sulla stessa rete Wi-Fi) e, se serve, apri la porta nel firewall:
`sudo firewall-cmd --add-port=8080/tcp`. `BASE_URL` finisce dentro i QR, quindi
quelli creati in locale servono solo per le prove.

Altri comandi: `make test`, `make check` (gofmt, vet, test con race detector),
`make clean`. Per eseguire i test anche su Postgres basta indicare un database di
prova, che viene **svuotato** a ogni test:

```bash
podman run --rm -d --name qrtest-pg -p 127.0.0.1:55432:5432 -e POSTGRES_PASSWORD=test postgres:17-alpine
TEST_DATABASE_URL="postgres://postgres:test@127.0.0.1:55432/postgres?sslmode=disable" make test
podman stop qrtest-pg
```

<details>
<summary><b>Configurazione (<code>server/.env</code>)</b></summary>

| Variabile | Default | Descrizione |
|---|---|---|
| `PORT` | `8080` | Porta su cui ascolta il servizio |
| `BASE_URL` | `http://localhost:8080` | URL pubblico, senza `/` finale. Finisce in ogni QR dinamico: non va più cambiato dopo averli stampati |
| `DATABASE_URL` | | Se impostato (`postgres://…`) usa Postgres invece di SQLite |
| `DB_PATH` | `data/qr.db` | File del database SQLite, usato se `DATABASE_URL` è vuoto |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | | Primo amministratore, usato solo con database vuoto |
| `TRUST_PROXY` | `false` | `true` se il servizio sta dietro un reverse proxy che imposta `X-Forwarded-For` |
| `TIMEZONE` | `Europe/Rome` | Fuso orario con cui raggruppare le scansioni per giorno |

</details>

<details>
<summary><b>API</b></summary>

La dashboard usa queste API JSON; le richieste che modificano dati vogliono
`Content-Type: application/json`.

| Metodo e percorso | Chi | Cosa fa |
|---|---|---|
| `POST /api/login`, `POST /api/logout` | tutti | Apre e chiude la sessione (cookie) |
| `GET /api/me`, `PUT /api/me/password` | utente | Dati dell'utente, cambio password |
| `GET /api/qrcodes`, `POST /api/qrcodes` | utente | Elenco e creazione dei propri QR (`kind`: `dynamic` o `static`) |
| `GET/PATCH/DELETE /api/qrcodes/{id}` | utente | Dettaglio, modifica, eliminazione |
| `GET /api/qrcodes/{id}/stats?days=30` | utente | Statistiche e storico delle destinazioni |
| `GET /api/qrcodes/{id}/logo` | utente | Logo del QR (PNG) |
| `GET /api/admin/qrcodes?user_id=` | admin | QR di tutti gli utenti, o di uno solo |
| `GET/POST /api/users`, `PATCH /api/users/{id}` | admin | Gestione degli utenti |

</details>

### In produzione

Due strade, a seconda di dove lo metti.

**Piattaforma senza disco persistente + Supabase** (es. Koyeb, gratuito). Il file
SQLite verrebbe cancellato a ogni riavvio, quindi si usa Postgres:

1. Su Supabase crea un progetto e copia la stringa di connessione da
   *Connect → Session pooler* (IPv4). Le tabelle le crea il servizio al primo avvio,
   con la Row Level Security attiva, quindi non sono leggibili dalle API pubbliche
   di Supabase.
2. Sulla piattaforma crea un servizio da questo repository con il `Dockerfile`
   nella root e le variabili `BASE_URL`,
   `DATABASE_URL`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`, `TRUST_PROXY=true`.
3. Collega il tuo dominio (es. `q.tuodominio.it`) e usalo in `BASE_URL`.
4. Se la piattaforma addormenta i servizi inattivi, un monitoraggio gratuito che
   chiama `/health` ogni ora lo tiene sveglio. `/health` interroga anche il
   database, quindi tiene attivo anche Supabase, che mette in pausa i progetti
   gratuiti dopo una settimana senza attività.

**Server tuo con Docker** (SQLite su un volume):

```bash
docker build -t qrserver .
docker run -d --name qrserver -p 127.0.0.1:8080:8080 -v qrdata:/data \
  -e BASE_URL=https://q.esempio.it -e TRUST_PROXY=true \
  -e ADMIN_USERNAME=admin -e ADMIN_PASSWORD='…' qrserver
```

Mettilo dietro un reverse proxy con HTTPS. Con Caddy basta:

```caddy
q.esempio.it {
    reverse_proxy 127.0.0.1:8080
}
```

- Con `BASE_URL` in `https` i cookie di sessione diventano automaticamente `Secure`.
- Scegli il dominio con cura **prima** di stampare i QR, perché non si potrà più cambiare.
- Per il backup: con SQLite basta copiare `/data/qr.db` (con i file `-wal`/`-shm`
  a servizio fermo, oppure a caldo con `sqlite3 qr.db ".backup copia.db"`); con
  Postgres usa `pg_dump`.

## 🖥️ App desktop

### Requisiti

- Python **3.10** o superiore
- Le dipendenze in [`requirements.txt`](requirements.txt): [PySide6](https://doc.qt.io/qtforpython-6/) e [qrcode](https://github.com/lincolnloop/python-qrcode) (con Pillow)

Sviluppata e provata su **Linux** (Fedora Silverblue, GNOME su Wayland). Le librerie
usate sono multipiattaforma, quindi dovrebbe funzionare anche su Windows e macOS,
ma lì non è stata testata.

### Installazione e avvio

```bash
git clone https://github.com/Alemazz99/qrcode_generator.git
cd qrcode_generator
./run.sh
```

Al primo avvio `run.sh` crea un ambiente virtuale in `.venv/` e installa le
dipendenze; dalle volte successive parte subito. Nel campo del link premi
<kbd>Invio</kbd> per salvare al volo.

<details>
<summary><b>Avvio manuale (Windows, macOS o se preferisci non usare lo script)</b></summary>

```bash
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt     # Windows: .venv\Scripts\pip install -r requirements.txt
.venv/bin/python app.py                       # Windows: .venv\Scripts\python app.py
```

</details>

<details>
<summary><b>Usarla dentro un distrobox (Fedora Silverblue / sistemi immutabili)</b></summary>

Se hai un [distrobox](https://distrobox.it/) chiamato `python`, `run.sh` lo
rileva da solo e avvia l'app lì dentro: sull'host non viene installato nulla.
Per usare un container con un altro nome, o per disattivare il comportamento:

```bash
QRGEN_CONTAINER=mio-container ./run.sh   # usa un altro distrobox
QRGEN_CONTAINER= ./run.sh                # avvio diretto, senza container
```

Il container deve avere accesso al display (distrobox lo fa di default).

</details>

### Aggiungerla al menu applicazioni (Linux)

```bash
./install-launcher.sh
```

Crea `~/.local/share/applications/qrcode-generator.desktop`, così "QR Generator"
compare tra le applicazioni di GNOME/KDE con la sua icona. Per rimuoverla basta
cancellare quel file.

### Uso da codice

La generazione vive in [`qr_core.py`](qr_core.py), separata dall'interfaccia,
quindi puoi usarla anche in uno script o in un altro progetto:

```python
from qr_core import QrOptions, generate, save

opts = QrOptions(
    data="https://github.com",
    fg_color="#1a3d7c",
    transparent=True,
    module_style="Arrotondato",
    logo_path="logo.png",
    logo_ratio=0.22,
)
img = generate(opts)          # PIL.Image in RGBA
save(img, "qr_github.png")    # .png / .jpg / .webp
```

| Opzione        | Default      | Descrizione                                                          |
| -------------- | ------------ | -------------------------------------------------------------------- |
| `data`         | -            | Link o testo da codificare                                           |
| `fg_color`     | `#000000`    | Colore dei moduli                                                    |
| `bg_color`     | `#ffffff`    | Colore di sfondo (usato anche per appiattire i JPG)                  |
| `transparent`  | `False`      | Sfondo trasparente                                                   |
| `module_style` | `Quadrato`   | Una delle chiavi di `MODULE_STYLES`                                  |
| `eye_style`    | `Quadrato`   | Una delle chiavi di `EYE_STYLES`                                     |
| `box_size`     | `30`         | Pixel per modulo                                                     |
| `border`       | `2`          | Margine, in moduli                                                   |
| `error_level`  | `M - 15%`    | Una delle chiavi di `ERROR_LEVELS` (ignorato se c'è un logo)         |
| `logo_path`    | `None`       | Immagine da mettere al centro                                        |
| `logo_ratio`   | `0.22`       | Lato del logo rispetto al QR, massimo `MAX_LOGO_RATIO` (0.24)        |
| `logo_padding` | `True`       | Svuota un riquadro dietro al logo                                    |

I valori di default di `QrOptions` sono anche quelli che l'app ripristina con
**Ripristina predefiniti**: per cambiarli basta modificare la dataclass.

## Leggibilità: le scelte fatte

Un QR code personalizzato è utile solo se il telefono lo legge. Per questo alcune
opzioni hanno dei limiti voluti, verificati decodificando centinaia di varianti con
[ZXing](https://github.com/zxing-cpp/zxing-cpp) e OpenCV e provando con smartphone reali:

- **Con un logo la correzione errori passa sempre a H (30%)**: il logo copre una
  parte dei moduli e serve la ridondanza massima per recuperarli.
- **Il logo non può superare il 24% del lato**: dal 25% in su i telefoni smettono
  di riconoscere il codice.
- **Gli angoli sono solo pieni** (quadrato o arrotondato): con cerchi o quadrati
  staccati i lettori non trovano più i *finder pattern* e il QR diventa illeggibile.
  Lo stile "creativo" resta disponibile per i moduli, dove non crea problemi.
- **Il riquadro dietro al logo è allineato alla griglia**, così nessun modulo resta
  tagliato a metà.

Consiglio comunque di provare sempre il QR con un telefono prima di stamparlo,
soprattutto con colori chiari su sfondo chiaro, con sfondo trasparente o con
**link lunghi insieme a un logo**: più il link è lungo, più il QR è fitto e più
conviene tenere il logo piccolo. I QR dinamici aiutano anche qui: il link corto
ha sempre la stessa lunghezza, qualunque sia la destinazione.

## Dove vengono salvate le impostazioni

Colori, stili, ultimo logo e cartella di salvataggio dell'app desktop vengono
ricordati tramite `QSettings` di Qt:

- **Linux**: `~/.config/qrcode_generator/settings.conf`
- **Windows**: registro, in `HKEY_CURRENT_USER\Software\qrcode_generator\settings`
- **macOS**: `~/Library/Preferences/com.qrcode_generator.settings.plist`

Cancellare quel file (o premere **Ripristina predefiniti**) riporta tutto ai valori iniziali.

Nella versione web le preferenze (e il logo) restano solo nel browser del
dispositivo, nel `localStorage` della pagina. Nel servizio lo stile di ogni QR è
salvato nel database insieme al QR.

## Struttura del progetto

```
qrcode_generator/
├── app.py                # Interfaccia grafica desktop (PySide6)
├── qr_core.py            # Generazione del QR, indipendente dalla UI
├── run.sh                # Avvio: crea il venv al primo lancio, usa distrobox se presente
├── install-launcher.sh   # Aggiunge l'app al menu applicazioni (Linux)
├── requirements.txt
├── Makefile              # make run/test/check: richiama server/
├── Dockerfile            # Immagine del servizio (contesto: la root, serve anche docs/)
├── TODO.md               # Idee per il futuro
├── assets/
│   ├── icon.png
│   └── screenshot.png
├── docs/                 # Versione web, pubblicata con GitHub Pages
│   ├── index.html
│   ├── app.js            # Interfaccia web
│   ├── qr-render.js      # Generazione del QR su canvas: stessa logica di qr_core.py
│   ├── sw.js             # Service worker per l'uso offline
│   ├── manifest.webmanifest
│   ├── style.css
│   ├── icons/
│   └── vendor/qrcode.js  # qrcode-generator di Kazuhiko Arase (MIT)
└── server/               # Servizio dei QR dinamici (Go)
    ├── main.go           # Avvio, primo amministratore, arresto pulito
    ├── .env.example
    ├── internal/
    │   ├── config/       # Lettura di .env e variabili d'ambiente
    │   ├── store/        # SQLite o Postgres: utenti, sessioni, QR, scansioni, migrazioni
    │   ├── auth/         # Password (bcrypt), sessioni, codici, limite ai login
    │   ├── useragent/    # Dispositivo/sistema/browser e riconoscimento dei bot
    │   └── httpapi/      # Redirect, API JSON, statistiche
    └── web/              # Dashboard su /app/ (inclusa nell'eseguibile)
        └── public/       # Copia di docs/ fatta da `make assets`, servita su /crea/ (non versionata)
```

`make assets` copia la web app di `docs/` in `server/web/public/`: il servizio la
offre su `/crea/` per i QR statici senza accesso, e la dashboard ne usa
`qr-render.js` e `style.css`. C'è quindi una sola copia del codice di disegno nel
browser, e un QR scaricato dalla dashboard è identico a uno fatto con la versione
web con lo stesso contenuto e lo stesso stile. La web app sta in una cartella sua
perché il suo service worker mette in cache tutto ciò che si trova nel suo percorso.

Se modifichi le regole di generazione (stili, limiti, valori predefiniti), tieni
allineati `qr_core.py` e `docs/qr-render.js`, così app desktop e versione web
producono lo stesso risultato.

Per provare la sola versione web in locale serve un piccolo server (il service
worker non funziona aprendo il file direttamente):

```bash
python3 -m http.server -d docs 8000   # poi apri http://localhost:8000
```

## Risoluzione problemi

- **L'app desktop non parte in un container minimale / errore su `libEGL`,
  `libxkbcommon` o simili**: le wheel di PySide6 includono Qt ma si appoggiano ad
  alcune librerie grafiche di sistema. Su Fedora: `sudo dnf install mesa-libEGL libxkbcommon fontconfig`
  (su Debian/Ubuntu: `libegl1 libxkbcommon0 fontconfig`).
- **Si apre una finestra enorme o "stirata"**: se usi un window manager a tiling
  (es. Pop Shell) è lui a decidere le dimensioni della finestra all'apertura.
- **Il servizio non parte: "porta già usata da un altro servizio"**: un altro
  programma occupa la porta; cambia `PORT` e `BASE_URL` in `server/.env`.
- **Dal telefono il QR dinamico non si apre**: dentro il QR c'è `BASE_URL`. Se è
  `localhost`, sul telefono non porta da nessuna parte: usa l'IP del PC (vedi
  *Avvio in locale*) e riscarica il PNG.

## Licenza

Distribuito con licenza [MIT](LICENSE): puoi usarlo, modificarlo e ridistribuirlo
liberamente, anche per uso commerciale.
