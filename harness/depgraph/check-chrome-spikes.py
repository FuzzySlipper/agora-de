#!/usr/bin/env python3
import json
import pathlib
import sys


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    path = root / "chrome" / "gtk4-layer-shell-spike" / "spike-record.json"
    helper_path = root / "chrome" / "webview-layer-shell" / "agora-de-gtk4-layer-shell-webview"
    failures: list[str] = []

    if not path.exists():
        failures.append("missing GTK4 layer-shell spike record")
    else:
        record = json.loads(path.read_text())
        if record.get("schema") != "agora-de.gtk4-layer-shell-spike.v1":
            failures.append("GTK4 layer-shell spike record has unexpected schema")
        if record.get("status") != "inspectable_spike":
            failures.append("GTK4 layer-shell spike must remain inspectable_spike until productized")

        toolkit = record.get("toolkit", {})
        for field in ("ui", "webview", "layerShell"):
            if not isinstance(toolkit.get(field), str) or not toolkit[field].strip():
                failures.append(f"GTK4 layer-shell spike toolkit missing {field}")

        surfaces = record.get("targetSurfaces", [])
        if not isinstance(surfaces, list) or "dock" not in surfaces or "panel" not in surfaces:
            failures.append("GTK4 layer-shell spike must name dock and panel target surfaces")

        observations = record.get("observations", {})
        for field in ("interaction", "layering", "deployment", "inspection"):
            if not isinstance(observations.get(field), str) or not observations[field].strip():
                failures.append(f"GTK4 layer-shell spike observations missing {field}")

        decision = record.get("decision", {})
        if decision.get("promoteToProductSource") is not False:
            failures.append("GTK4 layer-shell spike should not be product source before evidence promotion")
        if not isinstance(decision.get("reason"), str) or not decision["reason"].strip():
            failures.append("GTK4 layer-shell spike decision must include reason")

    if not helper_path.exists():
        failures.append("missing productized GTK4 layer-shell webview helper")
    else:
        helper_source = helper_path.read_text(encoding="utf-8")
        try:
            compile(helper_source, str(helper_path), "exec")
        except SyntaxError as error:
            failures.append(f"GTK4 layer-shell webview helper syntax error: {error}")
        for marker in (
            "socket.SOCK_DGRAM",
            '"io.agorade.ShellLauncher": "agora-de-shell-launcher.sock"',
            '"io.agorade.ShellSettings": "agora-de-shell-settings.sock"',
            '"io.agorade.ShellStatus": "agora-de-shell-status.sock"',
            'command == "show"',
            'command == "hide"',
            "self.hold()",
            "self.window.set_visible(True)",
            "finish_layer_hide",
            "window.set_child(None)",
            "self.desired_visible",
            '"close-request"',
            'role == "window"',
            '"activation-requested"',
        ):
            if marker not in helper_source:
                failures.append(f"GTK4 layer-shell webview helper missing reusable surface marker {marker!r}")

    if failures:
        print("\n".join(failures))
        return 1

    print("chrome spike fixtures: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
