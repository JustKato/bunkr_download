#!/usr/bin/env bash
# Package a Linux release with launcher metadata and its application icon.
# Usage: package-linux-release.sh <binary-path> <output-tar.gz-path>
set -euo pipefail

BINARY="${1:?binary path required}"
ARCHIVE="${2:?output archive path required}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGE_DIR="$(mktemp -d)/bunkrdownload"

cleanup() {
  rm -rf "$(dirname "$PACKAGE_DIR")"
}
trap cleanup EXIT

if [[ ! -f "$BINARY" ]]; then
  echo "ERROR: Linux binary not found: $BINARY" >&2
  exit 1
fi
if [[ ! -f "$ROOT/build/appicon.png" ]]; then
  echo "ERROR: missing Linux application icon: build/appicon.png" >&2
  exit 1
fi
if [[ ! -f "$ROOT/build/linux/bunkrdownload.desktop" ]]; then
  echo "ERROR: missing desktop entry: build/linux/bunkrdownload.desktop" >&2
  exit 1
fi

mkdir -p "$PACKAGE_DIR/icons/hicolor/256x256/apps"
cp "$BINARY" "$PACKAGE_DIR/bunkrdownload"
chmod +x "$PACKAGE_DIR/bunkrdownload"
cp "$ROOT/build/linux/bunkrdownload.desktop" "$PACKAGE_DIR/bunkrdownload.desktop"
cp "$ROOT/build/appicon.png" "$PACKAGE_DIR/icons/hicolor/256x256/apps/com.danlegt.bunkrdownload.png"

cat > "$PACKAGE_DIR/install.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${HOME}/.local/bin"
DATA_DIR="${XDG_DATA_HOME:-${HOME}/.local/share}"

mkdir -p "$BIN_DIR" "$DATA_DIR/applications" "$DATA_DIR/icons/hicolor/256x256/apps"
install -m 755 "$SOURCE_DIR/bunkrdownload" "$BIN_DIR/bunkrdownload"
install -m 644 "$SOURCE_DIR/bunkrdownload.desktop" "$DATA_DIR/applications/org.wails.bunkr_downloader.desktop"
install -m 644 "$SOURCE_DIR/icons/hicolor/256x256/apps/com.danlegt.bunkrdownload.png" \
  "$DATA_DIR/icons/hicolor/256x256/apps/com.danlegt.bunkrdownload.png"

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$DATA_DIR/applications" || true
fi
if command -v gtk4-update-icon-cache >/dev/null 2>&1; then
  gtk4-update-icon-cache -f "$DATA_DIR/icons/hicolor" || true
elif command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -f "$DATA_DIR/icons/hicolor" || true
fi

echo "Installed Bunkr Downloader. Launch it from the app menu or run: bunkrdownload"
EOF
chmod +x "$PACKAGE_DIR/install.sh"

mkdir -p "$(dirname "$ARCHIVE")"
tar -C "$(dirname "$PACKAGE_DIR")" -czf "$ARCHIVE" "$(basename "$PACKAGE_DIR")"

echo "Created $ARCHIVE"
