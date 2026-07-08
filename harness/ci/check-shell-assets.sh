#!/usr/bin/env bash
# Verifies the embedded shell HTML bundle under
# go/internal/shellui/server/shellassets is in sync with the TypeScript
# rendering authority in @agora-de/renderer. Regenerates to a temp dir and
# diffs; fails if the committed bundle has drifted from the source.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

GENERATED="$(mktemp -d)"
trap 'rm -rf "$GENERATED"' EXIT

node --experimental-strip-types "$ROOT/harness/build/generate-shell-html.mjs" "$GENERATED" >/dev/null

EXPECTED="$ROOT/go/internal/shellui/server/shellassets"
if ! diff -r "$EXPECTED" "$GENERATED" >/dev/null; then
  echo "embedded shell HTML bundle is out of sync with @agora-de/renderer" >&2
  echo "regenerate with: node --experimental-strip-types harness/build/generate-shell-html.mjs" >&2
  diff -r "$EXPECTED" "$GENERATED" >&2 || true
  exit 1
fi

echo "shell assets: OK"
