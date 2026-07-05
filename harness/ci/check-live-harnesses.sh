#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

python3 -m py_compile \
  "$ROOT/harness/live/check-den-k8.py" \
  "$ROOT/harness/live/check-installed-catalog.py" \
  "$ROOT/harness/live/check-shell-loop.py" \
  "$ROOT/harness/live/check-native-launch.py" \
  "$ROOT/harness/live/check-structured-layout.py" \
  "$ROOT/harness/live/check-overlay-labels.py"

python3 - "$ROOT" <<'PY'
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
expectations = {
    "harness/live/check-installed-catalog.py": "agora-de.installed-catalog-live.v1",
    "harness/live/check-shell-loop.py": "agora-de.shell-loop-live.v1",
    "harness/live/check-native-launch.py": "agora-de.native-launch-live.v1",
    "harness/live/check-structured-layout.py": "agora-de.structured-layout-live.v1",
    "harness/live/check-overlay-labels.py": "agora-de.overlay-labels-live.v1",
}
for relative, schema in expectations.items():
    text = (root / relative).read_text(encoding="utf-8")
    if schema not in text:
        raise SystemExit(f"{relative} missing schema {schema}")

native = (root / "harness/live/check-native-launch.py").read_text(encoding="utf-8")
for required in [
    "/usr/local/bin/compositorctl",
    "den-k8-native-launch-visible",
    "disabledCode",
    "/api/catalog/launch",
    "/api/surfaces/action",
]:
    if required not in native:
        raise SystemExit(f"check-native-launch.py missing required evidence hook {required!r}")

structured = (root / "harness/live/check-structured-layout.py").read_text(encoding="utf-8")
for required in [
    "/usr/local/bin/compositorctl",
    "den-k8-structured-layout-visible",
    "/api/catalog/launch",
    "/api/surfaces/action",
    "/api/layout",
    "/api/layout/action",
    "occlusion-overlap",
    "cleanup",
]:
    if required not in structured:
        raise SystemExit(f"check-structured-layout.py missing required evidence hook {required!r}")

overlay = (root / "harness/live/check-overlay-labels.py").read_text(encoding="utf-8")
for required in [
    "/usr/local/bin/compositorctl",
    "den-k8-overlay-labels-visible",
    "io.agorade.ShellOverlay",
    "/shell/dist/desktop/?surface=overlay",
    "agent-overlay-surface",
    "/api/catalog/launch",
    "/api/surfaces/action",
    "/api/layout",
    "focus-sequence",
    "layout-labels",
    "cleanup",
]:
    if required not in overlay:
        raise SystemExit(f"check-overlay-labels.py missing required evidence hook {required!r}")
PY

echo "live harness static checks: OK"
