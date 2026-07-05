#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

"$ROOT/harness/ci/check-depgraph.sh"
"$ROOT/harness/ci/check-rust.sh"
"$ROOT/harness/ci/check-go.sh"
"$ROOT/harness/ci/check-ts.sh"
"$ROOT/harness/ci/check-contracts.sh"
"$ROOT/harness/ci/check-compositor.sh"
"$ROOT/harness/ci/check-live-harnesses.sh"
"$ROOT/harness/ci/check-docs-commands.sh"

echo "agora-de check-all: OK"
