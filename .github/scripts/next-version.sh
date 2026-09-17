#!/usr/bin/env bash
# Compute the next semver tag from Conventional Commit messages since the
# last v* tag. Prints the new tag (e.g. v1.4.0) or nothing when no release
# is warranted.
#
#   feat!: / BREAKING CHANGE             -> major
#   feat:                                -> minor
#   fix: perf: refactor: revert:         -> patch
#   any non-conventional message         -> patch
#   chore: docs: ci: test: style: build: -> no release on their own
#
# Usage: next-version.sh [--bump]   (--bump prints only major|minor|patch|none)
set -euo pipefail

last=$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)
if [ -n "$last" ]; then
  range="$last..HEAD"
  base="${last#v}"
else
  range="HEAD"
  base="0.0.0"
fi

subjects=$(git log --format='%s' "$range" -- 2>/dev/null || true)
bodies=$(git log --format='%b' "$range" -- 2>/dev/null || true)

matches() { # matches <regex> <text>
  printf '%s\n' "$2" | grep -Eq -- "$1"
}

bump=none
if [ -n "$subjects" ]; then
  if matches '^[a-zA-Z]+(\([^)]*\))?!:' "$subjects" || matches '^BREAKING[ -]CHANGE' "$bodies"; then
    bump=major
  elif matches '^feat(\([^)]*\))?:' "$subjects"; then
    bump=minor
  elif matches '^(fix|perf|refactor|revert)(\([^)]*\))?:' "$subjects"; then
    bump=patch
  elif printf '%s\n' "$subjects" | grep -Evq -- '^(chore|docs|ci|test|style|build)(\([^)]*\))?:|^Merge '; then
    # Something that is not a housekeeping commit: ship it as a patch.
    bump=patch
  fi
fi

if [ "${1:-}" = "--bump" ]; then
  echo "$bump"
  exit 0
fi
[ "$bump" = none ] && exit 0

# First release: start at v0.1.0 unless a breaking change asks for v1.0.0.
if [ -z "$last" ] && [ "$bump" != major ]; then
  echo "v0.1.0"
  exit 0
fi

IFS=. read -r major minor patch <<<"$base"
case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac
echo "v${major}.${minor}.${patch}"
