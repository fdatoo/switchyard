#!/usr/bin/env sh
set -eu

cd "$(git rev-parse --show-toplevel)"
git config core.hooksPath .githooks
echo "hooks path set to .githooks"
