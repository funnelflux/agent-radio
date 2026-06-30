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

tmux_install_hint() {
  if [ "$os" = "darwin" ]; then
    if command -v brew >/dev/null 2>&1; then
      echo "  brew install tmux"
    else
      echo "  Install Homebrew from https://brew.sh, then run: brew install tmux"
    fi
    return
  fi

  if command -v apt-get >/dev/null 2>&1; then
    echo "  sudo apt-get update && sudo apt-get install -y tmux"
  elif command -v dnf >/dev/null 2>&1; then
    echo "  sudo dnf install -y tmux"
  elif command -v yum >/dev/null 2>&1; then
    echo "  sudo yum install -y tmux"
  elif command -v pacman >/dev/null 2>&1; then
    echo "  sudo pacman -S tmux"
  elif command -v zypper >/dev/null 2>&1; then
    echo "  sudo zypper install tmux"
  elif command -v apk >/dev/null 2>&1; then
    echo "  sudo apk add tmux"
  else
    echo "  Install tmux with your system package manager, then rerun this installer."
  fi
}

if ! command -v tmux >/dev/null 2>&1; then
  cat >&2 <<EOF
Agent Radio requires tmux, but tmux was not found on PATH.

Install tmux first, then rerun this installer:

$(tmux_install_hint)

Why this is required:
  Agent Radio launches, monitors, opens, and routes messages to tmux sessions.
  Without tmux, the panel and router cannot operate correctly.
EOF
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

  agent-radio setup
EOF
fi

cat <<EOF

Next steps:

  cd /path/to/project
  agent-radio setup
  agent-radio doctor
  agent-radio up
  agent-radio panel

Setup opens a wizard that can create ~/.config/agent-radio/config.yaml and
install the Agent Radio MCP server into selected Codex, Claude Code, and
OpenCode configs.

Edit the YAML after setup to match your real workspaces, repositories, and
sessions.
EOF
