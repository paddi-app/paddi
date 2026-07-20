#!/usr/bin/env bash
# Bump the CLI version: updates main.go, commits, and tags.
# Usage: scripts/bump-version.sh major|minor|patch
set -euo pipefail

part="${1:-}"
case "$part" in
  major|minor|patch) ;;
  *) echo "usage: $0 major|minor|patch" >&2; exit 1 ;;
esac

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
main_go="$repo_root/main.go"

current="$(grep -oE 'var version = "[0-9]+\.[0-9]+\.[0-9]+"' "$main_go" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')"
IFS='.' read -r major minor patch <<< "$current"

case "$part" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac

next="$major.$minor.$patch"
tag="v$next"

sed -i '' -E "s/var version = \"[0-9]+\.[0-9]+\.[0-9]+\"/var version = \"$next\"/" "$main_go"

git -C "$repo_root" add main.go
git -C "$repo_root" commit -m "chore: bump version to $tag"
git -C "$repo_root" tag "$tag"

echo "bumped $current -> $next, committed and tagged $tag"
