#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

python3 "$ROOT/harness/depgraph/check-go-boundaries.py" "$ROOT"

(cd "$ROOT/go" && go test ./...)
