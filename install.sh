#!/usr/bin/env bash
set -euo pipefail

repo="${AGENT_RADIO_REPO:-funnelflux/agent-radio}"
version="${AGENT_RADIO_VERSION:-latest}"
install_dir="${AGENT_RADIO_INSTALL_DIR:-$HOME/.local/bin}"
helper_dir="${AGENT_RADIO_HELPER_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/agent-radio/shell}"
assume_yes="${AGENT_RADIO_ASSUME_YES:-0}"

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

asset="agent-radio-$os-$arch"
helper_asset="agent-radio-shell-helpers.sh"
if [ "$version" = "latest" ]; then
  release_base="https://github.com/$repo/releases/latest/download"
  version_label="latest"
else
  release_base="https://github.com/$repo/releases/download/$version"
  version_label="$version"
fi
url="$release_base/$asset"
helper_url="$release_base/$helper_asset"
checksums_url="$release_base/checksums.txt"

confirm_overwrite() {
  target="$1"
  if [ ! -e "$target" ]; then
    return 0
  fi
  if [ "$assume_yes" = "1" ] || [ "$assume_yes" = "true" ] || [ "$assume_yes" = "yes" ]; then
    return 0
  fi
  if [ -t 0 ]; then
    printf 'Overwrite existing %s? [y/N] ' "$target" >&2
    read -r answer
    case "$answer" in
      y|Y|yes|YES) return 0 ;;
    esac
    echo "aborted" >&2
    exit 1
  fi
  cat >&2 <<EOF
Refusing to overwrite existing file in non-interactive mode:
  $target

Rerun interactively, remove the file first, or set AGENT_RADIO_ASSUME_YES=1.
EOF
  exit 1
}

hash_file() {
  file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

expected_hash() {
  filename="$1"
  awk -v f="$filename" '$2 == f { print $1; found = 1 } END { exit found ? 0 : 1 }' "$checksums"
}

verify_checksum() {
  file="$1"
  filename="$2"
  expected="$(expected_hash "$filename")" || {
    echo "checksum for $filename not found in checksums.txt" >&2
    exit 1
  }
  actual="$(hash_file "$file")"
  if [ "$actual" != "$expected" ]; then
    cat >&2 <<EOF
checksum mismatch for $filename
  expected: $expected
  actual:   $actual
EOF
    exit 1
  fi
}

mkdir -p "$install_dir"
confirm_overwrite "$install_dir/agent-radio"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
binary="$tmp_dir/$asset"
helper="$tmp_dir/$helper_asset"
checksums="$tmp_dir/checksums.txt"
helper_available=1

echo "Installing agent-radio $version_label for $os/$arch"
echo "Downloading release asset and checksums..."
curl -fsSL "$url" -o "$binary"
curl -fsSL "$checksums_url" -o "$checksums"
if ! curl -fsSL "$helper_url" -o "$helper"; then
  if [ "$version" = "latest" ]; then
    echo "failed to download required shell helper asset: $helper_url" >&2
    exit 1
  fi
  helper_available=0
  echo "Shell helper asset is not available for $version_label; skipping helper install." >&2
fi

verify_checksum "$binary" "$asset"
if [ "$helper_available" = "1" ]; then
  verify_checksum "$helper" "$helper_asset"
  mkdir -p "$helper_dir"
  confirm_overwrite "$helper_dir/agent-radio.sh"
fi

chmod +x "$binary"
mv "$binary" "$install_dir/agent-radio"
if [ "$helper_available" = "1" ]; then
  mv "$helper" "$helper_dir/agent-radio.sh"
  chmod 0644 "$helper_dir/agent-radio.sh"
fi

cat <<EOF
Installed:
  $install_dir/agent-radio
EOF

if [ "$helper_available" = "1" ]; then
  cat <<EOF
  $helper_dir/agent-radio.sh
EOF
fi

if ! command -v agent-radio >/dev/null 2>&1; then
  cat <<EOF

agent-radio is installed, but it is not currently on PATH.
Add this to your shell profile:

  export PATH="\$HOME/.local/bin:\$PATH"

Then run:

  agent-radio setup
EOF
fi

if [ "$helper_available" = "1" ]; then
  cat <<EOF

Optional shell helpers:

  source "$helper_dir/agent-radio.sh"
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
