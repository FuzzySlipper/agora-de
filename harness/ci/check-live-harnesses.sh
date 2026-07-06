#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

python3 -m py_compile \
  "$ROOT/harness/live/check-den-k8.py" \
  "$ROOT/harness/live/check-installed-catalog.py" \
  "$ROOT/harness/live/check-shell-loop.py" \
  "$ROOT/harness/live/check-native-launch.py" \
  "$ROOT/harness/live/check-structured-layout.py" \
  "$ROOT/harness/live/check-layout-commands.py" \
  "$ROOT/harness/live/check-planner-layout.py" \
  "$ROOT/harness/live/check-overlay-labels.py" \
  "$ROOT/harness/live/check-auto-tiling-wm.py"

python3 - "$ROOT" <<'PY'
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
expectations = {
    "harness/live/check-installed-catalog.py": "agora-de.installed-catalog-live.v1",
    "harness/live/check-shell-loop.py": "agora-de.shell-loop-live.v1",
    "harness/live/check-native-launch.py": "agora-de.native-launch-live.v1",
    "harness/live/check-structured-layout.py": "agora-de.structured-layout-live.v1",
    "harness/live/check-layout-commands.py": "agora-de.layout-commands-live.v1",
    "harness/live/check-planner-layout.py": "agora-de.planner-layout-live.v1",
    "harness/live/check-overlay-labels.py": "agora-de.overlay-labels-live.v1",
    "harness/live/check-auto-tiling-wm.py": "agora-de.auto-tiling-wm-live.v1",
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

commands = (root / "harness/live/check-layout-commands.py").read_text(encoding="utf-8")
for required in [
    "/usr/local/bin/compositorctl",
    "den-k8-layout-commands-visible",
    "/api/catalog/launch",
    "surface\", \"assign-zone",
    "layout\", \"get",
    "layout\", \"set-mode",
    "server[backend_unsupported]",
    "close-command",
]:
    if required not in commands:
        raise SystemExit(f"check-layout-commands.py missing required evidence hook {required!r}")

planner = (root / "harness/live/check-planner-layout.py").read_text(encoding="utf-8")
for required in [
    "/usr/local/bin/compositorctl",
    "den-k8-planner-layout-visible",
    "/api/catalog/launch",
    "assign-zone",
    "layout",
    "planner-mismatch",
    "backend-placement",
    "focus-order",
    "capture",
    "cleanup",
    "master_stack",
]:
    if required not in planner:
        raise SystemExit(f"check-planner-layout.py missing required evidence hook {required!r}")

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

auto_tiling = (root / "harness/live/check-auto-tiling-wm.py").read_text(encoding="utf-8")
for required in [
    "/usr/local/bin/compositorctl",
    "den-k8-auto-tiling-wm-visible",
    "/api/catalog/launch",
    "/api/surfaces/action",
    "/api/layout",
    "/api/layout/action",
    "wm-controls",
    "planner-mismatch",
    "backend-placement",
    "occlusion",
    "focus-order",
    "shell-action",
    "agent-action",
    "overlay",
    "capture",
    "restart",
    "cleanup",
    "master_stack",
]:
    if required not in auto_tiling:
        raise SystemExit(f"check-auto-tiling-wm.py missing required evidence hook {required!r}")
PY

echo "live harness static checks: OK"
