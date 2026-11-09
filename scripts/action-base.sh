#!/usr/bin/env bash
# Measures coverage of the pull request's base commit, so the head run can show the change.
# Prints cobertura-file=<path> to GITHUB_OUTPUT on success; any failure leaves it unset.
set -euo pipefail

HELMcov_BIN="${1:?helmcov binary required}"
BASE_SHA="${2:?base commit required}"
OUT_DIR="${3:?output directory required}"
shift 3

WORKSPACE="${GITHUB_WORKSPACE:-$PWD}"
SOURCE="$OUT_DIR/source"
REPORT="$OUT_DIR/base.xml"
mkdir -p "$OUT_DIR"

if ! git -C "$WORKSPACE" cat-file -e "${BASE_SHA}^{commit}" 2>/dev/null; then
  git -C "$WORKSPACE" fetch --quiet --no-tags --depth=1 origin "$BASE_SHA"
fi
# A killed earlier run on a reused runner can leave the worktree behind.
rm -rf "$SOURCE"
git -C "$WORKSPACE" worktree prune
git -C "$WORKSPACE" worktree add --quiet --detach "$SOURCE" "$BASE_SHA"
trap 'git -C "$WORKSPACE" worktree remove --force "$SOURCE"' EXIT

# Paths given under the workspace point at the head checkout; move them to the base one.
ARGS=()
for arg in "$@"; do
  case "$arg" in
    "$WORKSPACE" | "$WORKSPACE"/*) arg="$SOURCE${arg#"$WORKSPACE"}" ;;
  esac
  ARGS+=("$arg")
done

(cd "$SOURCE" && "$HELMcov_BIN" "${ARGS[@]}" --format cobertura --cobertura-file "$REPORT")
echo "cobertura-file=$REPORT" >> "$GITHUB_OUTPUT"
