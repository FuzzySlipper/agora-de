#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

python3 - "$ROOT" <<'PYEOF'
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
required = [
    "README.md",
    "AGENTS.md",
    "docs/successor-brief.md",
    "docs/successor-lesson-packet.md",
    "docs/implementation-plan.md",
    "governance/architecture.md",
    "governance/ownership.toml",
]

missing = [path for path in required if not (root / path).exists()]
if missing:
    for path in missing:
        print(f"missing required doc: {path}")
    raise SystemExit(1)

print("docs scaffold: OK")
PYEOF

