#!/usr/bin/env bash
set -euo pipefail

MARKER="${1:?comment marker required}"
MARKDOWN_FILE="${2:?markdown file required}"
TOKEN="${3:?token required}"

PR_NUMBER="$(jq -r '.pull_request.number // empty' "$GITHUB_EVENT_PATH")"
if [ -z "$PR_NUMBER" ]; then
  echo "not a pull_request event; skipping comment"
  exit 0
fi

if [ ! -f "$MARKDOWN_FILE" ]; then
  echo "markdown file not found; skipping comment"
  exit 0
fi

export GH_TOKEN="$TOKEN"
REPO="${GITHUB_REPOSITORY}"
MARKER_HTML="<!-- ${MARKER} -->"

if ! grep -qF "$MARKER_HTML" "$MARKDOWN_FILE"; then
  echo "markdown file missing comment marker ${MARKER_HTML}" >&2
  exit 1
fi

COMMENT_ID="$(
  gh api "repos/${REPO}/issues/${PR_NUMBER}/comments" --paginate \
    --jq ".[] | select(.body | contains(\"${MARKER_HTML}\")) | .id" | head -1
)"

if [ -n "$COMMENT_ID" ]; then
  jq -n --rawfile body "$MARKDOWN_FILE" '{body: $body}' | \
    gh api -X PATCH "repos/${REPO}/issues/comments/${COMMENT_ID}" --input -
else
  jq -n --rawfile body "$MARKDOWN_FILE" '{body: $body}' | \
    gh api "repos/${REPO}/issues/${PR_NUMBER}/comments" --input -
fi
