#!/usr/bin/env python3
import json
import pathlib
import sys


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    path = root / "chrome" / "gtk4-layer-shell-spike" / "spike-record.json"
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

    if failures:
        print("\n".join(failures))
        return 1

    print("chrome spike fixtures: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
