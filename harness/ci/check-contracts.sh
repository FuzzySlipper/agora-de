#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

GENERATED_TS="$ROOT/ts/packages/protocol/src/generated/contracts.ts"

cargo run \
  --quiet \
  --manifest-path "$ROOT/de-rs/Cargo.toml" \
  -p protocol-codegen \
  -- \
  --check "$GENERATED_TS"
