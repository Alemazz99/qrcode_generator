// Generazione del QR code nel browser: stessa logica e stessi limiti di qr_core.py,
// disegnata su un <canvas>. Non tocca il DOM della pagina.
(function (global) {
  'use strict';

  const MODULE_STYLES = [
    'Arrotondato',
    'Quadrato',
    'Quadrato distanziato',
    'Cerchi',
    'Barre verticali',
    'Barre orizzontali',
  ];

  // Solo stili "pieni" per gli angoli: con cerchi o quadrati staccati i lettori
  // non riconoscono più i finder pattern e il QR diventa illeggibile
  const EYE_STYLES = ['Arrotondato', 'Quadrato'];

  const ERROR_LEVELS = {
    'L - 7%': 'L',
    'M - 15%': 'M',
    'Q - 25%': 'Q',
    'H - 30%': 'H',
  };

  // Oltre questa quota del lato i telefoni smettono di leggere il QR (provato sul campo)
  const MAX_LOGO_RATIO = 0.24;

  // Limite dei canvas su iOS: oltre ~16,7 milioni di pixel il disegno fallisce
  const MAX_SIDE = 4096;

  const DEFAULTS = Object.freeze({
    fgColor: '#000000',
    bgColor: '#ffffff',
    transparent: false,
    moduleStyle: 'Quadrato',
    eyeStyle: 'Quadrato',
    boxSize: 30,
    border: 2,
    errorLevel: 'M - 15%',
    logoRatio: 0.22,
    // Svuota un riquadro dietro al logo così i moduli non ci passano sotto
    logoPadding: true,
  });

  global.qrcode.stringToBytes = global.qrcode.stringToBytesFuncs['UTF-8'];

  function buildMatrix(data, level) {
    const qr = global.qrcode(0, level);
    qr.addData(data);
    qr.make();
    const n = qr.getModuleCount();
    return {
      n,
      dark: (r, c) => r >= 0 && c >= 0 && r < n && c < n && qr.isDark(r, c),
    };
  }

  // Ogni drawer aggiunge al path la forma di un modulo attivo nel riquadro (x, y, b).
  // Tutti i moduli finiscono in un unico path riempito una volta sola: niente
  // righine di antialiasing tra moduli adiacenti.
  const DRAWERS = {
    'Quadrato': (p, x, y, b) => p.rect(x, y, b, b),

    'Quadrato distanziato': (p, x, y, b) => {
      const d = 0.1 * b;
      p.rect(x + d, y + d, b - 2 * d, b - 2 * d);
    },

    'Cerchi': (p, x, y, b) => {
      p.moveTo(x + b, y + b / 2);
      p.arc(x + b / 2, y + b / 2, b / 2, 0, 2 * Math.PI);
    },

    // Ogni quarto del modulo è arrotondato se i due vicini su quel lato sono vuoti
    'Arrotondato': (p, x, y, b, n) => {
      const h = b / 2;
      const cx = x + h;
      const cy = y + h;
      const quarters = [
        [!n.W && !n.N, x, y, Math.PI],
        [!n.N && !n.E, cx, y, 1.5 * Math.PI],
        [!n.E && !n.S, cx, cy, 0],
        [!n.S && !n.W, x, cy, 0.5 * Math.PI],
      ];
      for (const [round, qx, qy, angle] of quarters) {
        if (round) {
          p.moveTo(cx, cy);
          p.arc(cx, cy, h, angle, angle + Math.PI / 2);
          p.closePath();
        } else {
          p.rect(qx, qy, h, h);
        }
      }
    },

    'Barre verticali': (p, x, y, b, n) => {
      const w = 0.8 * b;
      const h = b / 2;
      const dx = x + (b - w) / 2;
      if (n.N) {
        p.rect(dx, y, w, h);
      } else {
        p.moveTo(dx, y + h);
        p.ellipse(dx + w / 2, y + h, w / 2, h, 0, Math.PI, 2 * Math.PI);
        p.closePath();
      }
      if (n.S) {
        p.rect(dx, y + h, w, h);
      } else {
        p.moveTo(dx + w, y + h);
        p.ellipse(dx + w / 2, y + h, w / 2, h, 0, 0, Math.PI);
        p.closePath();
      }
    },

    'Barre orizzontali': (p, x, y, b, n) => {
      const hh = 0.8 * b;
      const w = b / 2;
      const dy = y + (b - hh) / 2;
      if (n.W) {
        p.rect(x, dy, w, hh);
      } else {
        p.moveTo(x + w, dy + hh);
        p.ellipse(x + w, dy + hh / 2, w, hh / 2, 0, 0.5 * Math.PI, 1.5 * Math.PI);
        p.closePath();
      }
      if (n.E) {
        p.rect(x + w, dy, w, hh);
      } else {
        p.moveTo(x + w, dy);
        p.ellipse(x + w, dy + hh / 2, w, hh / 2, 0, -0.5 * Math.PI, 0.5 * Math.PI);
        p.closePath();
      }
    },
  };

  /**
   * Genera il QR code e lo restituisce come <canvas>.
   * @param {object} opts  DEFAULTS più `data` (il testo da codificare)
   * @param {CanvasImageSource|null} logo  immagine già caricata, oppure null
   */
  function render(opts, logo) {
    // Con un logo al centro serve la correzione massima per restare leggibile
    const level = logo ? 'H' : ERROR_LEVELS[opts.errorLevel];
    const m = buildMatrix(opts.data, level);
    const b = opts.boxSize;
    const offset = opts.border * b;
    const side = (m.n + 2 * opts.border) * b;
    if (side > MAX_SIDE) {
      throw new Error(`immagine troppo grande (${side} px), riduci la dimensione del modulo`);
    }

    const canvas = document.createElement('canvas');
    canvas.width = canvas.height = side;
    const ctx = canvas.getContext('2d');
    if (!opts.transparent) {
      ctx.fillStyle = opts.bgColor;
      ctx.fillRect(0, 0, side, side);
    }

    const isEye = (r, c) =>
      (r < 7 && c < 7) || (r < 7 && c >= m.n - 7) || (r >= m.n - 7 && c < 7);
    const path = new Path2D();
    for (let r = 0; r < m.n; r++) {
      for (let c = 0; c < m.n; c++) {
        if (!m.dark(r, c)) continue;
        const style = isEye(r, c) ? opts.eyeStyle : opts.moduleStyle;
        const neighbors = {
          N: m.dark(r - 1, c),
          E: m.dark(r, c + 1),
          S: m.dark(r + 1, c),
          W: m.dark(r, c - 1),
        };
        DRAWERS[style](path, offset + c * b, offset + r * b, b, neighbors);
      }
    }
    ctx.fillStyle = opts.fgColor;
    ctx.fill(path);

    if (logo) drawLogo(ctx, side, logo, opts, b);
    return canvas;
  }

  function drawLogo(ctx, side, logo, opts, cell) {
    const size = Math.floor(side * Math.min(opts.logoRatio, MAX_LOGO_RATIO));
    const lw = logo.naturalWidth || logo.width;
    const lh = logo.naturalHeight || logo.height;
    const scale = size / Math.max(lw, lh);
    const w = Math.max(1, Math.round(lw * scale));
    const h = Math.max(1, Math.round(lh * scale));
    const x = Math.floor((side - w) / 2);
    const y = Math.floor((side - h) / 2);

    if (opts.logoPadding) {
      // Il riquadro è allineato alla griglia: niente moduli tagliati a metà
      const pad = Math.max(4, Math.floor(size / 12));
      const x0 = Math.floor((x - pad) / cell) * cell;
      const y0 = Math.floor((y - pad) / cell) * cell;
      const x1 = Math.ceil((x + w + pad) / cell) * cell;
      const y1 = Math.ceil((y + h + pad) / cell) * cell;
      if (opts.transparent) {
        ctx.clearRect(x0, y0, x1 - x0, y1 - y0);
      } else {
        ctx.fillStyle = opts.bgColor;
        ctx.fillRect(x0, y0, x1 - x0, y1 - y0);
      }
    }
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(logo, x, y, w, h);
  }

  function defaultFilename(data) {
    let host = '';
    try {
      host = new URL(data).host;
    } catch (e) {
      // non è un URL: si usa il testo così com'è
    }
    const slug = (host || data)
      .replace(/^www\./, '')
      .replace(/[^A-Za-z0-9._-]+/g, '_')
      .replace(/^_+|_+$/g, '')
      .slice(0, 50);
    return `qr_${slug || 'code'}.png`;
  }

  global.QRRender = {
    MODULE_STYLES,
    EYE_STYLES,
    ERROR_LEVELS,
    MAX_LOGO_RATIO,
    DEFAULTS,
    render,
    defaultFilename,
  };
})(window);
