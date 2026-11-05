// Dashboard dei QR dinamici: elenco, modifica (destinazione e stile), statistiche e utenti.
// Il disegno del QR usa lo stesso renderer della web app (vendor/qr-render.js).
(function () {
  'use strict';

  const R = window.QRRender;
  const LOGO_MAX_SIDE = 1024;
  const LAST_STYLE_KEY = 'qr_dashboard.last_style';
  const FILTER_KEY = 'qr_dashboard.kind_filter';

  const $ = (id) => document.getElementById(id);
  const el = {
    nav: $('nav'),
    navUsers: $('nav-users'),
    navAll: $('nav-all'),
    preview: $('preview'),
    info: $('info'),
    download: $('download'),
    status: $('status'),
    // stile
    fg: $('fg'),
    fgHex: $('fg-hex'),
    bg: $('bg'),
    bgHex: $('bg-hex'),
    transparent: $('transparent'),
    moduleStyle: $('module-style'),
    eyeStyle: $('eye-style'),
    logoInput: $('logo-input'),
    logoName: $('logo-name'),
    logoClear: $('logo-clear'),
    logoRatio: $('logo-ratio'),
    logoRatioValue: $('logo-ratio-value'),
    logoPadding: $('logo-padding'),
    boxSize: $('box-size'),
    boxSizeValue: $('box-size-value'),
    border: $('border'),
    borderValue: $('border-value'),
    errorLevel: $('error-level'),
    errorNote: $('error-note'),
    // destinazione
    qrName: $('qr-name'),
    qrTarget: $('qr-target'),
    qrActive: $('qr-active'),
    qrSave: $('qr-save'),
    dirty: $('dirty'),
  };

  let me = null; // utente collegato
  let qr = null; // QR aperto nel dettaglio, così come salvato sul server
  let logo = null; // { image, name, dataUrl }: dataUrl solo per un logo appena scelto
  let logoDirty = false;
  let current = null; // canvas dell'ultimo QR disegnato
  let saved = ''; // istantanea del form all'ultimo salvataggio
  let timer = 0;
  let routeSeq = 0; // scarta le risposte arrivate dopo un cambio di pagina
  let qrItems = []; // ultimo elenco caricato, filtrato lato client
  let listMode = 'mine'; // 'mine' = I miei QR, 'all' = vista dell'admin
  let listHash = '#/'; // elenco da cui si è aperto il dettaglio, per il "torna indietro"
  let kindFilter = storageGet(FILTER_KEY) || 'all';

  // ---- Utilità -----------------------------------------------------------------

  class ApiError extends Error {
    constructor(status, message) {
      super(message);
      this.status = status;
    }
  }

  async function api(method, path, body) {
    const opts = { method, headers: {} };
    if (method !== 'GET') {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body || {});
    }
    let res;
    try {
      res = await fetch('/api' + path, opts);
    } catch (e) {
      throw new ApiError(0, 'server non raggiungibile');
    }
    if (res.status === 204) return null;
    let data = null;
    try {
      data = await res.json();
    } catch (e) {
      data = null;
    }
    if (!res.ok) {
      if (res.status === 401 && path !== '/login') {
        me = null;
        showLogin();
      }
      throw new ApiError(res.status, (data && data.error) || `errore ${res.status}`);
    }
    return data;
  }

  // Crea elementi senza mai passare da innerHTML: i dati restano testo
  function h(tag, props, ...children) {
    const node = document.createElement(tag);
    for (const [k, v] of Object.entries(props || {})) {
      if (k === 'class') node.className = v;
      else if (k.startsWith('on')) node.addEventListener(k.slice(2), v);
      else if (v === true) node.setAttribute(k, '');
      else if (v !== false && v != null) node.setAttribute(k, v);
    }
    for (const c of children.flat()) if (c != null) node.append(c);
    return node;
  }

  const SVG_NS = 'http://www.w3.org/2000/svg';
  function svg(tag, attrs, ...children) {
    const node = document.createElementNS(SVG_NS, tag);
    for (const [k, v] of Object.entries(attrs || {})) node.setAttribute(k, v);
    for (const c of children) node.append(c);
    return node;
  }

  const num = new Intl.NumberFormat('it-IT');
  const dateTime = new Intl.DateTimeFormat('it-IT', { dateStyle: 'medium', timeStyle: 'short' });
  const shortDay = new Intl.DateTimeFormat('it-IT', { day: 'numeric', month: 'short' });

  function formatDateTime(iso) {
    return iso ? dateTime.format(new Date(iso)) : 'mai';
  }

  // "AAAA-MM-GG" → "5 nov" senza passare da UTC
  function formatDay(day) {
    const [y, m, d] = day.split('-').map(Number);
    return shortDay.format(new Date(y, m - 1, d));
  }

  function plural(n, one, many) {
    return `${num.format(n)} ${n === 1 ? one : many}`;
  }

  function storageGet(key) {
    try {
      return localStorage.getItem(key);
    } catch (e) {
      return null;
    }
  }

  function storageSet(key, value) {
    try {
      localStorage.setItem(key, value);
    } catch (e) {
      // storage non disponibile: è solo una comodità
    }
  }

  let toastTimer = 0;
  function toast(text) {
    el.status.textContent = text;
    el.status.classList.add('visible');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.status.classList.remove('visible'), 2800);
  }

  function showError(node, message) {
    node.textContent = message || '';
    node.hidden = !message;
  }

  function loadImage(src) {
    return new Promise((resolve, reject) => {
      const img = new Image();
      img.onload = () => resolve(img);
      img.onerror = () => reject(new Error('immagine non leggibile'));
      img.src = src;
    });
  }

  // ---- Viste e navigazione -------------------------------------------------------

  const VIEWS = ['login', 'list', 'qr', 'users', 'account'];

  // nav è la voce di menu da evidenziare, se diversa dalla vista
  function show(name, nav) {
    for (const v of VIEWS) $('view-' + v).hidden = v !== name;
    for (const a of el.nav.querySelectorAll('a')) {
      a.classList.toggle('current', a.dataset.nav === (nav || name));
    }
    window.scrollTo(0, 0);
  }

  function showLogin() {
    el.nav.hidden = true;
    show('login');
    $('login-username').focus();
  }

  function route() {
    routeSeq++;
    if (!me) return showLogin();
    el.nav.hidden = false;
    el.navUsers.hidden = el.navAll.hidden = !me.is_admin;
    const hash = location.hash || '#/';
    const m = hash.match(/^#\/qr\/(\d+)$/);
    if (m) return openQR(Number(m[1]));
    const all = hash.match(/^#\/all(?:\/(\d+))?$/);
    if (all && me.is_admin) return openList('all', all[1] ? Number(all[1]) : 0);
    if (hash === '#/users' && me.is_admin) return openUsers();
    if (hash === '#/account') return openAccount();
    return openList('mine', 0);
  }

  // ---- Login ---------------------------------------------------------------------

  async function login(event) {
    event.preventDefault();
    const err = $('login-error');
    showError(err, '');
    try {
      me = await api('POST', '/login', {
        username: $('login-username').value.trim(),
        password: $('login-password').value,
      });
      $('login-password').value = '';
      route();
    } catch (e) {
      showError(err, e.message);
    }
  }

  async function logout() {
    try {
      await api('POST', '/logout');
    } catch (e) {
      // anche se fallisce, lato client si esce comunque
    }
    me = null;
    location.hash = '#/';
    showLogin();
  }

  // ---- Elenco ----------------------------------------------------------------------

  // mode 'all' è la vista dell'admin: i QR di tutti, o di un solo utente (ownerId)
  async function openList(mode, ownerId) {
    listMode = mode;
    listHash = location.hash || '#/';
    const isAll = mode === 'all';
    show('list', isAll ? 'all' : 'list');
    $('list-title').textContent = isAll ? 'Tutti i QR' : 'I miei QR';
    $('new-toggle').hidden = isAll;
    $('all-note').hidden = !isAll;
    if (isAll) $('new-form').hidden = true;
    $('owner-filter').hidden = !isAll;
    const seq = routeSeq;
    const list = $('qr-list');
    list.replaceChildren(h('p', { class: 'muted' }, 'Caricamento…'));
    try {
      const path = isAll ? '/admin/qrcodes' + (ownerId ? `?user_id=${ownerId}` : '') : '/qrcodes';
      const [items, users] = await Promise.all([api('GET', path), isAll ? api('GET', '/users') : null]);
      if (seq !== routeSeq) return;
      if (isAll) fillOwnerFilter(users, ownerId);
      qrItems = items;
      renderList();
    } catch (e) {
      if (seq === routeSeq) list.replaceChildren(h('p', { class: 'form-error' }, e.message));
    }
  }

  function renderList() {
    const list = $('qr-list');
    const filter = $('kind-filter');
    const counts = {
      all: qrItems.length,
      dynamic: qrItems.filter((q) => q.kind === 'dynamic').length,
      static: qrItems.filter((q) => q.kind === 'static').length,
    };
    if (!(kindFilter in counts)) kindFilter = 'all';
    filter.hidden = !qrItems.length;
    for (const btn of filter.querySelectorAll('button')) {
      btn.setAttribute('aria-selected', String(btn.dataset.kind === kindFilter));
      btn.querySelector('span').textContent = num.format(counts[btn.dataset.kind]);
    }

    if (!qrItems.length) {
      list.replaceChildren(h('p', { class: 'empty' }, 'Nessun QR ancora: crea il primo con "Nuovo QR".'));
      return;
    }
    const shown = kindFilter === 'all' ? qrItems : qrItems.filter((q) => q.kind === kindFilter);
    if (!shown.length) {
      const what = kindFilter === 'static' ? 'statico' : 'dinamico';
      list.replaceChildren(h('p', { class: 'empty' }, `Nessun QR ${what}.`));
      return;
    }
    list.replaceChildren(...shown.map(qrCard));
  }

  function fillOwnerFilter(users, ownerId) {
    const select = $('owner-filter');
    select.replaceChildren(
      new Option('Tutti gli utenti', ''),
      ...users.map((u) => new Option(`${u.username} (${num.format(u.qr_count || 0)} QR)`, String(u.id))));
    select.value = ownerId ? String(ownerId) : '';
  }

  function setKindFilter(kind) {
    kindFilter = kind;
    storageSet(FILTER_KEY, kind);
    renderList();
  }

  function qrCard(q) {
    const card = kindCard(q);
    // Nella vista dell'admin ogni QR dice di chi è
    if (listMode === 'all') {
      card.append(q.editable
        ? h('span', { class: 'owner' }, 'Tuo')
        : h('span', { class: 'owner' }, 'di ', h('strong', null, q.owner.username), ' · sola lettura'));
    }
    return card;
  }

  function kindCard(q) {
    if (q.kind === 'static') {
      return h('a', { class: 'qr-item static', href: `#/qr/${q.id}` },
        h('div', { class: 'qr-item-head' },
          h('strong', null, q.name || 'Senza nome'),
          h('span', { class: 'badge kind static' }, 'Statico')),
        h('code', null, q.target_url),
        h('span', { class: 'counts muted' }, 'Contenuto scritto nel QR · nessuna statistica'));
    }
    return h('a', { class: 'qr-item dynamic', href: `#/qr/${q.id}` },
      h('div', { class: 'qr-item-head' },
        h('strong', null, q.name || 'Senza nome'),
        h('span', { class: 'badges' },
          h('span', { class: 'badge kind dynamic' }, 'Dinamico'),
          h('span', { class: q.active ? 'badge' : 'badge off' }, q.active ? 'Attivo' : 'Disattivato'))),
      h('code', null, q.short_url),
      h('span', { class: 'target' }, '→ ' + q.target_url),
      h('span', { class: 'counts' },
        `${plural(q.scans_total, 'scansione', 'scansioni')} · ${num.format(q.scans_7d)} negli ultimi 7 giorni`));
  }

  function toggleNewForm(open) {
    $('new-form').hidden = !open;
    showError($('new-error'), '');
    updateNewKind();
    if (open) $('new-target').focus();
  }

  function newKind() {
    return document.querySelector('input[name="new-kind"]:checked').value;
  }

  // Il campo cambia con il tipo: un QR statico può contenere anche testo libero
  function updateNewKind() {
    const isStatic = newKind() === 'static';
    const target = $('new-target');
    target.type = isStatic ? 'text' : 'url';
    target.placeholder = isStatic ? 'https://esempio.it oppure un testo' : 'https://esempio.it';
    $('new-target-label').textContent = isStatic ? 'Link o testo' : 'Link di destinazione';
    $('new-note').textContent = isStatic
      ? 'Il contenuto viene scritto direttamente nel QR: se un domani cambia, il QR va rigenerato e ristampato.'
      : 'Il QR conterrà un link corto fisso: potrai cambiare la destinazione quando vuoi senza ristamparlo.';
  }

  async function createQR(event) {
    event.preventDefault();
    const err = $('new-error');
    showError(err, '');
    try {
      const created = await api('POST', '/qrcodes', {
        kind: newKind(),
        name: $('new-name').value,
        target_url: $('new-target').value,
        style: lastStyle(),
      });
      $('new-form').reset();
      toggleNewForm(false);
      toast(created.kind === 'static' ? 'QR statico creato: personalizzalo e scaricalo' : 'QR creato: personalizzalo e scaricalo');
      location.hash = `#/qr/${created.id}`;
    } catch (e) {
      showError(err, e.message);
    }
  }

  // Un nuovo QR parte dallo stile dell'ultimo salvato: di solito si usa sempre lo stesso
  function lastStyle() {
    try {
      return sanitizeStyle(JSON.parse(storageGet(LAST_STYLE_KEY)));
    } catch (e) {
      return { ...R.DEFAULTS };
    }
  }

  // ---- Dettaglio: stile -------------------------------------------------------------

  function sanitizeStyle(saved) {
    const s = { ...R.DEFAULTS };
    if (saved && typeof saved === 'object') {
      for (const key of Object.keys(R.DEFAULTS)) {
        if (typeof saved[key] === typeof R.DEFAULTS[key]) s[key] = saved[key];
      }
    }
    if (!R.MODULE_STYLES.includes(s.moduleStyle)) s.moduleStyle = R.DEFAULTS.moduleStyle;
    if (!R.EYE_STYLES.includes(s.eyeStyle)) s.eyeStyle = R.DEFAULTS.eyeStyle;
    if (!(s.errorLevel in R.ERROR_LEVELS)) s.errorLevel = R.DEFAULTS.errorLevel;
    return s;
  }

  function readStyle() {
    return {
      fgColor: el.fg.value,
      bgColor: el.bg.value,
      transparent: el.transparent.checked,
      moduleStyle: el.moduleStyle.value,
      eyeStyle: el.eyeStyle.value,
      boxSize: Number(el.boxSize.value),
      border: Number(el.border.value),
      errorLevel: el.errorLevel.value,
      logoRatio: Number(el.logoRatio.value) / 100,
      logoPadding: el.logoPadding.checked,
    };
  }

  function applyStyle(s) {
    el.fg.value = el.fgHex.value = s.fgColor;
    el.bg.value = el.bgHex.value = s.bgColor;
    el.transparent.checked = s.transparent;
    el.moduleStyle.value = s.moduleStyle;
    el.eyeStyle.value = s.eyeStyle;
    el.boxSize.value = s.boxSize;
    el.border.value = s.border;
    el.errorLevel.value = s.errorLevel;
    el.logoRatio.value = Math.round(s.logoRatio * 100);
    el.logoPadding.checked = s.logoPadding;
  }

  function updateLabels() {
    el.logoRatioValue.textContent = `${el.logoRatio.value}%`;
    el.boxSizeValue.textContent = `${el.boxSize.value} px`;
    el.borderValue.textContent = `${el.border.value} ${el.border.value === '1' ? 'modulo' : 'moduli'}`;
    el.bg.disabled = el.bgHex.disabled = el.transparent.checked;
    // Con il logo la correzione errori è forzata al massimo
    el.errorLevel.disabled = !!logo;
    el.errorNote.hidden = !logo;
    el.logoClear.disabled = !logo;
    el.logoName.textContent = logo ? logo.name : 'Nessun logo';
    el.logoName.classList.toggle('muted', !logo);
  }

  function snapshot() {
    return JSON.stringify({
      name: el.qrName.value.trim(),
      target: el.qrTarget.value.trim(),
      active: el.qrActive.checked,
      style: readStyle(),
    });
  }

  function updateDirty() {
    const dirty = !!qr && (logoDirty || snapshot() !== saved);
    el.dirty.hidden = !dirty;
    el.qrSave.disabled = !dirty;
  }

  function schedule() {
    updateLabels();
    updateDirty();
    clearTimeout(timer);
    timer = setTimeout(renderPreview, 120);
  }

  function placeholder(text, isError) {
    current = null;
    el.preview.replaceChildren(h('p', { class: 'placeholder' + (isError ? ' error' : '') }, text));
    el.info.textContent = '';
    el.download.disabled = true;
  }

  function isStatic() {
    return !!qr && qr.kind === 'static';
  }

  // Cosa c'è scritto nel QR: per i dinamici sempre e solo il link corto (la
  // destinazione non c'entra), per gli statici il contenuto così com'è nel campo
  function qrData() {
    return isStatic() ? el.qrTarget.value.trim() : qr.short_url;
  }

  function renderPreview() {
    if (!qr) return;
    const style = readStyle();
    const data = qrData();
    el.preview.classList.toggle('transparent', style.transparent);
    if (!data) {
      placeholder('Inserisci il link o il testo da mettere nel QR');
      return;
    }
    try {
      current = R.render({ ...style, data }, logo && logo.image);
    } catch (e) {
      const message = /overflow/i.test(String(e)) ? 'testo troppo lungo per un QR code' : e.message || e;
      placeholder(`Errore: ${message}`, true);
      return;
    }
    current.setAttribute('role', 'img');
    current.setAttribute('aria-label', `QR code per ${data}`);
    el.preview.replaceChildren(current);
    el.info.textContent = `${current.width} × ${current.height} px`;
    el.download.disabled = false;
  }

  async function setLogoFromFile(file) {
    try {
      const original = await loadImage(URL.createObjectURL(file));
      URL.revokeObjectURL(original.src);
      const w = original.naturalWidth || LOGO_MAX_SIDE;
      const hgt = original.naturalHeight || LOGO_MAX_SIDE;
      const scale = Math.min(1, LOGO_MAX_SIDE / Math.max(w, hgt));
      const canvas = document.createElement('canvas');
      canvas.width = Math.max(1, Math.round(w * scale));
      canvas.height = Math.max(1, Math.round(hgt * scale));
      const ctx = canvas.getContext('2d');
      ctx.imageSmoothingQuality = 'high';
      ctx.drawImage(original, 0, 0, canvas.width, canvas.height);
      const dataUrl = canvas.toDataURL('image/png');
      logo = { image: await loadImage(dataUrl), name: file.name, dataUrl };
      logoDirty = true;
    } catch (e) {
      toast('Impossibile caricare il logo: ' + e.message);
    }
    el.logoInput.value = '';
    schedule();
  }

  function clearLogo() {
    if (!logo) return;
    logo = null;
    logoDirty = true;
    schedule();
  }

  // ---- Dettaglio: caricamento e salvataggio ----------------------------------------

  async function openQR(id) {
    show('qr', listHash.startsWith('#/all') ? 'all' : 'list');
    $('qr-back').href = listHash;
    $('qr-back').textContent = listHash.startsWith('#/all') ? '← Tutti i QR' : '← I miei QR';
    const seq = routeSeq;
    qr = null;
    logo = null;
    logoDirty = false;
    $('delete-confirm').hidden = true;
    $('qr-title').textContent = '';
    $('qr-short').textContent = '';
    placeholder('Caricamento…');
    updateDirty();
    try {
      const data = await api('GET', `/qrcodes/${id}`);
      let loadedLogo = null;
      if (data.has_logo) {
        const img = await loadImage(`/api/qrcodes/${id}/logo?v=${encodeURIComponent(data.updated_at)}`);
        loadedLogo = { image: img, name: data.logo_name || 'logo.png', dataUrl: null };
      }
      if (seq !== routeSeq) return;
      qr = data;
      logo = loadedLogo;
      fillDetail();
      loadStats();
    } catch (e) {
      if (seq !== routeSeq) return;
      if (e.status === 404) {
        toast('QR non trovato');
        location.hash = '#/';
      } else {
        placeholder(`Errore: ${e.message}`, true);
      }
    }
  }

  function fillDetail() {
    $('qr-title').textContent = qr.name || 'Senza nome';
    const st = isStatic();
    $('qr-kind').textContent = st ? 'Statico' : 'Dinamico';
    $('qr-kind').className = `badge kind ${st ? 'static' : 'dynamic'}`;
    $('qr-badge').hidden = st || qr.active;
    $('qr-short').textContent = qr.short_url;
    $('qr-shortlink').hidden = st;
    $('qr-active-row').hidden = st;
    $('stats-section').hidden = st;
    $('target-legend').textContent = st ? 'Contenuto' : 'Destinazione';
    $('qr-target-label').textContent = st ? 'Link o testo' : 'Link';
    el.qrTarget.type = st ? 'text' : 'url';
    $('preview-note').textContent = st
      ? 'QR statico: il contenuto è scritto nel QR. Se lo cambi, cambia anche il disegno e il QR va ristampato.'
      : 'Il disegno dipende solo dal link corto e dallo stile: cambiando la destinazione resta identico.';
    $('delete-text').textContent = st
      ? 'Eliminare questo QR dall\'elenco? I QR statici già stampati continueranno a funzionare, perché non passano dal servizio.'
      : 'Eliminare questo QR? Chi lo scansiona vedrà "QR code non trovato". Il link corto non verrà mai riassegnato.';
    // QR di un altro utente (vista dell'admin): si guarda e si scarica, non si modifica
    const readonly = !qr.editable;
    const controls = document.querySelector('#view-qr .controls');
    controls.classList.toggle('readonly', readonly);
    for (const fs of controls.querySelectorAll('fieldset')) fs.disabled = readonly;
    $('qr-save').closest('.savebar').hidden = readonly;
    $('delete-confirm').hidden = true;
    const note = $('readonly-note');
    note.hidden = !readonly;
    note.textContent = readonly
      ? `QR di ${qr.owner.username}: puoi vedere ${st ? 'il QR' : 'statistiche e storico'} e scaricarlo, ma solo ${qr.owner.username} può modificarlo.`
      : '';

    el.qrName.value = qr.name;
    el.qrTarget.value = qr.target_url;
    el.qrActive.checked = qr.active;
    applyStyle(sanitizeStyle(qr.style));
    updateLabels();
    saved = snapshot();
    logoDirty = false;
    updateDirty();
    renderPreview();
  }

  async function saveQR() {
    if (!qr) return;
    const style = readStyle();
    const body = {
      name: el.qrName.value,
      target_url: el.qrTarget.value,
      active: el.qrActive.checked,
      style,
    };
    if (logoDirty) {
      body.logo = logo ? logo.dataUrl : '';
      body.logo_name = logo ? logo.name : '';
    }
    el.qrSave.disabled = true;
    try {
      const targetChanged = el.qrTarget.value.trim() !== qr.target_url;
      qr = await api('PATCH', `/qrcodes/${qr.id}`, body);
      if (logo) logo.dataUrl = null;
      fillDetail();
      storageSet(LAST_STYLE_KEY, JSON.stringify(style));
      if (isStatic()) {
        toast(targetChanged ? 'Salvato: il contenuto è cambiato, riscarica e ristampa il QR' : 'Modifiche salvate');
      } else {
        toast(targetChanged ? 'Salvato: il QR ora porta alla nuova destinazione' : 'Modifiche salvate');
        if (targetChanged) loadStats();
      }
    } catch (e) {
      toast('Salvataggio non riuscito: ' + e.message);
      updateDirty();
    }
  }

  async function deleteQR() {
    if (!qr) return;
    try {
      await api('DELETE', `/qrcodes/${qr.id}`);
      toast('QR eliminato');
      location.hash = '#/';
    } catch (e) {
      toast('Eliminazione non riuscita: ' + e.message);
    }
  }

  function filename() {
    const slug = (qr.name || '')
      .toLowerCase()
      .normalize('NFD')
      .replace(/[̀-ͯ]/g, '')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 40);
    if (isStatic()) return slug ? `qr_${slug}.png` : R.defaultFilename(qrData());
    return `qr_${slug ? slug + '_' : ''}${qr.code}.png`;
  }

  function download() {
    if (!current || !qr) return;
    current.toBlob((blob) => {
      const url = URL.createObjectURL(blob);
      const a = h('a', { href: url, download: filename() });
      document.body.append(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 2000);
      toast('QR code scaricato');
    }, 'image/png');
  }

  async function copyShortLink() {
    if (!qr) return;
    const text = qr.short_url;
    try {
      await navigator.clipboard.writeText(text);
    } catch (e) {
      // Su http non in localhost l'API degli appunti non c'è: si seleziona il testo
      const range = document.createRange();
      range.selectNodeContents($('qr-short'));
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
      toast('Link selezionato: copialo con Ctrl+C');
      return;
    }
    toast('Link copiato');
  }

  // ---- Dettaglio: statistiche --------------------------------------------------------

  async function loadStats() {
    if (!qr || isStatic()) return;
    const id = qr.id;
    try {
      const s = await api('GET', `/qrcodes/${id}/stats?days=${$('stats-days').value}`);
      if (qr && qr.id === id) renderStats(s);
    } catch (e) {
      toast('Statistiche non disponibili: ' + e.message);
    }
  }

  function renderStats(s) {
    $('kpi-total').textContent = num.format(s.total);
    $('kpi-period').textContent = num.format(s.period);
    $('kpi-last').textContent = formatDateTime(s.last_scan_at);
    $('kpi-bots').textContent = num.format(s.bots_ignored);
    renderChart(s.daily);
    renderBreakdown($('bd-devices'), s.devices, s.period);
    renderBreakdown($('bd-os'), s.os, s.period);
    renderBreakdown($('bd-browsers'), s.browsers, s.period);

    const history = $('history');
    if (!s.history.length) {
      history.replaceChildren(h('li', { class: 'muted' }, 'La destinazione non è mai stata cambiata.'));
    } else {
      history.replaceChildren(...s.history.map((x) => h('li', null,
        h('time', { datetime: x.changed_at }, formatDateTime(x.changed_at)),
        ` · ${x.changed_by || 'utente eliminato'}: `,
        h('span', { class: 'muted' }, x.old_url), ' → ', x.new_url)));
    }
  }

  function renderChart(daily) {
    const W = 600;
    const H = 160;
    const max = Math.max(1, ...daily.map((d) => d.count));
    const bw = W / daily.length;
    const gap = Math.min(2, bw * 0.15);
    const bars = daily.map((d, i) => {
      const bh = d.count ? Math.max(2, (d.count / max) * (H - 4)) : 0;
      return svg('rect', { class: 'bar', x: i * bw + gap, y: H - bh, width: Math.max(1, bw - 2 * gap), height: bh },
        svg('title', null, `${formatDay(d.date)}: ${plural(d.count, 'scansione', 'scansioni')}`));
    });
    const chart = svg('svg', {
      viewBox: `0 0 ${W} ${H}`,
      preserveAspectRatio: 'none',
      role: 'img',
      'aria-label': `Scansioni per giorno, massimo ${max}`,
    }, svg('line', { class: 'axis', x1: 0, y1: H, x2: W, y2: H }), ...bars);

    const first = daily[0];
    const last = daily[daily.length - 1];
    const ticks = h('div', { class: 'chart-ticks' },
      h('span', null, formatDay(first.date)),
      h('span', null, `max ${num.format(max)} al giorno`),
      h('span', null, formatDay(last.date)));
    $('chart').replaceChildren(chart, ticks);
  }

  function renderBreakdown(node, rows, total) {
    if (!rows.length) {
      node.replaceChildren(h('p', { class: 'muted note' }, 'Nessuna scansione nel periodo'));
      return;
    }
    node.replaceChildren(...rows.map((r) => {
      const fill = h('div', { class: 'bd-fill' });
      fill.style.width = `${Math.round((r.count / Math.max(1, total)) * 100)}%`;
      return h('div', { class: 'bd-row' },
        h('span', null, r.label),
        h('span', null, num.format(r.count)),
        h('div', { class: 'bd-track' }, fill));
    }));
  }

  // ---- Utenti (admin) -------------------------------------------------------------------

  let resetUser = null;

  async function openUsers() {
    show('users');
    const seq = routeSeq;
    $('reset-form').hidden = true;
    const wrap = $('users-list');
    wrap.replaceChildren(h('p', { class: 'muted placeholder' }, 'Caricamento…'));
    try {
      const users = await api('GET', '/users');
      if (seq !== routeSeq) return;
      wrap.replaceChildren(h('table', null,
        h('thead', null, h('tr', null,
          h('th', null, 'Utente'), h('th', null, 'Ruolo'), h('th', null, 'Stato'),
          h('th', null, 'QR'), h('th', null, 'Scansioni'), h('th', null, 'Creato il'), h('th', null, ''))),
        h('tbody', null, users.map(userRow))));
    } catch (e) {
      if (seq === routeSeq) wrap.replaceChildren(h('p', { class: 'form-error placeholder' }, e.message));
    }
  }

  function userRow(u) {
    const self = u.id === me.id;
    const btn = (label, onclick, disabled) =>
      h('button', { class: 'btn small', type: 'button', disabled: !!disabled, onclick }, label);
    return h('tr', null,
      h('td', null, u.username, self ? h('span', { class: 'muted' }, ' (tu)') : null),
      h('td', null, u.is_admin ? 'Amministratore' : 'Utente'),
      h('td', null, h('span', { class: u.disabled ? 'badge off' : 'badge' }, u.disabled ? 'Disattivato' : 'Attivo')),
      h('td', null, num.format(u.qr_count || 0)),
      h('td', null, num.format(u.scan_count || 0)),
      h('td', null, new Date(u.created_at).toLocaleDateString('it-IT')),
      h('td', { class: 'actions-cell' },
        btn('Vedi QR', () => { location.hash = `#/all/${u.id}`; }, !u.qr_count),
        btn('Password', () => openReset(u)),
        btn(u.is_admin ? 'Togli admin' : 'Rendi admin', () => patchUser(u, { is_admin: !u.is_admin }), self),
        btn(u.disabled ? 'Riattiva' : 'Disattiva', () => patchUser(u, { disabled: !u.disabled }), self)));
  }

  async function patchUser(u, body) {
    try {
      await api('PATCH', `/users/${u.id}`, body);
      toast(`Utente ${u.username} aggiornato`);
      openUsers();
    } catch (e) {
      toast(e.message);
    }
  }

  function openReset(u) {
    resetUser = u;
    $('reset-title').textContent = `Nuova password per ${u.username}`;
    $('reset-form').reset();
    showError($('reset-error'), '');
    $('reset-form').hidden = false;
    $('reset-password').focus();
  }

  async function submitReset(event) {
    event.preventDefault();
    try {
      await api('PATCH', `/users/${resetUser.id}`, { password: $('reset-password').value });
      $('reset-form').hidden = true;
      toast(`Password di ${resetUser.username} reimpostata`);
    } catch (e) {
      showError($('reset-error'), e.message);
    }
  }

  async function createUser(event) {
    event.preventDefault();
    const err = $('user-error');
    showError(err, '');
    try {
      const u = await api('POST', '/users', {
        username: $('user-username').value.trim(),
        password: $('user-password').value,
        is_admin: $('user-admin').checked,
      });
      $('user-form').reset();
      toast(`Utente ${u.username} creato`);
      openUsers();
    } catch (e) {
      showError(err, e.message);
    }
  }

  // ---- Account ----------------------------------------------------------------------------

  function openAccount() {
    show('account');
    $('account-name').textContent = me.username;
    $('password-form').reset();
    showError($('pw-error'), '');
  }

  async function changePassword(event) {
    event.preventDefault();
    const err = $('pw-error');
    showError(err, '');
    if ($('pw-new').value !== $('pw-confirm').value) {
      showError(err, 'Le due password nuove non coincidono');
      return;
    }
    try {
      await api('PUT', '/me/password', {
        current_password: $('pw-current').value,
        new_password: $('pw-new').value,
      });
      $('password-form').reset();
      toast('Password cambiata');
    } catch (e) {
      showError(err, e.message);
    }
  }

  // ---- Avvio ------------------------------------------------------------------------------

  function fillSelect(select, values) {
    for (const value of values) select.add(new Option(value, value));
  }

  function bindColor(picker, hex) {
    picker.addEventListener('input', () => {
      hex.value = picker.value;
      schedule();
    });
    hex.addEventListener('input', () => {
      const v = hex.value.trim();
      const normalized = (v.startsWith('#') ? v : '#' + v).toLowerCase();
      if (/^#[0-9a-f]{6}$/.test(normalized)) {
        picker.value = normalized;
        schedule();
      }
    });
    hex.addEventListener('blur', () => {
      hex.value = picker.value;
    });
  }

  async function init() {
    fillSelect(el.moduleStyle, R.MODULE_STYLES);
    fillSelect(el.eyeStyle, R.EYE_STYLES);
    fillSelect(el.errorLevel, Object.keys(R.ERROR_LEVELS));
    el.logoRatio.max = Math.round(R.MAX_LOGO_RATIO * 100);

    $('login-form').addEventListener('submit', login);
    $('logout').addEventListener('click', logout);
    $('new-toggle').addEventListener('click', () => toggleNewForm($('new-form').hidden));
    $('new-cancel').addEventListener('click', () => toggleNewForm(false));
    $('new-form').addEventListener('submit', createQR);

    bindColor(el.fg, el.fgHex);
    bindColor(el.bg, el.bgHex);
    for (const input of [el.transparent, el.moduleStyle, el.eyeStyle, el.logoRatio,
      el.logoPadding, el.boxSize, el.border, el.errorLevel]) {
      input.addEventListener('input', schedule);
      input.addEventListener('change', schedule);
    }
    for (const input of [el.qrName, el.qrActive]) {
      input.addEventListener('input', updateDirty);
      input.addEventListener('change', updateDirty);
    }
    // Per un QR statico il contenuto è il disegno: l'anteprima segue quello che scrivi
    el.qrTarget.addEventListener('input', () => (isStatic() ? schedule() : updateDirty()));
    $('owner-filter').addEventListener('change', (e) => {
      location.hash = e.target.value ? `#/all/${e.target.value}` : '#/all';
    });
    for (const btn of $('kind-filter').querySelectorAll('button')) {
      btn.addEventListener('click', () => setKindFilter(btn.dataset.kind));
    }
    for (const radio of document.querySelectorAll('input[name="new-kind"]')) {
      radio.addEventListener('change', updateNewKind);
    }
    el.qrTarget.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !el.qrSave.disabled) saveQR();
    });
    el.logoInput.addEventListener('change', () => {
      if (el.logoInput.files[0]) setLogoFromFile(el.logoInput.files[0]);
    });
    el.logoClear.addEventListener('click', clearLogo);
    el.qrSave.addEventListener('click', saveQR);
    el.download.addEventListener('click', download);
    $('qr-copy').addEventListener('click', copyShortLink);
    $('qr-delete').addEventListener('click', () => {
      $('delete-confirm').hidden = false;
    });
    $('delete-cancel').addEventListener('click', () => {
      $('delete-confirm').hidden = true;
    });
    $('delete-ok').addEventListener('click', deleteQR);
    $('stats-days').addEventListener('change', loadStats);

    $('reset-form').addEventListener('submit', submitReset);
    $('reset-cancel').addEventListener('click', () => {
      $('reset-form').hidden = true;
    });
    $('user-form').addEventListener('submit', createUser);
    $('password-form').addEventListener('submit', changePassword);

    window.addEventListener('hashchange', route);
    window.addEventListener('beforeunload', (e) => {
      if (!el.qrSave.disabled && !$('view-qr').hidden) e.preventDefault();
    });

    try {
      me = await api('GET', '/me');
    } catch (e) {
      me = null;
    }
    route();
  }

  init();
})();
