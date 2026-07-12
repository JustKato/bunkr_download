#!/usr/bin/env bash
# Production build for CI/release. Usage: release-build.sh <goos> <goarch> <output-path>
set -euo pipefail

GOOS="${1:?GOOS required}"
GOARCH="${2:?GOARCH required}"
OUTPUT="${3:?output path required}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

./scripts/sync-embed-assets.sh

export PATH="$(go env GOPATH)/bin:${PATH:-}"
if ! command -v wails3 >/dev/null 2>&1; then
  echo "wails3 CLI not found in PATH" >&2
  exit 1
fi

wails3 generate bindings -clean=true -b

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
    ;;
  *)
    echo "unsupported GOOS: $GOOS" >&2
    exit 1
    ;;
esac

GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED="$CGO_ENABLED" \
  go build "${BUILD_FLAGS[@]}" -ldflags="${LDFLAGS[*]}" -o "$OUTPUT" ./src

echo "Built $OUTPUT"
