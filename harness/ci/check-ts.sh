#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

node "$ROOT/harness/depgraph/check-ts-structure.mjs" "$ROOT"

if command -v tsc >/dev/null 2>&1; then
  tsc --build "$ROOT/ts/tsconfig.json"
else
  echo "tsc not found; skipped TypeScript compile"
fi

