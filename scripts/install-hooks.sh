#!/usr/bin/env bash
# Installs git hooks from scripts/hooks/ into .git/hooks/.
# Run once after cloning: scripts/install-hooks.sh
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
GIT_DIR="$(git rev-parse --git-dir)"
HOOKS_SRC="$REPO_ROOT/scripts/hooks"
HOOKS_DST="$GIT_DIR/hooks"

for hook in "$HOOKS_SRC"/*; do
  name="$(basename "$hook")"
  dst="$HOOKS_DST/$name"
  ln -sf "$hook" "$dst"
  chmod +x "$hook"
  echo "installed $name"
done

echo "hooks installed — run 'git hook list' to verify"
