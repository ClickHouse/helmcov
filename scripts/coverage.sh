#!/usr/bin/env bash
set -euo pipefail

# Runs Go tests with coverage, fails if total line coverage is below THRESHOLD.
# Pass --badge-json <path> to also write a shields.io endpoint badge.

threshold="${THRESHOLD:-85}"
profile="coverage-go.out"
badge_json=""
if [ "${1:-}" = "--badge-json" ]; then
  badge_json="${2:?--badge-json needs a path}"
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

go test -race -covermode=atomic ./... -coverprofile="$profile"
total="$(go tool cover -func="$profile" | awk '/^total:/ {sub(/%/,"",$3); print $3}')"

echo "total coverage: ${total}%"

if [ -n "$badge_json" ]; then
  color=orange
  if awk "BEGIN {exit !($total >= 90)}"; then
    color=brightgreen
  elif awk "BEGIN {exit !($total >= 70)}"; then
    color=green
  fi
  mkdir -p "$(dirname "$badge_json")"
  printf '{"schemaVersion":1,"label":"coverage","message":"%s%%","color":"%s"}\n' "$total" "$color" > "$badge_json"
  echo "wrote coverage badge: ${total}% (${color})"
fi

if awk "BEGIN {exit !($total < $threshold)}"; then
  echo "coverage ${total}% is below threshold ${threshold}%" >&2
  exit 1
fi
