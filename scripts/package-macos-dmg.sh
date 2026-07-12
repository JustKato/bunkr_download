#!/usr/bin/env bash
# Wrap a macOS binary in .app and create a compressed DMG.
# Usage: package-macos-dmg.sh <binary-path> <output-dmg-path> [version]
set -euo pipefail

BINARY="${1:?binary path required}"
DMG_PATH="${2:?output dmg path required}"
VERSION="${3:-0.0.0}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_NAME="Bunkr Downloader"
APP_DIR="$(mktemp -d)/${APP_NAME}.app"
STAGING="$(mktemp -d)"

cleanup() {
  rm -rf "$(dirname "$APP_DIR")" "$STAGING"
}
trap cleanup EXIT

mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"

cp "$BINARY" "$APP_DIR/Contents/MacOS/bunkrdownload"
chmod +x "$APP_DIR/Contents/MacOS/bunkrdownload"

if [[ -f "$ROOT/build/darwin/icons.icns" ]]; then
  cp "$ROOT/build/darwin/icons.icns" "$APP_DIR/Contents/Resources/icons.icns"
fi

if [[ -f "$ROOT/build/darwin/Info.plist" ]]; then
  sed \
    -e "s/{{APP_VERSION}}/${VERSION#v}/g" \
    "$ROOT/build/darwin/Info.plist" > "$APP_DIR/Contents/Info.plist"
else
  cat > "$APP_DIR/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleExecutable</key>
    <string>bunkrdownload</string>
    <key>CFBundleIdentifier</key>
    <string>com.danlegt.bunkrdownload</string>
    <key>CFBundleVersion</key>
    <string>${VERSION#v}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION#v}</string>
    <key>LSMinimumSystemVersion</key>
    <string>12.0.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
  </dict>
</plist>
EOF
fi

mkdir -p "$(dirname "$DMG_PATH")"
cp -R "$APP_DIR" "$STAGING/"
hdiutil create -volname "$APP_NAME" -srcfolder "$STAGING/${APP_NAME}.app" -ov -format UDZO "$DMG_PATH"

echo "Created $DMG_PATH"
