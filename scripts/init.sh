#!/usr/bin/env bash
# One-time setup on Bazzite (calls bazzite.sh).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Initializing development environment ..."
"$SCRIPT_DIR/bazzite.sh"
