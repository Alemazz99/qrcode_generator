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
