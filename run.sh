#!/usr/bin/env bash
# Avvia QR Generator. Se esiste il distrobox indicato da QRGEN_CONTAINER (default
# "python") l'app gira lì dentro, così sull'host non si installa nulla; altrimenti
# usa il Python di sistema. QRGEN_CONTAINER= (vuoto) forza l'avvio diretto.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTAINER="${QRGEN_CONTAINER-python}"

in_container() {
    [[ -n "${CONTAINER_ID:-}" || -f /run/.containerenv ]]
}

container_exists() {
    [[ -n "$CONTAINER" ]] && command -v distrobox >/dev/null &&
        distrobox list --no-color 2>/dev/null | awk -F'|' 'NR > 1 { gsub(/ /, "", $2); print $2 }' | grep -qx "$CONTAINER"
}

# Dall'host rientra in questo stesso script dentro il container
if ! in_container && container_exists; then
    exec distrobox enter "$CONTAINER" -- "$DIR/run.sh" "$@"
fi

if [[ ! -x "$DIR/.venv/bin/python" ]]; then
    echo "Prima esecuzione: creo l'ambiente virtuale…"
    python3 -m venv "$DIR/.venv"
    "$DIR/.venv/bin/pip" install -q --upgrade pip
    "$DIR/.venv/bin/pip" install -q -r "$DIR/requirements.txt"
fi

exec "$DIR/.venv/bin/python" "$DIR/app.py" "$@"
