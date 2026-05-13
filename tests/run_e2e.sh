#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

tests/e2e/test_clam.sh "$@"
