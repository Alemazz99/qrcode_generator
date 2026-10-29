// Interfaccia della versione web: legge i controlli, chiama QRRender e salva le preferenze.
(function () {
  'use strict';

  const R = window.QRRender;
  const SETTINGS_KEY = 'qrcode_generator.settings';
  const LOGO_KEY = 'qrcode_generator.logo';
  // Il logo viene ridotto a questo lato massimo: basta per l'output e sta nel localStorage
  const LOGO_MAX_SIDE = 1024;

  const $ = (id) => document.getElementById(id);
  const el = {
    data: $('data'),
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
    preview: $('preview'),
    info: $('info'),
    download: $('download'),
    share: $('share'),
    reset: $('reset'),
    status: $('status'),
  };

  let logo = null; // { image, name, dataUrl }
  let current = null; // canvas dell'ultimo QR generato
  let timer = 0;

  // ---- Preferenze (solo comodità: se lo storage non c'è si riparte dai default) ----

  function storageGet(key) {
    try {
      return localStorage.getItem(key);
    } catch (e) {
      return null;
    }
  }

  function storageSet(key, value) {
    try {
      if (value === null) localStorage.removeItem(key);
      else localStorage.setItem(key, value);
    } catch (e) {
      // quota piena o storage bloccato: le preferenze semplicemente non restano
    }
  }

  function loadSettings() {
    let saved = {};
    try {
      saved = JSON.parse(storageGet(SETTINGS_KEY)) || {};
    } catch (e) {
      saved = {};
    }
    const s = { ...R.DEFAULTS };
    for (const key of Object.keys(R.DEFAULTS)) {
      if (typeof saved[key] === typeof R.DEFAULTS[key]) s[key] = saved[key];
    }
    if (!R.MODULE_STYLES.includes(s.moduleStyle)) s.moduleStyle = R.DEFAULTS.moduleStyle;
    if (!R.EYE_STYLES.includes(s.eyeStyle)) s.eyeStyle = R.DEFAULTS.eyeStyle;
    if (!(s.errorLevel in R.ERROR_LEVELS)) s.errorLevel = R.DEFAULTS.errorLevel;
    return s;
  }

  // ---- Controlli <-> impostazioni ----

  function readSettings() {
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

  function applySettings(s) {
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
    updateLabels();
  }

  function isDefault(s) {
    return !logo && Object.keys(R.DEFAULTS).every((key) => s[key] === R.DEFAULTS[key]);
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

  // ---- Anteprima ----

  function schedule() {
    updateLabels();
    clearTimeout(timer);
    timer = setTimeout(update, 120);
  }

  function showPlaceholder(text, isError) {
    current = null;
    const p = document.createElement('p');
    p.className = 'placeholder' + (isError ? ' error' : '');
    p.textContent = text;
    el.preview.replaceChildren(p);
    el.info.textContent = '';
    el.download.disabled = el.share.disabled = true;
  }

  function update() {
    const settings = readSettings();
    storageSet(SETTINGS_KEY, JSON.stringify(settings));
    el.reset.disabled = isDefault(settings);
    el.preview.classList.toggle('transparent', settings.transparent);

    const data = el.data.value.trim();
    if (!data) {
      showPlaceholder("Inserisci un link per vedere l'anteprima");
      return;
    }
    try {
      current = R.render({ ...settings, data }, logo && logo.image);
    } catch (e) {
      // es. testo troppo lungo per un QR o immagine troppo grande
      const message = /overflow/i.test(String(e)) ? 'testo troppo lungo per un QR code' : e.message || e;
      showPlaceholder(`Errore: ${message}`, true);
      return;
    }
    current.setAttribute('role', 'img');
    current.setAttribute('aria-label', `QR code per ${data}`);
    el.preview.replaceChildren(current);
    el.info.textContent = `${current.width} × ${current.height} px`;
    el.download.disabled = el.share.disabled = false;
  }

  // ---- Logo ----

  function loadImage(src) {
    return new Promise((resolve, reject) => {
      const img = new Image();
      img.onload = () => resolve(img);
      img.onerror = () => reject(new Error('immagine non leggibile'));
      img.src = src;
    });
  }

  async function setLogoFromFile(file) {
    try {
      const original = await loadImage(URL.createObjectURL(file));
      URL.revokeObjectURL(original.src);
      const w = original.naturalWidth || LOGO_MAX_SIDE;
      const h = original.naturalHeight || LOGO_MAX_SIDE;
      const scale = Math.min(1, LOGO_MAX_SIDE / Math.max(w, h));
      const canvas = document.createElement('canvas');
      canvas.width = Math.max(1, Math.round(w * scale));
      canvas.height = Math.max(1, Math.round(h * scale));
      const ctx = canvas.getContext('2d');
      ctx.imageSmoothingQuality = 'high';
      ctx.drawImage(original, 0, 0, canvas.width, canvas.height);
      const dataUrl = canvas.toDataURL('image/png');
      logo = { image: await loadImage(dataUrl), name: file.name, dataUrl };
      storageSet(LOGO_KEY, JSON.stringify({ name: file.name, dataUrl }));
    } catch (e) {
      toast('Impossibile caricare il logo: ' + e.message);
    }
    schedule();
  }

  async function restoreLogo() {
    try {
      const saved = JSON.parse(storageGet(LOGO_KEY));
      if (saved && saved.dataUrl) {
        logo = { image: await loadImage(saved.dataUrl), name: saved.name, dataUrl: saved.dataUrl };
      }
    } catch (e) {
      logo = null;
    }
  }

  function clearLogo() {
    logo = null;
    el.logoInput.value = '';
    storageSet(LOGO_KEY, null);
    schedule();
  }

  // ---- Esportazione ----

  function currentBlob() {
    return new Promise((resolve) => current.toBlob(resolve, 'image/png'));
  }

  async function download() {
    if (!current) return;
    const blob = await currentBlob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = R.defaultFilename(el.data.value.trim());
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 2000);
    toast('QR code scaricato');
  }

  async function share() {
    if (!current) return;
    const blob = await currentBlob();
    const file = new File([blob], R.defaultFilename(el.data.value.trim()), { type: 'image/png' });
    try {
      await navigator.share({ files: [file] });
    } catch (e) {
      if (e.name !== 'AbortError') toast('Condivisione non riuscita');
    }
  }

  function canShareFiles() {
    try {
      const probe = new File([''], 'qr.png', { type: 'image/png' });
      return !!(navigator.canShare && navigator.canShare({ files: [probe] }));
    } catch (e) {
      return false;
    }
  }

  let toastTimer = 0;
  function toast(text) {
    el.status.textContent = text;
    el.status.classList.add('visible');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.status.classList.remove('visible'), 2500);
  }

  // ---- Avvio ----

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

    applySettings(loadSettings());
    await restoreLogo();

    bindColor(el.fg, el.fgHex);
    bindColor(el.bg, el.bgHex);
    for (const input of [el.data, el.transparent, el.moduleStyle, el.eyeStyle, el.logoRatio,
      el.logoPadding, el.boxSize, el.border, el.errorLevel]) {
      input.addEventListener('input', schedule);
      input.addEventListener('change', schedule);
    }
    // Invio chiude la tastiera sul telefono
    el.data.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') el.data.blur();
    });
    el.logoInput.addEventListener('change', () => {
      if (el.logoInput.files[0]) setLogoFromFile(el.logoInput.files[0]);
    });
    el.logoClear.addEventListener('click', clearLogo);
    el.download.addEventListener('click', download);
    el.share.hidden = !canShareFiles();
    el.share.addEventListener('click', share);
    el.reset.addEventListener('click', () => {
      applySettings(R.DEFAULTS);
      clearLogo();
      toast('Impostazioni predefinite ripristinate');
    });

    updateLabels();
    update();

    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.register('sw.js').catch(() => {});
    }
  }

  init();
})();
