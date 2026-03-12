#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
src="$repo_root/.githooks/commit-msg"
dst="$repo_root/.git/hooks/commit-msg"

if [ ! -f "$src" ]; then
  echo "missing hook source: $src" >&2
  exit 1
fi

cp "$src" "$dst"
chmod +x "$dst"

echo "installed commit-msg hook at $dst"
