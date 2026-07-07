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
  "$ROOT/harness/live/check-auto-tiling-wm.py" \
  "$ROOT/harness/live/check-daily-wm-workflow.py" \
  "$ROOT/harness/live/check-popup-stability.py" \
  "$ROOT/harness/live/check-theme-switch.py" \
  "$ROOT/harness/live/check-responsiveness-baseline.py"

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
    "harness/live/check-daily-wm-workflow.py": "agora-de.daily-wm-workflow-live.v1",
    "harness/live/check-popup-stability.py": "agora-de.popup-stability-live.v1",
    "harness/live/check-theme-switch.py": "agora-de.theme-switch-live.v1",
    "harness/live/check-responsiveness-baseline.py": "agora-de.responsiveness-baseline-live.v1",
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

daily = (root / "harness/live/check-daily-wm-workflow.py").read_text(encoding="utf-8")
for required in [
    "/usr/local/bin/compositorctl",
    "den-k8-daily-wm-workflow-visible",
    "/api/catalog/launch",
    "/api/surfaces/action",
    "/api/layout",
    "/api/layout/action",
    "wm-controls",
    "apps-button",
    "operator-button",
    "focusNext",
    "focusPrevious",
    "promoteToMaster",
    "moveZone",
    "setFloating",
    "launcher-status",
    "overlay",
    "capture",
    "recovery",
    "master_stack",
]:
    if required not in daily:
        raise SystemExit(f"check-daily-wm-workflow.py missing required evidence hook {required!r}")

popup = (root / "harness/live/check-popup-stability.py").read_text(encoding="utf-8")
for required in [
    "/home/agent/.local/bin/agora-de-compositorctl",
    "den-k8-popup-stability",
    "/api/catalog/launch",
    "/api/surfaces/action",
    "shell-status",
    "shell-launcher",
    "panel-geometry",
    "status-popup-geometry",
    "launcher-popup-geometry",
    "work-surface-geometry",
    "unmanaged-transient",
    "cleanup",
]:
    if required not in popup:
        raise SystemExit(f"check-popup-stability.py missing required evidence hook {required!r}")

theme_switch = (root / "harness/live/check-theme-switch.py").read_text(encoding="utf-8")
for required in [
    "/home/agent/.local/bin/agora-de-compositorctl",
    "den-k8-theme-switch",
    "AGORA_DE_SHELLUI_THEME_ID",
    "AGORA_DE_SHELLUI_THEME_MANIFEST",
    "/api/theme",
    "/shell/dist/desktop/?surface=dock",
    "agora-ember",
    "theme-variant-visible-capture",
    "den-k8-theme-switch-visible",
    "agora-de.theme-evidence.v1",
    "systemctl\", \"--user\", \"restart\", \"agora-de-shellui.service",
]:
    if required not in theme_switch:
        raise SystemExit(f"check-theme-switch.py missing required evidence hook {required!r}")

responsiveness = (root / "harness/live/check-responsiveness-baseline.py").read_text(encoding="utf-8")
for required in [
    "/home/agent/.local/bin/agora-de-compositorctl",
    "/api/theme",
    "/api/catalog/apps",
    "/api/catalog/launch",
    "/api/surfaces",
    "/api/surfaces/action",
    "/api/layout",
    "/api/workspaces",
    "/api/workspaces/action",
    "/api/operator/status",
    "shell-status",
    "shell-launcher",
    "Alacritty.desktop",
    "launch-to-observed",
    "p50Ms",
    "p95Ms",
    "cleanup",
]:
    if required not in responsiveness:
        raise SystemExit(f"check-responsiveness-baseline.py missing required evidence hook {required!r}")

kill_all = (root / "deploy/shellui/agora-de-kill-all").read_text(encoding="utf-8")
for required in [
    "Usage: agora-de-kill-all [--help]",
    "-h|--help|help)",
    "unknown argument",
    "exit 2",
    "log \"stopping Agora display/session processes",
]:
    if required not in kill_all:
        raise SystemExit(f"agora-de-kill-all missing safety hook {required!r}")
stop_index = kill_all.index('log "stopping Agora display/session processes')
for required in ["-h|--help|help)", "unknown argument", "exit 2"]:
    if kill_all.index(required) > stop_index:
        raise SystemExit(f"agora-de-kill-all handles {required!r} after destructive cleanup starts")
PY

echo "live harness static checks: OK"
