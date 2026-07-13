#!/usr/bin/env bash
# Package a Linux release with launcher metadata and its application icon.
# Usage: package-linux-release.sh <binary-path> <output-tar.gz-path>
set -euo pipefail

BINARY="${1:?binary path required}"
ARCHIVE="${2:?output archive path required}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGE_DIR="$(mktemp -d)/bunkrdownload"
ICON_NAME="com.danlegt.bunkrdownload"
DESKTOP_ID="org.wails.bunkr_downloader"

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

mkdir -p "$PACKAGE_DIR/icons/hicolor"
for size in 32 48 64 128 256; do
  dir="$PACKAGE_DIR/icons/hicolor/${size}x${size}/apps"
  mkdir -p "$dir"
  cp "$ROOT/build/appicon.png" "$dir/${ICON_NAME}.png"
done

cp "$BINARY" "$PACKAGE_DIR/bunkrdownload"
chmod 755 "$PACKAGE_DIR/bunkrdownload"

cat > "$PACKAGE_DIR/run.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$DIR/bunkrdownload" "$@"
EOF
chmod 755 "$PACKAGE_DIR/run.sh"

cat > "$PACKAGE_DIR/install.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${HOME}/.local/bin"
DATA_DIR="${XDG_DATA_HOME:-${HOME}/.local/share}"
ICON_NAME="com.danlegt.bunkrdownload"
DESKTOP_ID="org.wails.bunkr_downloader"

mkdir -p "$BIN_DIR" "$DATA_DIR/applications"

install -m 755 "$SOURCE_DIR/bunkrdownload" "$BIN_DIR/bunkrdownload"
install -m 755 "$SOURCE_DIR/run.sh" "$BIN_DIR/bunkrdownload-run"

for size_dir in "$SOURCE_DIR/icons/hicolor"/*; do
  [[ -d "$size_dir/apps" ]] || continue
  size="$(basename "$size_dir")"
  target="$DATA_DIR/icons/hicolor/$size/apps"
  mkdir -p "$target"
  install -m 644 "$size_dir/apps/${ICON_NAME}.png" "$target/${ICON_NAME}.png"
done

cat > "$DATA_DIR/applications/${DESKTOP_ID}.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Version=1.0
Name=Bunkr Downloader
Comment=Bunkr album downloader
Exec=${BIN_DIR}/bunkrdownload
Icon=${ICON_NAME}
Terminal=false
Categories=Network;Utility;
StartupNotify=true
StartupWMClass=${DESKTOP_ID}
X-GNOME-Startup-Class=${DESKTOP_ID}
X-KDE-StartupClass=${DESKTOP_ID}
DESKTOP
chmod 644 "$DATA_DIR/applications/${DESKTOP_ID}.desktop"

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$DATA_DIR/applications" || true
fi
if command -v gtk4-update-icon-cache >/dev/null 2>&1; then
  gtk4-update-icon-cache -f "$DATA_DIR/icons/hicolor" || true
elif command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -f "$DATA_DIR/icons/hicolor" || true
fi

echo "Installed Bunkr Downloader."
echo "  App menu: Bunkr Downloader"
echo "  Command:  bunkrdownload"
echo
echo "Make sure ~/.local/bin is on your PATH."
EOF
chmod 755 "$PACKAGE_DIR/install.sh"

cat > "$PACKAGE_DIR/README.txt" <<'EOF'
Bunkr Downloader — Linux

Recommended install (adds app menu icon and command):
  tar -xzf bunkrdownload-linux-amd64.tar.gz
  cd bunkrdownload
  ./install.sh

Then launch "Bunkr Downloader" from your app menu, or run:
  bunkrdownload

Quick try without installing:
  ./run.sh

The bare binary also self-registers its icon on first launch, but install.sh
is the reliable way to get the launcher icon on KDE, GNOME, and Plasma.
EOF

mkdir -p "$(dirname "$ARCHIVE")"
tar -C "$(dirname "$PACKAGE_DIR")" --owner=0 --group=0 --mode=755 \
  -czf "$ARCHIVE" "$(basename "$PACKAGE_DIR")"

echo "Created $ARCHIVE"
