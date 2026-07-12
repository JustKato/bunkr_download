#!/usr/bin/env bash
# Generate platform icon assets from build/appicon.png.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ICON_SRC="build/appicon.png"
if [[ ! -f "$ICON_SRC" ]]; then
    echo "ERROR: missing $ICON_SRC" >&2
    exit 1
fi

mkdir -p build/windows build/darwin

if command -v wails3 >/dev/null 2>&1; then
    echo "==> Generating platform icons ..."
    wails3 generate icons \
        -input "$ICON_SRC" \
        -windowsfilename build/windows/icon.ico \
        -macfilename build/darwin/icons.icns
else
    echo "wails3 not installed; skipping platform icon generation"
    exit 0
fi

if [[ "${1:-}" == "--windows-syso" ]]; then
    ARCH="${GOARCH:-amd64}"
    MANIFEST="build/windows/wails.exe.manifest"
    INFO="build/windows/info.json"
    if [[ ! -f "$MANIFEST" || ! -f "$INFO" ]]; then
        echo "ERROR: missing Windows syso inputs ($MANIFEST, $INFO)" >&2
        exit 1
    fi
    echo "==> Generating Windows syso ..."
    wails3 generate syso \
        -arch "$ARCH" \
        -icon build/windows/icon.ico \
        -manifest "$MANIFEST" \
        -info "$INFO" \
        -out "src/wails_windows_${ARCH}.syso"
fi
