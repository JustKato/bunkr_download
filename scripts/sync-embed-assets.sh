#!/usr/bin/env bash
# Copy frontend assets into src/embedded for go:embed (paths must stay under src/).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/src/embedded/frontend"
DEST_PARENT="$(dirname "$SRC")"

mkdir -p "$DEST_PARENT"
rm -rf "$SRC"
mkdir -p "$SRC"

if [[ ! -f "$ROOT/build/appicon.png" ]]; then
    echo "ERROR: missing build/appicon.png (application icon source)" >&2
    exit 1
fi

cp -a "$ROOT/frontend/." "$SRC/"
cp "$ROOT/build/appicon.png" "$DEST_PARENT/appicon.png"
