#!/usr/bin/env bash
set -euo pipefail

current="${1:-}"
bump="${2:-patch}"

case "$bump" in
  major|minor|patch) ;;
  *)
    echo "usage: $0 [vMAJOR.MINOR.PATCH] [major|minor|patch]" >&2
    exit 2
    ;;
esac

if [[ -z "$current" ]]; then
  echo "v0.1.0"
  exit 0
fi

if [[ ! "$current" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "invalid current version: $current" >&2
  exit 2
fi

major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

case "$bump" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch)
    patch=$((patch + 1))
    ;;
esac

echo "v${major}.${minor}.${patch}"
