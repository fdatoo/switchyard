#!/usr/bin/env bash
# Fails if a field tag was retired from any proto file without a corresponding
# `reserved N;` statement being added in the same commit.
#
# A tag is "retired" only if its line was removed AND no replacement line uses
# the same tag number. A field rename that keeps the same tag (e.g. renaming
# `Sensor sensor = 12;` to `NumericSensor numeric_sensor = 12;`) is wire-safe
# and does not require `reserved` — the tag is still in active use.
# (`reserved "old_name";` for the field NAME is governed by a separate rule
# and is not enforced by this script.)

set -euo pipefail

BASE="${1:-HEAD^}"

diff=$(git diff "$BASE" -- ':(glob)proto/**/*.proto')

removed=$(echo "$diff" \
  | grep -E '^-[[:space:]]+[a-zA-Z_][a-zA-Z0-9_.]*[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]*=[[:space:]]*[0-9]+;' \
  | grep -v '^-//' || true)

if [ -z "$removed" ]; then
  exit 0
fi

fail=0
while IFS= read -r line; do
  num=$(echo "$line" | grep -oE '= *[0-9]+;' | grep -oE '[0-9]+' || true)
  [ -z "$num" ] && continue

  # Allow if a new field declaration uses the same tag (rename keeping tag).
  if echo "$diff" | grep -E "^\+[[:space:]]+[a-zA-Z_][a-zA-Z0-9_.]*[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]*=[[:space:]]*$num;" >/dev/null 2>&1; then
    continue
  fi

  # Otherwise require a `reserved N;` statement to be added.
  if ! echo "$diff" | grep -E "^\+[[:space:]]+reserved[[:space:]]+.*\b$num\b" >/dev/null 2>&1; then
    echo "ERROR: field removed without reserved tag $num: $line" >&2
    fail=1
  fi
done <<< "$removed"

exit $fail
