#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

GENERATED_TS="$ROOT/ts/packages/protocol/src/generated/contracts.ts"
GENERATED_GO="$ROOT/go/internal/settingsprotocol/generated.go"

cargo run \
  --quiet \
  --manifest-path "$ROOT/de-rs/Cargo.toml" \
  -p protocol-codegen \
  -- \
  --check "$GENERATED_TS"

cargo run \
  --quiet \
  --manifest-path "$ROOT/de-rs/Cargo.toml" \
  -p protocol-codegen \
  -- \
  --check-go-settings "$GENERATED_GO"

go -C "$ROOT/go" test ./internal/settingsprotocol

python3 "$ROOT/harness/depgraph/check-live-evidence.py" "$ROOT"
python3 "$ROOT/harness/depgraph/check-layout-model-fixtures.py" "$ROOT"
