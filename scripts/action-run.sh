#!/usr/bin/env bash
set -euo pipefail

HELMcov_BIN="${1:?helmcov binary required}"
MARKDOWN_FILE="${2:?markdown file required}"
shift 2

set +e
OUTPUT="$("$HELMcov_BIN" "$@" 2>&1)"
STATUS=$?
set -e

printf '%s\n' "$OUTPUT"

LINE_COVERAGE="$(printf '%s\n' "$OUTPUT" | sed -n 's/.*line-coverage=\([0-9.]*\)%.*/\1/p' | tail -1)"
BRANCH_COVERAGE="$(printf '%s\n' "$OUTPUT" | sed -n 's/.*branch-coverage=\([0-9.]*\)%.*/\1/p' | tail -1)"
LINE_COVERAGE_TESTS="$(printf '%s\n' "$OUTPUT" | sed -n 's/.*line-coverage-tests=\([0-9.]*\)%.*/\1/p' | tail -1)"
BRANCH_COVERAGE_TESTS="$(printf '%s\n' "$OUTPUT" | sed -n 's/.*branch-coverage-tests=\([0-9.]*\)%.*/\1/p' | tail -1)"

{
  echo "line-coverage=${LINE_COVERAGE:-0}"
  echo "branch-coverage=${BRANCH_COVERAGE:-0}"
  # Empty, not 0, when a pinned release older than these fields does not print them.
  echo "line-coverage-tests=${LINE_COVERAGE_TESTS}"
  echo "branch-coverage-tests=${BRANCH_COVERAGE_TESTS}"
  echo "markdown-file=${MARKDOWN_FILE}"
} >> "$GITHUB_OUTPUT"

exit "$STATUS"
