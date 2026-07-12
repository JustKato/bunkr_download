#!/usr/bin/env bash
# Build inside the wails-dev distrobox.
set -euo pipefail

BOX_NAME="${BOX_NAME:-wails-dev}"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! distrobox list | grep -qw "$BOX_NAME"; then
    echo "ERROR: distrobox '$BOX_NAME' not found. Run ./scripts/init.sh first." >&2
    exit 1
fi

echo "==> Building inside distrobox '$BOX_NAME' ..."
distrobox enter "$BOX_NAME" -- bash -lc "
    set -euo pipefail
    export PATH=\"\$HOME/.local/bin:\$PATH\"
    cd '$PROJECT_DIR'
    mkdir -p build/bin
    go build -o build/bin/bunkrdownload .
"

echo
echo "==> Build complete: $PROJECT_DIR/build/bin/bunkrdownload"
