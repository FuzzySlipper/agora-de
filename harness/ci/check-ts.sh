#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

node "$ROOT/harness/depgraph/check-ts-structure.mjs" "$ROOT"

if [ -x "$ROOT/ts/node_modules/.bin/tsc" ]; then
  "$ROOT/ts/node_modules/.bin/tsc" --build "$ROOT/ts/tsconfig.json"
elif command -v tsc >/dev/null 2>&1; then
  tsc --build "$ROOT/ts/tsconfig.json"
else
  echo "tsc not found. Run: cd ts && npm install"
  exit 1
fi
