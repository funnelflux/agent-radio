#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist_dir="${DIST_DIR:-"$repo_root/dist"}"
version="${VERSION:-dev}"

mkdir -p "$dist_dir"
rm -f "$dist_dir"/agent-radio-* "$dist_dir"/checksums.txt

targets=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
)

for target in "${targets[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  out="$dist_dir/agent-radio-$os-$arch"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w" -o "$out" "$repo_root/cmd/agent-radio"
done

(
  cd "$dist_dir"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum agent-radio-* > checksums.txt
  else
    shasum -a 256 agent-radio-* > checksums.txt
  fi
)

printf 'release artifacts written to %s\n' "$dist_dir"
