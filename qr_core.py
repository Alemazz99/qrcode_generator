"""Logica di generazione dei QR code, indipendente dalla UI."""

from dataclasses import dataclass
from pathlib import Path

import qrcode
from PIL import Image, ImageDraw, ImageOps
from qrcode.image.styledpil import StyledPilImage
from qrcode.image.styles.moduledrawers.pil import (
    CircleModuleDrawer,
    GappedSquareModuleDrawer,
    HorizontalBarsDrawer,
    RoundedModuleDrawer,
    SquareModuleDrawer,
    VerticalBarsDrawer,
)

MODULE_STYLES = {
    "Arrotondato": RoundedModuleDrawer,
    "Quadrato": SquareModuleDrawer,
    "Quadrato distanziato": GappedSquareModuleDrawer,
    "Cerchi": CircleModuleDrawer,
    "Barre verticali": VerticalBarsDrawer,
    "Barre orizzontali": HorizontalBarsDrawer,
}

# Solo stili "pieni" per gli angoli: con cerchi o quadrati staccati i lettori
# non riconoscono più i finder pattern e il QR diventa illeggibile
EYE_STYLES = {
    "Arrotondato": RoundedModuleDrawer,
    "Quadrato": SquareModuleDrawer,
}

# Oltre questa quota del lato i telefoni smettono di leggere il QR (provato sul campo)
MAX_LOGO_RATIO = 0.24

ERROR_LEVELS = {
    "L - 7%": qrcode.constants.ERROR_CORRECT_L,
    "M - 15%": qrcode.constants.ERROR_CORRECT_M,
    "Q - 25%": qrcode.constants.ERROR_CORRECT_Q,
    "H - 30%": qrcode.constants.ERROR_CORRECT_H,
}


@dataclass
class QrOptions:
    data: str
    fg_color: str = "#000000"
    bg_color: str = "#ffffff"
    transparent: bool = False
    module_style: str = "Quadrato"
    eye_style: str = "Quadrato"
    box_size: int = 30
    border: int = 2
    error_level: str = "M - 15%"
    logo_path: str | None = None
    logo_ratio: float = 0.22
    # Svuota un riquadro dietro al logo così i moduli non ci passano sotto
    logo_padding: bool = True


def hex_to_rgba(value: str, alpha: int = 255) -> tuple[int, int, int, int]:
    value = value.lstrip("#")
    return int(value[0:2], 16), int(value[2:4], 16), int(value[4:6], 16), alpha


def generate(opts: QrOptions) -> Image.Image:
    """Genera il QR code come immagine RGBA."""
    # Con un logo al centro serve la correzione massima per restare leggibile
    error = qrcode.constants.ERROR_CORRECT_H if opts.logo_path else ERROR_LEVELS[opts.error_level]
    qr = qrcode.QRCode(error_correction=error, box_size=opts.box_size, border=opts.border)
    qr.add_data(opts.data)
    qr.make(fit=True)

    # Disegna la forma in nero su bianco, poi la usa come maschera per i colori:
    # molto più veloce delle color mask di qrcode, che lavorano pixel per pixel
    shape = qr.make_image(
        image_factory=StyledPilImage,
        module_drawer=MODULE_STYLES[opts.module_style](),
        eye_drawer=EYE_STYLES[opts.eye_style](),
    ).get_image().convert("L")
    mask = ImageOps.invert(shape)

    fg = hex_to_rgba(opts.fg_color)
    if opts.transparent:
        img = Image.new("RGBA", shape.size, fg[:3] + (0,))
        img.putalpha(mask)
    else:
        img = Image.new("RGBA", shape.size, hex_to_rgba(opts.bg_color))
        img.paste(Image.new("RGBA", shape.size, fg), mask=mask)

    if opts.logo_path:
        bg = (0, 0, 0, 0) if opts.transparent else hex_to_rgba(opts.bg_color)
        _add_logo(img, opts.logo_path, opts.logo_ratio, opts.logo_padding, bg, opts.box_size)
    return img


def _add_logo(img: Image.Image, path: str, ratio: float, padding: bool, pad_color, cell: int) -> None:
    logo = ImageOps.exif_transpose(Image.open(path)).convert("RGBA")
    side = int(img.width * min(ratio, MAX_LOGO_RATIO))
    scale = side / max(logo.size)
    logo = logo.resize(
        (max(1, round(logo.width * scale)), max(1, round(logo.height * scale))),
        Image.Resampling.LANCZOS,
    )
    x = (img.width - logo.width) // 2
    y = (img.height - logo.height) // 2

    if padding:
        # Il riquadro è allineato alla griglia: niente moduli tagliati a metà
        pad = max(4, side // 12)
        x0 = (x - pad) // cell * cell
        y0 = (y - pad) // cell * cell
        x1 = -(-(x + logo.width + pad) // cell) * cell
        y1 = -(-(y + logo.height + pad) // cell) * cell
        ImageDraw.Draw(img).rectangle((x0, y0, x1 - 1, y1 - 1), fill=pad_color)
    img.alpha_composite(logo, (x, y))


def save(img: Image.Image, path: str | Path, bg_color: str = "#ffffff") -> Path:
    """Salva l'immagine; per i formati senza trasparenza appiattisce sul colore di sfondo."""
    path = Path(path)
    if path.suffix.lower() in (".jpg", ".jpeg"):
        flat = Image.new("RGB", img.size, hex_to_rgba(bg_color)[:3])
        flat.paste(img, mask=img.getchannel("A"))
        flat.save(path, quality=95)
    else:
        img.save(path)
    return path
