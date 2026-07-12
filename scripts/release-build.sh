#!/usr/bin/env bash
# Production build for CI/release. Usage: release-build.sh <goos> <goarch> <output-path>
set -euo pipefail

GOOS="${1:?GOOS required}"
GOARCH="${2:?GOARCH required}"
OUTPUT="${3:?output path required}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

./scripts/sync-embed-assets.sh

if command -v wails3 >/dev/null 2>&1; then
  wails3 generate bindings -clean=true -b
else
  echo "wails3 not installed; using committed frontend bindings"
fi

mkdir -p "$(dirname "$OUTPUT")"

BUILD_FLAGS=(-tags production -trimpath -buildvcs=false)
LDFLAGS=(-w -s)
export CGO_ENABLED=1

case "$GOOS" in
  windows)
    CGO_ENABLED=0
    LDFLAGS=(-w -s -H windowsgui)
    ;;
  darwin)
    export CGO_CFLAGS="-mmacosx-version-min=12.0"
    export CGO_LDFLAGS="-mmacosx-version-min=12.0"
    export MACOSX_DEPLOYMENT_TARGET="12.0"
    ;;
  linux)
    if ! pkg-config --exists gtk4 webkitgtk-6.0 2>/dev/null; then
      echo "Linux build requires GTK4/WebKitGTK 6.0 dev packages." >&2
      echo "Install: sudo apt-get install libgtk-4-dev libwebkitgtk-6.0-dev pkg-config" >&2
      exit 1
    fi
    ;;
  *)
    echo "unsupported GOOS: $GOOS" >&2
    exit 1
    ;;
esac

GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED="$CGO_ENABLED" \
  go build "${BUILD_FLAGS[@]}" -ldflags="${LDFLAGS[*]}" -o "$OUTPUT" ./src

echo "Built $OUTPUT"
