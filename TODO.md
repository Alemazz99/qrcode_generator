# TODO / idee

Lista di idee raccolte strada facendo. Non sono impegni: prima di implementarne
una vanno sciolti i punti aperti.

---

## Domini aggiuntivi per i QR dinamici

**Idea**: il `BASE_URL` del `.env` resta il dominio principale e predefinito per
tutti i QR. Solo l'admin può creare un QR dinamico su un dominio diverso, per
esempio per un cliente o un marchio con il suo dominio.

### Come funzionerebbe
- Ogni QR dinamico salva il proprio dominio alla creazione. Il link corto è
  `dominio + "/" + codice`.
- Il dominio di un QR non si cambia dopo: cambierebbe il disegno.
- Senza scelta vale il `BASE_URL` del `.env`, e i QR esistenti restano su quello.
- Il dominio principale ospita dashboard e login. Gli altri servono solo i
  redirect, e `/app/` su un dominio secondario rimanda al principale.

### Attenzione
Un dominio funziona solo se **DNS**, **reverse proxy** e **certificato HTTPS**
arrivano davvero al servizio. Se manca uno dei tre, il QR stampato non funziona
e te ne accorgi solo quando qualcuno lo scansiona.

### Proposta
- **Lista di domini gestita dall'admin**: una pagina "Domini" accanto a "Utenti".
  In creazione del QR il dominio si sceglie da un menu e non si scrive a mano,
  così un errore di battitura non finisce stampato per sempre.
- Un pulsante **Verifica** controlla che `https://dominio/...` risponda proprio
  questo servizio. Finché la verifica non passa, il dominio non si può usare.
- Un dominio con QR collegati non si elimina: al massimo si disattiva per i
  QR nuovi.
- Il controllo "la destinazione non può puntare al servizio stesso" va esteso
  a tutti i domini in lista.
- **Certificati automatici con Caddy** (*on-demand TLS*): Caddy chiede al
  servizio "questo dominio è tuo?" tramite un endpoint e, se è in lista,
  ottiene il certificato da solo. Aggiungere un dominio diventa: punto il DNS,
  lo aggiungo in dashboard, fatto.
- In locale si prova senza DNS con i nomi `*.localhost`, che il browser risolve
  da solo (es. `http://promo.localhost:8080`). Dal telefono serve un DNS vero.

### Punti aperti
- [ ] Domini: lista gestita dall'admin oppure campo libero in creazione?
- [ ] Permessi: solo l'admin, oppure l'admin assegna dei domini a singoli utenti?
- [ ] Un codice risponde solo sul proprio dominio (marchi separati) o su tutti
      (più tollerante)?
- [ ] Endpoint per l'on-demand TLS di Caddy, o certificati gestiti a mano?

---

## QR dinamici funzionanti anche a servizio spento

**Idea**: un QR dinamico oggi funziona solo se il servizio Go risponde. L'obiettivo
è che continui a funzionare anche con il servizio spento, addormentato o in crash,
restando indipendenti da un singolo provider.

**Il limite fisico**: al momento della scansione *qualcosa* deve rispondere
all'URL scritto nel QR, perché la destinazione attuale deve stare da qualche parte.
Senza niente di acceso da nessuna parte è impossibile, per qualsiasi soluzione.
Si può però ridurre la parte che deve restare online a qualcosa di minimo,
standard e intercambiabile.

**Regola d'oro, valida per qualunque soluzione**: nei QR va sempre un dominio
nostro (es. `q.fearlix.com`), mai `*.onrender.com` o `*.supabase.co`. Così si
cambia provider spostando solo il DNS, senza ristampare nulla.

### Soluzione proposta: "pubblicazione statica"
- Il servizio Go (in locale o online) gestisce i QR come oggi e, con un comando
  **Pubblica**, genera i redirect come file statici e li carica online.
- Online resta solo qualcosa di "stupido" che serve quei file:
  - **nginx/Caddy su un nostro server**: 302 veri, e i **log degli accessi**
    permettono di recuperare tutte le scansioni importandole in seguito (scartando
    l'IP);
  - **oppure un hosting statico** (Render Static Site, Netlify, GitHub Pages):
    redirect tramite una pagina HTML/JS invece del 302, e nessun log.
- Cambio di destinazione = modifica + ripubblicazione. Il disegno resta identico.
- Variante gratuita con Render + Supabase: redirect su Render Static Site; la
  pagina segnala la scansione a Supabase con una chiave pubblica che può solo
  inserire nella tabella delle scansioni; il servizio Go importa poi le scansioni.
  Se Supabase va in pausa i redirect continuano a funzionare, si perdono solo le
  statistiche di quel periodo.

### Compromessi
| Aspetto | Servizio sempre acceso (oggi) | Pubblicazione statica |
|---|---|---|
| QR funzionanti col servizio spento | ❌ | ✅ |
| Cambio di destinazione | immediato | alla ripubblicazione (secondi o un paio di minuti) |
| Statistiche | in tempo reale | importate dopo, dai log o da Supabase |
| Login esposto su internet | sì | no, se la dashboard resta solo in locale |
| Più utenti online | ✅ | solo se la dashboard resta comunque online |

Rischi della variante con Supabase: la chiave pubblica permette di **gonfiare le
statistiche** con scansioni false (rischio sulla qualità dei numeri, non sulla
sicurezza), e il redirect HTML aggiunge qualche centinaio di millisecondi.

### Scartate
- **Edge di terzi (Cloudflare Worker + KV)**: robustissimo ma lega a Cloudflare,
  e l'obiettivo è restare indipendenti.
- **Cache del CDN "servi la copia vecchia se il server non risponde"**: non
  copre i QR mai scansionati prima e perde le scansioni durante il fermo.
- **Due server con database replicato**: molto più da gestire, per un risultato
  inferiore.

### Punti aperti
- [ ] Dove far girare la parte statica: nginx/Caddy su un nostro server o un hosting statico?
- [ ] Dashboard solo in locale o anche online per gli altri utenti?
- [ ] Verificare che i Render Static Site non si addormentino e come gestiscono gli header di cache.

---

## `BASE_URL` visibile nella dashboard

Mostrare all'admin, in sola lettura, il dominio usato per i link corti, con
l'avviso di non cambiarlo dopo aver stampato i QR. Rendere `BASE_URL` modificabile
dalla dashboard è stato scartato: un cambio romperebbe tutti i QR già stampati,
e dominio, DNS e certificati si decidono al deploy, non dall'app.

---

## ✅ Fatto: admin che vede i QR di tutti gli utenti

L'admin ha la pagina "Tutti i QR" (filtrabile per utente e per tipo) e può aprire
qualsiasi QR per vederne statistiche e storico e scaricarlo, ma solo il
proprietario può modificarlo o eliminarlo.
