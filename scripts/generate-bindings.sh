#!/usr/bin/env bash
# Generate Wails JS bindings from ./src and place them where the frontend imports them.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BINDINGS_PKG="frontend/bindings/github.com/justkato/bunkr_download"

echo "==> Generating Wails bindings ..."
wails3 generate bindings -clean=true -b ./src

# Wails mirrors the Go package directory, so ./src emits under .../src/.
if [ -d "$BINDINGS_PKG/src" ]; then
    mv "$BINDINGS_PKG/src/"* "$BINDINGS_PKG/"
    rmdir "$BINDINGS_PKG/src"
fi

for required in bunkrservice.js models.js index.js; do
    if [ ! -f "$BINDINGS_PKG/$required" ]; then
        echo "ERROR: expected binding file missing: $BINDINGS_PKG/$required" >&2
        exit 1
    fi
done

if grep -rq '@wailsio/runtime' frontend/bindings/ 2>/dev/null; then
    echo "ERROR: bindings import @wailsio/runtime (needs npm). Regenerate with:" >&2
    echo "  ./scripts/generate-bindings.sh" >&2
    exit 1
fi

echo "==> Bindings ready: $BINDINGS_PKG"
