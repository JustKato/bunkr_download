#!/usr/bin/env bash
# Fedora distrobox with build deps for this project.
set -euo pipefail

BOX_NAME="${BOX_NAME:-wails-dev}"
BOX_IMAGE="${BOX_IMAGE:-registry.fedoraproject.org/fedora:44}"

if ! command -v distrobox >/dev/null 2>&1; then
    echo "ERROR: distrobox not found. It ships with Bazzite by default." >&2
    exit 1
fi

if distrobox list | grep -qw "$BOX_NAME"; then
    echo "==> Distrobox '$BOX_NAME' already exists, skipping creation."
else
    echo "==> Creating distrobox '$BOX_NAME' from $BOX_IMAGE ..."
    distrobox create --name "$BOX_NAME" --image "$BOX_IMAGE" --yes
fi

echo "==> Installing Wails build dependencies inside '$BOX_NAME' ..."
distrobox enter "$BOX_NAME" -- bash -lc '
    set -euo pipefail
    sudo dnf install -y \
        golang nodejs npm git \
        gcc gcc-c++ make pkgconf-pkg-config \
        gtk4-devel webkitgtk6.0-devel

    # wails3 CLI -> ~/.local/bin
    mkdir -p "$HOME/.local/bin"
    GOBIN="$HOME/.local/bin" go install github.com/wailsapp/wails/v3/cmd/wails3@latest

    echo
    echo "==> Versions:"
    go version
    "$HOME/.local/bin/wails3" version
'

echo
echo "==> Done. Build with:  ./scripts/build.sh"
echo "==> Dev mode:          distrobox enter $BOX_NAME -- bash -lc 'PATH=\$HOME/.local/bin:\$PATH wails3 dev'"
