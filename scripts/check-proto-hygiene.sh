#!/usr/bin/env bash
# Fails if a field was removed from any proto file without a corresponding
# `reserved` statement being added in the same commit.

set -euo pipefail

BASE="${1:-HEAD^}"

removed=$(git diff "$BASE" -- ':(glob)proto/**/*.proto' \
  | grep -E '^-[[:space:]]+[a-zA-Z_][a-zA-Z0-9_.]*[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]*=[[:space:]]*[0-9]+;' \
  | grep -v '^-//' || true)

if [ -z "$removed" ]; then
  exit 0
fi

fail=0
while IFS= read -r line; do
  num=$(echo "$line" | grep -oE '= *[0-9]+;' | grep -oE '[0-9]+' || true)
  [ -z "$num" ] && continue
  if ! git diff "$BASE" -- ':(glob)proto/**/*.proto' | grep -E "^\+[[:space:]]+reserved[[:space:]]+.*\b$num\b" >/dev/null 2>&1; then
    echo "ERROR: field removed without reserved tag $num: $line" >&2
    fail=1
  fi
done <<< "$removed"

exit $fail
