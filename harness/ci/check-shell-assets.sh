#!/usr/bin/env bash
# Verifies the embedded shell assets under
# go/internal/shellui/server/{shellassets,iconassets} are in sync with the
# TypeScript rendering authority in @agora-de/renderer + @agora-de/components.
# Regenerates to a temp dir and diffs; fails if the committed bundle has drifted.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

GENERATED="$(mktemp -d)"
trap 'rm -rf "$GENERATED"' EXIT

node --experimental-strip-types "$ROOT/harness/build/generate-shell-html.mjs" "$GENERATED" >/dev/null

EXPECTED="$ROOT/go/internal/shellui/server"
status=0
for sub in shellassets iconassets; do
  if ! diff -r "$EXPECTED/$sub" "$GENERATED/$sub" >/dev/null; then
    echo "embedded $sub bundle is out of sync with @agora-de rendering authority" >&2
    diff -r "$EXPECTED/$sub" "$GENERATED/$sub" >&2 || true
    status=1
  fi
done
if [ "$status" -ne 0 ]; then
  echo "regenerate with: node --experimental-strip-types harness/build/generate-shell-html.mjs" >&2
  exit 1
fi

echo "shell assets: OK"
