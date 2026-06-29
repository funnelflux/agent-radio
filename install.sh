#!/usr/bin/env bash
set -euo pipefail

repo="${AGENT_RADIO_REPO:-funnelflux/agent-radio}"
version="${AGENT_RADIO_VERSION:-latest}"
install_dir="${AGENT_RADIO_INSTALL_DIR:-$HOME/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  linux) os="linux" ;;
  darwin) os="darwin" ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

if [ "$version" = "latest" ]; then
  url="https://github.com/$repo/releases/latest/download/agent-radio-$os-$arch"
  version_label="latest"
else
  url="https://github.com/$repo/releases/download/$version/agent-radio-$os-$arch"
  version_label="$version"
fi

mkdir -p "$install_dir"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

echo "Installing agent-radio $version_label for $os/$arch"
curl -fsSL "$url" -o "$tmp"
chmod +x "$tmp"
mv "$tmp" "$install_dir/agent-radio"

echo "Installed: $install_dir/agent-radio"
if ! command -v agent-radio >/dev/null 2>&1; then
  cat <<EOF

agent-radio is installed, but it is not currently on PATH.
Add this to your shell profile:

  export PATH="\$HOME/.local/bin:\$PATH"

Then run:

  agent-radio doctor
EOF
else
  agent-radio doctor || true
fi

cat <<EOF

Next steps:

  cd /path/to/project
  agent-radio setup
  agent-radio up
  agent-radio panel

Edit ~/.config/agent-radio/config.yaml after setup to match your real
workspaces, repositories, and sessions.
EOF
