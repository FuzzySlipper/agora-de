#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

test -f "$ROOT/de-rs/crates/protocol/protocol-codegen/src/lib.rs"
test -f "$ROOT/ts/packages/protocol/src/index.ts"

echo "contract scaffold: OK"

