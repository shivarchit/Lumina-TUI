#!/usr/bin/env bash
# Lumina-TUI installer for macOS and Linux.
#   curl -fsSL https://raw.githubusercontent.com/shivarchit/Lumina-TUI/master/install.sh | bash
set -euo pipefail

REPO="shivarchit/Lumina-TUI"
BIN_NAME="lumina"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
    Darwin) os_tag="mac" ;;
    Linux)  os_tag="linux" ;;
    *)
        echo "Unsupported OS: $os (use install.ps1 on Windows)" >&2
        exit 1
        ;;
esac

case "$arch" in
    x86_64|amd64)  arch_tag="x64" ;;
    arm64|aarch64) arch_tag="arm64" ;;
    armv7l|armv6l) arch_tag="arm" ;;
    *)
        echo "Unsupported architecture: $arch" >&2
        exit 1
        ;;
esac

asset="lumina-${os_tag}-${arch_tag}"
url="https://github.com/${REPO}/releases/latest/download/${asset}"

# Upgrade in place if an existing writable install is found (avoids a stale
# copy earlier on PATH shadowing the new one), else /usr/local/bin when
# writable, else ~/.local/bin.
install_dir=""
existing="$(command -v "$BIN_NAME" 2>/dev/null || true)"
if [ -n "$existing" ] && [ -w "$existing" ]; then
    install_dir="$(dirname "$existing")"
elif [ -w "/usr/local/bin" ]; then
    install_dir="/usr/local/bin"
else
    install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

echo "Downloading ${asset} ..."
curl -fSL --progress-bar "$url" -o "$tmp"

if [ ! -s "$tmp" ]; then
    echo "Download failed or empty file" >&2
    exit 1
fi

chmod +x "$tmp"
mv "$tmp" "${install_dir}/${BIN_NAME}"
trap - EXIT

echo "Installed ${BIN_NAME} to ${install_dir}/${BIN_NAME}"
"${install_dir}/${BIN_NAME}" -v

case ":$PATH:" in
    *":${install_dir}:"*) ;;
    *)
        echo ""
        echo "NOTE: ${install_dir} is not on your PATH. Add it:"
        echo "  export PATH=\"${install_dir}:\$PATH\""
        ;;
esac
