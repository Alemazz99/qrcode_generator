#!/usr/bin/env bash
# Aggiunge "QR Generator" al menu applicazioni di GNOME (solo un file .desktop utente).
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="$HOME/.local/share/applications/qrcode-generator.desktop"

mkdir -p "$(dirname "$TARGET")"
cat > "$TARGET" <<DESKTOP
[Desktop Entry]
Type=Application
Name=QR Generator
Comment=Genera QR code personalizzati
Exec="$DIR/run.sh"
Icon=$DIR/assets/icon.png
Terminal=false
Categories=Graphics;Utility;
StartupWMClass=qrcode-generator
DESKTOP

echo "Launcher installato: $TARGET"
