#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

test -d "$ROOT/compositor/wayfire-plugin"
test -d "$ROOT/compositor/protocol-fixtures"
test -d "$ROOT/compositor/standard-protocol-probe"
test -d "$ROOT/compositor/smithay-spike"
test -d "$ROOT/chrome/native-dock"
test -d "$ROOT/chrome/panel-supervisor"

echo "compositor/chrome scaffold: OK"

