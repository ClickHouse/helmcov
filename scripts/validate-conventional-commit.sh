#!/usr/bin/env bash
set -euo pipefail

msg_file="${1:-}"
if [ -z "$msg_file" ] || [ ! -f "$msg_file" ]; then
  echo "commit message file path is required" >&2
  exit 1
fi

subject="$(head -n 1 "$msg_file" | tr -d '\r')"
pattern='^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)(\([a-zA-Z0-9._/-]+\))?(!)?: .+'

if [[ "$subject" =~ $pattern ]]; then
  exit 0
fi

cat >&2 <<'EOF'
Invalid commit message subject.

Expected Conventional Commit format:
  <type>(<optional-scope>)!: <description>

Examples:
  feat(cli): add charts flag
  fix(release): use dist artifact handoff
  chore: update docs
  feat!: change default report format
EOF
echo "Got: $subject" >&2
exit 1
