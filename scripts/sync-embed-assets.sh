#!/usr/bin/env bash
# Copy frontend assets into src/embedded for go:embed (paths must stay under src/).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/src/embedded/frontend"
DEST_PARENT="$(dirname "$SRC")"

mkdir -p "$DEST_PARENT"
rm -rf "$SRC"
mkdir -p "$SRC"
cp -a "$ROOT/frontend/." "$SRC/"
