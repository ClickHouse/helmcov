#!/usr/bin/env bash
set -euo pipefail

# Runs Go tests with coverage, fails if total line coverage is below THRESHOLD.
# Pass --update-readme to rewrite the coverage badge in README.md.

threshold="${THRESHOLD:-70}"
profile="coverage-go.out"
update_readme=false
[ "${1:-}" = "--update-readme" ] && update_readme=true

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

go test ./... -coverprofile="$profile"
total="$(go tool cover -func="$profile" | awk '/^total:/ {sub(/%/,"",$3); print $3}')"

echo "total coverage: ${total}%"

if awk "BEGIN {exit !($total < $threshold)}"; then
  echo "coverage ${total}% is below threshold ${threshold}%" >&2
  exit 1
fi

if [ "$update_readme" = true ]; then
  color=red
  awk "BEGIN {exit !($total >= 90)}" && color=brightgreen || \
    { awk "BEGIN {exit !($total >= 70)}" && color=green || color=orange; }
  badge="https://img.shields.io/badge/coverage-${total}%25-${color}"
  sed -i.bak -E "s|https://img.shields.io/badge/coverage-[^)]*|${badge}|" README.md
  rm -f README.md.bak
  echo "updated README coverage badge: ${total}% (${color})"
fi
