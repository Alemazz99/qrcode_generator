"""QR Generator - interfaccia grafica."""

import dataclasses
import re
import sys
from pathlib import Path
from urllib.parse import urlparse

from PySide6.QtCore import QLoggingCategory, QSettings, QStandardPaths, Qt, QTimer
from PySide6.QtGui import QBrush, QColor, QIcon, QImage, QPainter, QPixmap
from PySide6.QtWidgets import (
    QApplication,
    QCheckBox,
    QColorDialog,
    QComboBox,
    QFileDialog,
    QFormLayout,
    QGroupBox,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QMainWindow,
    QMessageBox,
    QPushButton,
    QSizePolicy,
    QSlider,
    QSpinBox,
    QVBoxLayout,
    QWidget,
)

import qr_core

APP_DIR = Path(__file__).resolve().parent
PREVIEW_SIZE = 420
# Valori predefiniti ripristinabili: arrivano dai default di QrOptions
DEFAULTS = qr_core.QrOptions(data="")


class ColorButton(QPushButton):
    """Pulsante che mostra un colore e apre il selettore al click."""

    def __init__(self, color: str, on_change):
        super().__init__()
        self._on_change = on_change
        self.setFixedWidth(120)
        self.set_color(color)
        self.clicked.connect(self._pick)

    def set_color(self, color: str) -> None:
        self.color = QColor(color).name()
        text = "#000000" if QColor(self.color).lightness() > 140 else "#ffffff"
        self.setText(self.color.upper())
        self.setStyleSheet(
            f"background-color: {self.color}; color: {text}; border: 1px solid #888; "
            "border-radius: 4px; padding: 4px;"
        )

    def _pick(self) -> None:
        color = QColorDialog.getColor(QColor(self.color), self, "Scegli colore")
        if color.isValid():
            self.set_color(color.name())
            self._on_change()


def pil_to_qimage(img) -> QImage:
    data = img.convert("RGBA").tobytes("raw", "RGBA")
    return QImage(data, img.width, img.height, img.width * 4, QImage.Format.Format_RGBA8888).copy()


def checkerboard_brush() -> QBrush:
    tile = QPixmap(20, 20)
    # Grigi medi: si legge sia un QR bianco sia uno nero
    tile.fill(QColor("#a0a0a0"))
    p = QPainter(tile)
    p.fillRect(0, 0, 10, 10, QColor("#7a7a7a"))
    p.fillRect(10, 10, 10, 10, QColor("#7a7a7a"))
    p.end()
    return QBrush(tile)


def default_filename(data: str) -> str:
    host = urlparse(data).netloc or data
    slug = re.sub(r"[^A-Za-z0-9._-]+", "_", host.removeprefix("www.")).strip("_")[:50]
    return f"qr_{slug or 'code'}.png"


class MainWindow(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("QR Generator")
        self.settings = QSettings("Alemazz99", "qrcode_generator")
        self.current_image = None
        self.logo_path: str | None = None

        self._preview_timer = QTimer(self, singleShot=True, interval=250)
        self._preview_timer.timeout.connect(self.update_preview)

        self._build_ui()
        self._load_settings()
        self.update_preview()
        # Su Wayland la finestra non parte dalla dimensione suggerita dal layout
        self.resize(self.sizeHint())

    # ---- UI ----------------------------------------------------------------

    def _build_ui(self) -> None:
        changed = self.schedule_preview

        # Contenuto
        self.data_edit = QLineEdit()
        self.data_edit.setPlaceholderText("https://esempio.it")
        self.data_edit.setClearButtonEnabled(True)
        self.data_edit.textChanged.connect(changed)
        self.data_edit.returnPressed.connect(self.save_qr)
        content_box = QGroupBox("Link o testo")
        QVBoxLayout(content_box).addWidget(self.data_edit)

        # Aspetto
        self.fg_button = ColorButton("#000000", changed)
        self.bg_button = ColorButton("#ffffff", changed)
        self.transparent_check = QCheckBox("Trasparente")
        self.transparent_check.toggled.connect(lambda on: self.bg_button.setDisabled(on))
        self.transparent_check.toggled.connect(changed)
        bg_row = QHBoxLayout()
        bg_row.addWidget(self.bg_button)
        bg_row.addWidget(self.transparent_check)
        bg_row.addStretch()

        self.module_combo = QComboBox()
        self.module_combo.addItems(qr_core.MODULE_STYLES)
        self.module_combo.currentIndexChanged.connect(changed)
        self.eye_combo = QComboBox()
        self.eye_combo.addItems(qr_core.EYE_STYLES)
        self.eye_combo.currentIndexChanged.connect(changed)

        style_form = QFormLayout()
        style_form.addRow("Colore QR", self.fg_button)
        style_form.addRow("Sfondo", bg_row)
        style_form.addRow("Stile moduli", self.module_combo)
        style_form.addRow("Stile angoli", self.eye_combo)
        style_box = QGroupBox("Aspetto")
        style_box.setLayout(style_form)

        # Logo
        self.logo_label = QLabel("Nessun logo")
        self.logo_label.setStyleSheet("color: palette(placeholder-text);")
        logo_pick = QPushButton("Scegli…")
        logo_pick.clicked.connect(self.pick_logo)
        self.logo_clear = QPushButton("Rimuovi")
        self.logo_clear.clicked.connect(lambda: self.set_logo(None))
        logo_row = QHBoxLayout()
        logo_row.addWidget(self.logo_label, 1)
        logo_row.addWidget(logo_pick)
        logo_row.addWidget(self.logo_clear)

        self.logo_ratio = QSlider(Qt.Orientation.Horizontal)
        self.logo_ratio.setRange(10, round(qr_core.MAX_LOGO_RATIO * 100))
        self.logo_ratio.setValue(22)
        self.logo_ratio_label = QLabel("22%")
        self.logo_ratio.valueChanged.connect(lambda v: self.logo_ratio_label.setText(f"{v}%"))
        self.logo_ratio.valueChanged.connect(changed)
        ratio_row = QHBoxLayout()
        ratio_row.addWidget(self.logo_ratio, 1)
        ratio_row.addWidget(self.logo_ratio_label)

        self.logo_padding = QCheckBox("Riquadro di sfondo dietro al logo")
        self.logo_padding.setChecked(True)
        self.logo_padding.toggled.connect(changed)

        logo_form = QFormLayout()
        logo_form.addRow("File", logo_row)
        logo_form.addRow("Dimensione", ratio_row)
        logo_form.addRow("", self.logo_padding)
        logo_box = QGroupBox("Logo centrale")
        logo_box.setLayout(logo_form)

        # Dimensioni e qualità
        self.box_size = QSpinBox(minimum=4, maximum=80, value=30, suffix=" px")
        self.box_size.valueChanged.connect(changed)
        self.border = QSpinBox(minimum=0, maximum=10, value=2, suffix=" moduli")
        self.border.valueChanged.connect(changed)
        self.error_combo = QComboBox()
        self.error_combo.addItems(qr_core.ERROR_LEVELS)
        self.error_combo.setCurrentText("M - 15%")
        self.error_combo.currentIndexChanged.connect(changed)

        size_form = QFormLayout()
        size_form.addRow("Dimensione modulo", self.box_size)
        size_form.addRow("Margine", self.border)
        size_form.addRow("Correzione errori", self.error_combo)
        size_box = QGroupBox("Dimensioni e qualità")
        size_box.setLayout(size_form)

        left = QVBoxLayout()
        for box in (content_box, style_box, logo_box, size_box):
            left.addWidget(box)
        left.addStretch()

        # Anteprima
        self.preview = QLabel(alignment=Qt.AlignmentFlag.AlignCenter)
        self.preview.setFixedSize(PREVIEW_SIZE, PREVIEW_SIZE)
        self.preview.setStyleSheet("border: 1px solid palette(mid); border-radius: 6px;")
        self.info_label = QLabel(alignment=Qt.AlignmentFlag.AlignCenter)
        self.info_label.setStyleSheet("color: palette(placeholder-text);")

        self.save_button = QPushButton("Genera e salva")
        self.save_button.setMinimumHeight(44)
        self.save_button.setStyleSheet("font-weight: bold; font-size: 15px;")
        self.save_button.clicked.connect(self.save_qr)

        self.reset_button = QPushButton("Ripristina predefiniti")
        self.reset_button.setToolTip("Riporta colori, stili, dimensioni e logo ai valori iniziali")
        self.reset_button.clicked.connect(self.reset_defaults)

        right = QVBoxLayout()
        right.addWidget(self.preview)
        right.addWidget(self.info_label)
        right.addSpacing(8)
        right.addWidget(self.save_button)
        right.addWidget(self.reset_button)
        right.addStretch()

        root = QHBoxLayout()
        root.addLayout(left, 1)
        root.addSpacing(12)
        root.addLayout(right)
        central = QWidget()
        central.setLayout(root)
        central.setSizePolicy(QSizePolicy.Policy.Preferred, QSizePolicy.Policy.Preferred)
        self.setCentralWidget(central)

    # ---- Logica ------------------------------------------------------------

    def options(self) -> qr_core.QrOptions:
        return qr_core.QrOptions(
            data=self.data_edit.text().strip(),
            fg_color=self.fg_button.color,
            bg_color=self.bg_button.color,
            transparent=self.transparent_check.isChecked(),
            module_style=self.module_combo.currentText(),
            eye_style=self.eye_combo.currentText(),
            box_size=self.box_size.value(),
            border=self.border.value(),
            error_level=self.error_combo.currentText(),
            logo_path=self.logo_path,
            logo_ratio=self.logo_ratio.value() / 100,
            logo_padding=self.logo_padding.isChecked(),
        )

    def schedule_preview(self) -> None:
        self._preview_timer.start()

    def apply_options(self, opts: qr_core.QrOptions) -> None:
        """Porta i widget ai valori di opts (il testo del link non viene toccato)."""
        self.fg_button.set_color(opts.fg_color)
        self.bg_button.set_color(opts.bg_color)
        self.transparent_check.setChecked(opts.transparent)
        self.module_combo.setCurrentText(opts.module_style)
        self.eye_combo.setCurrentText(opts.eye_style)
        self.box_size.setValue(opts.box_size)
        self.border.setValue(opts.border)
        self.error_combo.setCurrentText(opts.error_level)
        self.logo_ratio.setValue(round(opts.logo_ratio * 100))
        self.logo_padding.setChecked(opts.logo_padding)
        self.set_logo(opts.logo_path)

    def reset_defaults(self) -> None:
        self.apply_options(DEFAULTS)
        self.statusBar().showMessage("Impostazioni predefinite ripristinate", 4000)

    def update_preview(self) -> None:
        opts = self.options()
        self.reset_button.setEnabled(dataclasses.replace(opts, data="") != DEFAULTS)
        self.save_button.setEnabled(bool(opts.data))
        if not opts.data:
            self.current_image = None
            self.preview.setPixmap(QPixmap())
            self.preview.setText("Inserisci un link per vedere l'anteprima")
            self.info_label.clear()
            return

        try:
            self.current_image = qr_core.generate(opts)
        except Exception as exc:  # es. logo non leggibile o testo troppo lungo
            self.current_image = None
            self.preview.setPixmap(QPixmap())
            self.preview.setText(f"Errore: {exc}")
            self.info_label.clear()
            return

        canvas = QPixmap(PREVIEW_SIZE - 2, PREVIEW_SIZE - 2)
        canvas.fill(Qt.GlobalColor.transparent)
        painter = QPainter(canvas)
        painter.setRenderHint(QPainter.RenderHint.SmoothPixmapTransform)
        if opts.transparent:
            painter.fillRect(canvas.rect(), checkerboard_brush())
        painter.drawImage(canvas.rect(), pil_to_qimage(self.current_image))
        painter.end()
        self.preview.setPixmap(canvas)

        w, h = self.current_image.size
        self.info_label.setText(f"{w} × {h} px")

    def pick_logo(self) -> None:
        start = self.settings.value("logo_dir", str(Path.home()))
        path, _ = QFileDialog.getOpenFileName(
            self, "Scegli logo", start, "Immagini (*.png *.jpg *.jpeg *.webp *.bmp *.gif)"
        )
        if path:
            self.settings.setValue("logo_dir", str(Path(path).parent))
            self.set_logo(path)

    def set_logo(self, path: str | None) -> None:
        self.logo_path = path if path and Path(path).is_file() else None
        self.logo_label.setText(Path(self.logo_path).name if self.logo_path else "Nessun logo")
        self.logo_label.setToolTip(self.logo_path or "")
        self.logo_clear.setEnabled(bool(self.logo_path))
        # Con il logo la correzione errori è forzata al massimo
        self.error_combo.setDisabled(bool(self.logo_path))
        self.error_combo.setToolTip("Con il logo viene usato H - 30%" if self.logo_path else "")
        self.schedule_preview()

    def save_qr(self) -> None:
        self._preview_timer.stop()
        self.update_preview()
        if self.current_image is None:
            return

        pictures = QStandardPaths.writableLocation(QStandardPaths.StandardLocation.PicturesLocation)
        out_dir = self.settings.value("output_dir", pictures or str(Path.home()))
        suggested = str(Path(out_dir) / default_filename(self.data_edit.text().strip()))
        path, _ = QFileDialog.getSaveFileName(
            self, "Salva QR code", suggested, "PNG (*.png);;JPEG (*.jpg *.jpeg);;WebP (*.webp)"
        )
        if not path:
            return
        if not Path(path).suffix:
            path += ".png"

        try:
            saved = qr_core.save(self.current_image, path, self.bg_button.color)
        except Exception as exc:
            QMessageBox.critical(self, "Errore", f"Impossibile salvare il file:\n{exc}")
            return
        self.settings.setValue("output_dir", str(saved.parent))
        self.statusBar().showMessage(f"Salvato: {saved}", 8000)

    # ---- Impostazioni ------------------------------------------------------

    def _load_settings(self) -> None:
        s, d = self.settings, DEFAULTS
        self.apply_options(qr_core.QrOptions(
            data="",
            fg_color=s.value("fg_color", d.fg_color),
            bg_color=s.value("bg_color", d.bg_color),
            transparent=s.value("transparent", d.transparent, type=bool),
            module_style=s.value("module_style", d.module_style),
            eye_style=s.value("eye_style", d.eye_style),
            box_size=s.value("box_size", d.box_size, type=int),
            border=s.value("border", d.border, type=int),
            error_level=s.value("error_level", d.error_level),
            logo_ratio=s.value("logo_ratio", round(d.logo_ratio * 100), type=int) / 100,
            logo_padding=s.value("logo_padding", d.logo_padding, type=bool),
            logo_path=s.value("logo_path", "") or None,
        ))

    def closeEvent(self, event) -> None:
        s = self.settings
        s.setValue("fg_color", self.fg_button.color)
        s.setValue("bg_color", self.bg_button.color)
        s.setValue("transparent", self.transparent_check.isChecked())
        s.setValue("module_style", self.module_combo.currentText())
        s.setValue("eye_style", self.eye_combo.currentText())
        s.setValue("box_size", self.box_size.value())
        s.setValue("border", self.border.value())
        s.setValue("error_level", self.error_combo.currentText())
        s.setValue("logo_ratio", self.logo_ratio.value())
        s.setValue("logo_padding", self.logo_padding.isChecked())
        s.setValue("logo_path", self.logo_path or "")
        super().closeEvent(event)


def main() -> int:
    # Dal distrobox il portale dell'host rifiuta la registrazione dell'app ID
    # (connessione già associata): avviso innocuo, lo nascondiamo
    QLoggingCategory.setFilterRules("qt.qpa.services.warning=false")
    app = QApplication(sys.argv)
    app.setApplicationName("QR Generator")
    app.setDesktopFileName("qrcode-generator")
    icon = APP_DIR / "assets" / "icon.png"
    if icon.is_file():
        app.setWindowIcon(QIcon(str(icon)))
    window = MainWindow()
    window.show()
    return app.exec()


if __name__ == "__main__":
    sys.exit(main())
